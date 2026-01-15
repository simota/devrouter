package devrouter

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Allow non-browser requests (no Origin header)
		if origin == "" {
			return true
		}
		u, ok := validateOrigin(origin)
		if !ok {
			return false
		}
		// Allow if Origin host matches the request Host
		return strings.EqualFold(u.Host, r.Host)
	},
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// handleLogsWebSocket streams logs for an entire stack
func (s *Server) handleLogsWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]
	s.streamLogs(w, r, stackID, "")
}

// handleServiceLogsWebSocket streams logs for a specific service
func (s *Server) handleServiceLogsWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]
	serviceName := vars["serviceName"]
	s.streamLogs(w, r, stackID, serviceName)
}

func (s *Server) streamLogs(w http.ResponseWriter, r *http.Request, stackID, serviceName string) {
	reg, err := LoadRegistry()
	if err != nil {
		http.Error(w, "registry error", http.StatusInternalServerError)
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		http.Error(w, "stack not found", http.StatusNotFound)
		return
	}

	composeService := ""
	serviceLabel := ""
	if serviceName != "" {
		svc, ok := FindService(stack, serviceName)
		if !ok {
			http.Error(w, "service not found", http.StatusNotFound)
			return
		}
		composeService = svc.ComposeService
		serviceLabel = svc.Name
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	sendLogMessage(conn, LogMessage{
		Type:      "connected",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Content:   "connected to logs: " + stackID,
	})

	// Check container states and send warning if needed
	containerStates, err := ComposePs(stack.ComposeFilePath)
	if err != nil {
		sendLogMessage(conn, LogMessage{
			Type:      "warning",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Content:   "Failed to check container state: " + err.Error(),
		})
	} else {
		allStopped := true
		for _, state := range containerStates {
			if state == "running" {
				allStopped = false
				break
			}
		}
		if len(containerStates) == 0 {
			sendLogMessage(conn, LogMessage{
				Type:      "warning",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Content:   "No containers found. The stack may have been stopped or removed.",
			})
		} else if allStopped {
			sendLogMessage(conn, LogMessage{
				Type:      "warning",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Content:   "All containers are stopped. No new logs will appear.",
			})
		} else if serviceName != "" {
			// Check specific service
			state := containerStates[composeService]
			if state != "running" {
				sendLogMessage(conn, LogMessage{
					Type:      "warning",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Content:   "Container '" + serviceName + "' is not running (state: " + state + ").",
				})
			}
		}
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	args := []string{"compose", "-f", stack.ComposeFilePath, "logs", "-f", "--tail=100"}
	if composeService != "" {
		args = append(args, composeService)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sendLogMessage(conn, LogMessage{Type: "error", Content: err.Error()})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		sendLogMessage(conn, LogMessage{Type: "error", Content: err.Error()})
		return
	}

	if err := cmd.Start(); err != nil {
		sendLogMessage(conn, LogMessage{Type: "error", Content: err.Error()})
		return
	}

	resolver := newLogServiceResolver(stack, serviceLabel)
	done := make(chan struct{})
	go pipeToWebSocket(conn, stdout, done, resolver)
	go pipeToWebSocket(conn, stderr, done, resolver)

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					cancel()
					return
				}
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			cancel()
			break
		}
	}

	cmd.Wait()
	close(done)

	sendLogMessage(conn, LogMessage{
		Type:      "disconnected",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Content:   "disconnected from logs",
	})
}

func pipeToWebSocket(conn *websocket.Conn, reader io.Reader, done chan struct{}, resolver logServiceResolver) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-done:
			return
		default:
			line := scanner.Text()
			service, content := resolver.resolve(line)
			sendLogMessage(conn, LogMessage{
				Type:      "log",
				Service:   service,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Content:   content,
			})
		}
	}
}

type logServiceResolver struct {
	byCompose       map[string]string
	composeServices []string
	defaultService  string
}

