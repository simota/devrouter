//go:build linux || darwin

package devrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestLogStreamDockerHelper runs in a child process instead of a real Docker
// daemon. The normal test process leaves this test immediately.
func TestLogStreamDockerHelper(t *testing.T) {
	if os.Getenv("DEVROUTER_LOG_TEST_HELPER") != "1" {
		return
	}
	waitForRelease := func() {
		conn, err := net.DialTimeout("tcp", os.Getenv("DEVROUTER_LOG_TEST_READY"), 5*time.Second)
		if err != nil {
			os.Exit(3)
		}
		conn.Write([]byte("ready"))
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		var release [1]byte
		conn.Read(release[:])
		conn.Close()
		os.Exit(0)
	}
	if os.Getenv("DEVROUTER_LOG_TEST_PIPE_CHILD") == "1" {
		pgid, err := syscall.Getpgid(0)
		if err != nil {
			os.Exit(3)
		}
		if err := os.WriteFile(os.Getenv("DEVROUTER_LOG_TEST_CHILD_GROUP"), []byte(strconv.Itoa(pgid)), 0600); err != nil {
			os.Exit(3)
		}
		waitForRelease()
	}
	runPipeChild := func() {
		child := exec.Command(os.Args[0], "-test.run=^TestLogStreamDockerHelper$")
		child.Env = append(os.Environ(), "DEVROUTER_LOG_TEST_PIPE_CHILD=1")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Run(); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 5 {
		os.Exit(2)
	}
	args = args[1:]
	recordCommand := func() {
		if err := os.WriteFile(os.Getenv("DEVROUTER_LOG_TEST_PID"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(3)
		}
		pgid, err := syscall.Getpgid(0)
		if err != nil {
			os.Exit(3)
		}
		if err := os.WriteFile(os.Getenv("DEVROUTER_LOG_TEST_GROUP"), []byte(strconv.Itoa(pgid)), 0600); err != nil {
			os.Exit(3)
		}
		argsJSON, _ := json.Marshal(args)
		if err := os.WriteFile(os.Getenv("DEVROUTER_LOG_TEST_ARGS"), argsJSON, 0600); err != nil {
			os.Exit(3)
		}
	}
	if args[3] == "ps" {
		mode := os.Getenv("DEVROUTER_LOG_TEST_MODE")
		if mode == "preflight" || mode == "preflight-inherited" {
			recordCommand()
			if mode == "preflight-inherited" {
				runPipeChild()
			}
			waitForRelease()
		}
		fmt.Println(`{"Name":"myapp-api-1","Service":"myapp-api","State":"running"}`)
		fmt.Println(`{"Name":"myapp-web-1","Service":"myapp-web","State":"running"}`)
		os.Exit(0)
	}
	if args[3] != "logs" {
		os.Exit(2)
	}
	recordCommand()

	switch os.Getenv("DEVROUTER_LOG_TEST_MODE") {
	case "finite":
		fmt.Fprintln(os.Stdout, "myapp-api | first")
		fmt.Fprintln(os.Stdout, "myapp-api | second")
	case "concurrent":
		var wg sync.WaitGroup
		for i, out := range []io.Writer{os.Stdout, os.Stderr} {
			wg.Add(1)
			go func(stream int, out io.Writer) {
				defer wg.Done()
				service := []string{"myapp-api", "myapp-web"}[stream]
				for j := 0; j < 100; j++ {
					fmt.Fprintf(out, "%s | %d:%d %s\n", service, stream, j, strings.Repeat("x", 8192))
				}
			}(i, out)
		}
		wg.Wait()
	case "long":
		fmt.Fprintln(os.Stdout, "myapp-api | "+strings.Repeat("x", 128*1024))
		fmt.Fprintln(os.Stdout, "myapp-api | after long line")
	case "oversized":
		fmt.Fprintln(os.Stdout, "myapp-api | "+strings.Repeat("x", 2*1024*1024))
		// Keep the process alive so the test verifies cancellation after a scan error.
		<-time.After(30 * time.Second)
		os.Exit(9)
	case "failure":
		fmt.Fprintln(os.Stderr, "myapp-api | before failure")
		os.Exit(7)
	case "idle":
		fmt.Fprintln(os.Stdout, "myapp-api | ready")
		// This is a safety timeout, not synchronization for the test.
		<-time.After(30 * time.Second)
		os.Exit(9)
	case "idle-inherited":
		fmt.Fprintln(os.Stdout, "myapp-api | ready")
		runPipeChild()
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

type logStreamHarness struct {
	conn           *websocket.Conn
	done           <-chan struct{}
	pidFile        string
	argsFile       string
	ready          *net.TCPListener
	control        net.Conn
	groupFile      string
	childGroupFile string
}

func newLogStreamHarness(t *testing.T, mode, service string) *logStreamHarness {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	// Quote the test executable as a single shell word, including paths with spaces.
	quotedExecutable := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	script := "#!/bin/sh\nexec " + quotedExecutable + " -test.run=^TestLogStreamDockerHelper$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(dir, "docker.pid")
	argsFile := filepath.Join(dir, "docker.args")
	groupFile := filepath.Join(dir, "docker.pgid")
	childGroupFile := filepath.Join(dir, "plugin.pgid")
	h := &logStreamHarness{pidFile: pidFile, argsFile: argsFile, groupFile: groupFile, childGroupFile: childGroupFile}
	if mode == "preflight" || mode == "preflight-inherited" || mode == "idle-inherited" {
		ready, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ready.Close() })
		h.ready = ready
		t.Setenv("DEVROUTER_LOG_TEST_READY", ready.Addr().String())
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DEVROUTER_LOG_TEST_HELPER", "1")
	t.Setenv("DEVROUTER_LOG_TEST_PIPE_CHILD", "")
	t.Setenv("DEVROUTER_LOG_TEST_MODE", mode)
	t.Setenv("DEVROUTER_LOG_TEST_PID", pidFile)
	t.Setenv("DEVROUTER_LOG_TEST_ARGS", argsFile)
	t.Setenv("DEVROUTER_LOG_TEST_GROUP", groupFile)
	t.Setenv("DEVROUTER_LOG_TEST_CHILD_GROUP", childGroupFile)
	stack := StackRecord{
		ID: "log-test", Name: "log-test", RepoPath: dir,
		ComposeFilePath: filepath.Join(dir, "compose.yaml"),
		Services: []ServiceRecord{
			{Name: "api", ComposeService: "myapp-api"},
			{Name: "web", ComposeService: "myapp-web"},
		},
	}
	if err := SaveRegistry(Registry{Version: 1, Stacks: []StackRecord{stack}}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	router := NewServer(ServerConfig{}).setupRoutes()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		router.ServeHTTP(w, r)
	}))
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws/logs/log-test"
	if service != "" {
		url += "/" + service
	}
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close()
		if h.control != nil {
			h.control.Close()
		}
		select {
		case <-done:
			server.Close()
		case <-time.After(5 * time.Second):
			server.Listener.Close()
			t.Error("log handler did not stop after client disconnect")
		}
	})
	h.conn = conn
	h.done = done
	return h
}

