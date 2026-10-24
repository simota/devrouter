package devrouter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"sync"
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
	writeWait           = 10 * time.Second
	pongWait            = 60 * time.Second
	pingPeriod          = (pongWait * 9) / 10
	maxMessageSize      = 512
	maxLogLineSize      = 1024 * 1024
	logPreflightTimeout = 10 * time.Second
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
	ctx, cancel := context.WithCancel(r.Context())
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()
	defer func() {
		cancel()
		conn.Close()
		<-readDone
	}()
	// Start the heartbeat clock with the read deadline so a slow preflight
	// cannot delay the first ping beyond that deadline.
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()
	finish := func(errorMessage string) {
		if errorMessage != "" {
			if err := sendLogMessage(conn, LogMessage{
				Type:      "error",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Content:   errorMessage,
			}); err != nil {
				return
			}
		}
		if err := sendLogMessage(conn, LogMessage{
			Type:      "disconnected",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Content:   "disconnected from logs",
		}); err != nil {
			return
		}
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(writeWait))
	}

	if err := sendLogMessage(conn, LogMessage{
		Type:      "connected",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Content:   "connected to logs: " + stackID,
	}); err != nil {
		return
	}

	// Check container states and send warning if needed
	preflightCtx, cancelPreflight := context.WithTimeout(ctx, logPreflightTimeout)
	containerStates, err := composePsContext(preflightCtx, stack.ComposeFilePath)
	cancelPreflight()
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		if err := sendLogMessage(conn, LogMessage{
			Type:      "warning",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Content:   "Failed to check container state: " + err.Error(),
		}); err != nil {
			return
		}
	} else {
		allStopped := true
		for _, state := range containerStates {
			if state == "running" {
				allStopped = false
				break
			}
		}
		if len(containerStates) == 0 {
			if err := sendLogMessage(conn, LogMessage{
				Type:      "warning",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Content:   "No containers found. The stack may have been stopped or removed.",
			}); err != nil {
				return
			}
		} else if allStopped {
			if err := sendLogMessage(conn, LogMessage{
				Type:      "warning",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Content:   "All containers are stopped. No new logs will appear.",
			}); err != nil {
				return
			}
		} else if serviceName != "" {
			// Check specific service
			state := containerStates[composeService]
			if state != "running" {
				if err := sendLogMessage(conn, LogMessage{
					Type:      "warning",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Content:   "Container '" + serviceName + "' is not running (state: " + state + ").",
				}); err != nil {
					return
				}
			}
		}
	}

	args := []string{"compose", "-f", stack.ComposeFilePath, "logs", "-f", "--tail=100"}
	if composeService != "" {
		args = append(args, composeService)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	configureCommandCancellation(cmd)
	cmd.WaitDelay = time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		finish(err.Error())
		return
	}
	defer stdout.Close()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		finish(err.Error())
		return
	}
	defer stderr.Close()

	if err := cmd.Start(); err != nil {
		finish(err.Error())
		return
	}

	// Pipe readers only enqueue messages. This handler owns every WebSocket
	// write, including pings and terminal messages.
	messages := make(chan LogMessage)
	scanErrors := make(chan error, 2)
	resolver := newLogServiceResolver(stack, serviceLabel)
	var readers sync.WaitGroup
	for _, reader := range []io.Reader{stdout, stderr} {
		readers.Add(1)
		go func(reader io.Reader) {
			defer readers.Done()
			if err := pipeLogMessages(ctx, reader, messages, resolver); err != nil {
				scanErrors <- err
			}
		}(reader)
	}

	processDone := make(chan struct{})
	var waitErr error
	go func() {
		// Wait must not close the pipes before their buffered output is drained.
		readers.Wait()
		waitErr = cmd.Wait()
		close(processDone)
	}()
	defer func() {
		cancel()
		stdout.Close()
		stderr.Close()
		<-processDone
	}()
	for {
		select {
		case msg := <-messages:
			if err := sendLogMessage(conn, msg); err != nil {
				return
			}
		case err := <-scanErrors:
			finish("read logs: " + err.Error())
			return
		case <-processDone:
			// A scan error can arrive together with process completion. Preserve it
			// instead of reporting a successful end of the stream.
			errorMessage := ""
			select {
			case err := <-scanErrors:
				errorMessage = "read logs: " + err.Error()
			default:
				if waitErr != nil {
					errorMessage = "docker logs failed: " + waitErr.Error()
				}
			}
			finish(errorMessage)
			return
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func pipeLogMessages(ctx context.Context, reader io.Reader, messages chan<- LogMessage, resolver logServiceResolver) error {
	scanner := bufio.NewScanner(reader)
	// The limit applies to the raw line (including the Compose prefix), excluding
	// its LF or CRLF delimiter. Leave room for either delimiter in Scanner's buffer.
	scanner.Buffer(make([]byte, 64*1024), maxLogLineSize+2)
	for scanner.Scan() {
		if len(scanner.Bytes()) > maxLogLineSize {
			return fmt.Errorf("log line exceeds %d bytes", maxLogLineSize)
		}
		service, content := resolver.resolve(scanner.Text())
		msg := LogMessage{
			Type:      "log",
			Service:   service,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Content:   content,
		}
		select {
		case messages <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return scanner.Err()
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