func newLogServiceResolver(stack StackRecord, defaultService string) logServiceResolver {
	byCompose := make(map[string]string, len(stack.Services))
	composeServices := make([]string, 0, len(stack.Services))
	for _, svc := range stack.Services {
		if svc.ComposeService == "" {
			continue
		}
		byCompose[svc.ComposeService] = svc.Name
		composeServices = append(composeServices, svc.ComposeService)
	}
	sort.Slice(composeServices, func(i, j int) bool {
		return len(composeServices[i]) > len(composeServices[j])
	})
	return logServiceResolver{
		byCompose:       byCompose,
		composeServices: composeServices,
		defaultService:  defaultService,
	}
}

func (r logServiceResolver) resolve(line string) (string, string) {
	if line == "" {
		if r.defaultService != "" {
			return r.defaultService, line
		}
		return "", line
	}
	prefix, content, ok := splitLogPrefix(line)
	if ok {
		service := r.resolvePrefix(prefix)
		if service != "" {
			return service, trimLeadingSpace(content)
		}
	}
	if r.defaultService != "" {
		return r.defaultService, line
	}
	return "", line
}

func (r logServiceResolver) resolvePrefix(prefix string) string {
	if prefix == "" {
		return ""
	}
	if svc, ok := r.byCompose[prefix]; ok {
		return svc
	}
	stripped := stripContainerIndex(prefix)
	if svc, ok := r.byCompose[stripped]; ok {
		return svc
	}
	for _, compose := range r.composeServices {
		if strings.HasSuffix(stripped, compose) {
			return r.byCompose[compose]
		}
	}
	return ""
}

func splitLogPrefix(line string) (string, string, bool) {
	idx := strings.Index(line, "|")
	if idx == -1 {
		return "", "", false
	}
	prefix := strings.TrimSpace(line[:idx])
	if prefix == "" {
		return "", "", false
	}
	content := line[idx+1:]
	return prefix, content, true
}

func stripContainerIndex(prefix string) string {
	idx := strings.LastIndexAny(prefix, "-_")
	if idx == -1 || idx+1 >= len(prefix) {
		return prefix
	}
	for i := idx + 1; i < len(prefix); i++ {
		if prefix[i] < '0' || prefix[i] > '9' {
			return prefix
		}
	}
	return prefix[:idx]
}

func trimLeadingSpace(value string) string {
	if value == "" {
		return value
	}
	if value[0] == ' ' {
		return value[1:]
	}
	return value
}

func sendLogMessage(conn *websocket.Conn, msg LogMessage) error {
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	data, _ := json.Marshal(msg)
	return conn.WriteMessage(websocket.TextMessage, data)
}

// handleStatsWebSocket streams resource stats for a stack via WebSocket
func (s *Server) handleStatsWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		http.Error(w, "registry error", http.StatusInternalServerError)
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		http.Error(w, "stack not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Stream stats every 2 seconds
	statsCh := StreamStackStats(ctx, stack.ComposeFilePath, 2*time.Second)

	done := make(chan struct{})

	// Read pump (for detecting client disconnect)
	go func() {
		defer close(done)
		conn.SetReadLimit(maxMessageSize)
		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Ping ticker
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case stats, ok := <-statsCh:
			if !ok {
				return
			}
			stats.StackID = stackID
			if err := sendStatsMessage(conn, stats); err != nil {
				return
			}
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func sendStatsMessage(conn *websocket.Conn, stats *StackStatsResponse) error {
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	data, _ := json.Marshal(stats)
	return conn.WriteMessage(websocket.TextMessage, data)
}

// handleWatchWebSocket streams file watch events for a stack via WebSocket
func (s *Server) handleWatchWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Get watch event stream
	eventCh, cleanup := StreamWatchEvents(stackID)
	defer cleanup()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	done := make(chan struct{})

	// Read pump
	go func() {
		defer close(done)
		conn.SetReadLimit(maxMessageSize)
		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Ping ticker
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	// Send initial status
	status := GetWatchStatus(stackID)
	if status != nil {
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		data, _ := json.Marshal(map[string]interface{}{
			"type":   "status",
			"status": status,
		})
		conn.WriteMessage(websocket.TextMessage, data)
	}

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			data, _ := json.Marshal(map[string]interface{}{
				"type":  "event",
				"event": event,
			})
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}