func (h *logStreamHarness) awaitControlledProcess(t *testing.T) {
	t.Helper()
	h.ready.SetDeadline(time.Now().Add(5 * time.Second))
	conn, err := h.ready.Accept()
	if err != nil {
		t.Fatal(err)
	}
	h.control = conn
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	ready := make([]byte, len("ready"))
	if _, err := io.ReadFull(conn, ready); err != nil {
		t.Fatal(err)
	}
}

func (h *logStreamHarness) assertControlledProcessStopped(t *testing.T) {
	t.Helper()
	// No release signal has been sent. EOF proves cancellation stopped the
	// process holding this connection, independently of zombie reaping by PID 1.
	h.control.SetReadDeadline(time.Now().Add(5 * time.Second))
	var tail [1]byte
	if _, err := h.control.Read(tail[:]); err != io.EOF {
		t.Errorf("process survived cancellation: %v", err)
	}
}

func (h *logStreamHarness) waitStopped(t *testing.T) {
	t.Helper()
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("log handler did not finish")
	}
	data, err := os.ReadFile(h.pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Errorf("docker logs process %d was not reaped: %v", pid, err)
	}
	data, err = os.ReadFile(h.groupFile)
	if err != nil {
		t.Fatal(err)
	}
	pgid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if pgid != pid {
		t.Errorf("command PGID=%d, want dedicated group %d", pgid, pid)
	}
	testGroup, err := syscall.Getpgid(0)
	if err != nil {
		t.Fatal(err)
	}
	if pgid == testGroup {
		t.Error("command shares the caller's process group")
	}
	if data, err := os.ReadFile(h.childGroupFile); err == nil {
		childGroup, err := strconv.Atoi(string(data))
		if err != nil {
			t.Fatal(err)
		}
		if childGroup != pgid {
			t.Errorf("plugin PGID=%d, want %d", childGroup, pgid)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func (h *logStreamHarness) collect(t *testing.T) []LogMessage {
	t.Helper()
	h.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var messages []LogMessage
	for {
		var msg LogMessage
		if err := h.conn.ReadJSON(&msg); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				t.Errorf("log connection did not close cleanly after output finished: %v", err)
			}
			break
		}
		messages = append(messages, msg)
	}
	h.conn.Close()
	h.waitStopped(t)
	return messages
}

func logMessages(messages []LogMessage) []LogMessage {
	var logs []LogMessage
	for _, msg := range messages {
		if msg.Type == "log" {
			logs = append(logs, msg)
		}
	}
	return logs
}

func TestLogStreamNaturalExit(t *testing.T) {
	for _, service := range []string{"", "api"} {
		t.Run("service="+service, func(t *testing.T) {
			h := newLogStreamHarness(t, "finite", service)
			messages := h.collect(t)
			logs := logMessages(messages)
			if len(logs) != 2 || logs[0].Content != "first" || logs[1].Content != "second" {
				t.Fatalf("output was not fully drained: %#v", logs)
			}
			for _, msg := range logs {
				if msg.Service != "api" {
					t.Errorf("service = %q, want api", msg.Service)
				}
			}
			if messages[0].Type != "connected" || messages[len(messages)-1].Type != "disconnected" {
				t.Errorf("missing connection lifecycle messages: %#v", messages)
			}
			data, err := os.ReadFile(h.argsFile)
			if err != nil {
				t.Fatal(err)
			}
			var args []string
			if err := json.Unmarshal(data, &args); err != nil {
				t.Fatal(err)
			}
			wantLast := "--tail=100"
			if service != "" {
				wantLast = "myapp-api"
			}
			if args[len(args)-1] != wantLast {
				t.Errorf("unexpected docker arguments: %v", args)
			}
		})
	}
}

func TestLogStreamConcurrentOutput(t *testing.T) {
	h := newLogStreamHarness(t, "concurrent", "")
	logs := logMessages(h.collect(t))
	if len(logs) != 200 {
		t.Fatalf("received %d of 200 stdout/stderr messages", len(logs))
	}
	seen := make(map[string]bool)
	for _, msg := range logs {
		key := strings.SplitN(msg.Content, " ", 2)[0]
		if seen[key] {
			t.Errorf("duplicate message: %s", key)
		}
		seen[key] = true
		wantService := "api"
		if strings.HasPrefix(key, "1:") {
			wantService = "web"
		}
		if msg.Service != wantService {
			t.Errorf("message %s service = %q, want %s", key, msg.Service, wantService)
		}
	}
}

func TestLogStreamLongLine(t *testing.T) {
	h := newLogStreamHarness(t, "long", "")
	logs := logMessages(h.collect(t))
	if len(logs) != 2 || logs[0].Content != strings.Repeat("x", 128*1024) || logs[1].Content != "after long line" {
		t.Errorf("long log line or following output was lost (received %d messages)", len(logs))
	}
}

func TestLogStreamErrorsStopProcess(t *testing.T) {
	for _, mode := range []string{"oversized", "failure"} {
		t.Run(mode, func(t *testing.T) {
			h := newLogStreamHarness(t, mode, "")
			messages := h.collect(t)
			found := false
			for _, msg := range messages {
				if msg.Type == "error" && msg.Content != "" {
					found = true
					if _, err := time.Parse(time.RFC3339, msg.Timestamp); err != nil {
						t.Errorf("invalid error timestamp %q: %v", msg.Timestamp, err)
					}
				}
			}
			if !found {
				t.Errorf("stream failure was not reported: %#v", messages)
			}
		})
	}
}

func TestLogStreamClientDisconnectReapsProcess(t *testing.T) {
	for _, mode := range []string{"idle", "idle-inherited"} {
		t.Run(mode, func(t *testing.T) {
			h := newLogStreamHarness(t, mode, "")
			h.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			for {
				var msg LogMessage
				if err := h.conn.ReadJSON(&msg); err != nil {
					t.Fatal(err)
				}
				if msg.Type == "log" && msg.Content == "ready" {
					break
				}
			}
			if h.ready != nil {
				h.awaitControlledProcess(t)
			}
			h.conn.Close()
			h.waitStopped(t)
			if h.control != nil {
				h.assertControlledProcessStopped(t)
			}
		})
	}
}

func TestLogStreamDisconnectDuringPreflight(t *testing.T) {
	for _, mode := range []string{"preflight", "preflight-inherited"} {
		t.Run(mode, func(t *testing.T) {
			h := newLogStreamHarness(t, mode, "")
			h.awaitControlledProcess(t)
			// Accept proves docker ps has started and recorded its PID. Its
			// output pipes remain open until cancellation or fixture cleanup.
			h.conn.Close()
			h.waitStopped(t)
			h.assertControlledProcessStopped(t)
			data, err := os.ReadFile(h.argsFile)
			if err != nil {
				t.Fatal(err)
			}
			var args []string
			if err := json.Unmarshal(data, &args); err != nil {
				t.Fatal(err)
			}
			if args[3] != "ps" {
				t.Errorf("started logs after preflight cancellation: %v", args)
			}
		})
	}
}

func TestPipeLogMessagesLineLimit(t *testing.T) {
	const prefix = "myapp-api | "
	resolver := newLogServiceResolver(StackRecord{
		Services: []ServiceRecord{{Name: "api", ComposeService: "myapp-api"}},
	}, "")
	for _, size := range []int{maxLogLineSize - 1, maxLogLineSize, maxLogLineSize + 1} {
		for _, ending := range []string{"", "\n", "\r\n"} {
			t.Run(fmt.Sprintf("size=%d/ending=%q", size, ending), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				messages := make(chan LogMessage)
				result := make(chan error, 1)
				input := prefix + strings.Repeat("x", size-len(prefix)) + ending
				go func() { result <- pipeLogMessages(ctx, strings.NewReader(input), messages, resolver) }()
				count := 0
				for {
					select {
					case msg := <-messages:
						count++
						if msg.Service != "api" || len(msg.Content) != size-len(prefix) {
							t.Errorf("line changed: service=%q, content length=%d", msg.Service, len(msg.Content))
						}
					case err := <-result:
						if size > maxLogLineSize {
							if err == nil || count != 0 {
								t.Errorf("oversized line: error=%v, messages=%d", err, count)
							}
						} else if err != nil || count != 1 {
							t.Errorf("valid line: error=%v, messages=%d", err, count)
						}
						return
					case <-ctx.Done():
						t.Fatal("pipe did not finish")
					}
				}
			})
		}
	}
}
