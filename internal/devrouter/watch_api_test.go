package devrouter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStartWatchRequestValidation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stack := StackRecord{ID: "watch-api-test", Name: "watch-api-test", RepoPath: t.TempDir()}
	if err := SaveRegistry(Registry{Stacks: []StackRecord{stack}}); err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ServerConfig{}).setupRoutes()
	defaultRules := []WatchRule{
		{Pattern: "*.go", Action: watchActionRestart},
		{Pattern: "*.ts", Action: watchActionRestart},
		{Pattern: "*.tsx", Action: watchActionRestart},
		{Pattern: "*.js", Action: watchActionRestart},
		{Pattern: "*.jsx", Action: watchActionRestart},
		{Pattern: "*.py", Action: watchActionRestart},
	}
	tests := []struct {
		name         string
		body         string
		noBody       bool
		invalid      bool
		wantRules    []WatchRule
		wantDebounce time.Duration
	}{
		{name: "no body", noBody: true},
		{name: "empty body"},
		{name: "whitespace", body: " \n\t "},
		{name: "empty object", body: "{}"},
		{name: "null preserves defaults", body: "null"},
		{name: "unknown field", body: `{"futureOption":true}`},
		{name: "trailing whitespace", body: "{} \n\t "},
		{name: "body at size limit", body: "{}" + strings.Repeat(" ", maxJSONBodySize-2)},
		{
			name:         "configured rules",
			body:         `{"enabled":false,"debounce":125000000,"rules":[{"pattern":"*.go","action":"ignore"}]}`,
			wantRules:    []WatchRule{{Pattern: "*.go", Action: watchActionIgnore}},
			wantDebounce: 125 * time.Millisecond,
		},
		{name: "malformed JSON", body: "{", invalid: true},
		{name: "invalid rules type", body: `{"rules":"invalid"}`, invalid: true},
		{name: "invalid debounce type", body: `{"debounce":"500ms"}`, invalid: true},
		{name: "multiple JSON values", body: "{} {}", invalid: true},
		{name: "trailing null", body: "{} null", invalid: true},
		{name: "trailing invalid data", body: "{} invalid", invalid: true},
		{name: "oversized JSON", body: `{"futureOption":"` + strings.Repeat("x", 2*maxJSONBodySize) + `"}`, invalid: true},
		{name: "oversized whitespace", body: strings.Repeat(" ", maxJSONBodySize+1), invalid: true},
		{name: "oversized trailing whitespace", body: "{}" + strings.Repeat(" ", maxJSONBodySize-1), invalid: true},
	}
	for _, existing := range []bool{false, true} {
		name := "without existing watcher"
		if existing {
			name = "with existing watcher"
		}
		t.Run(name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Cleanup(func() { StopWatching(stack.ID) })
					var original *StackWatcher
					if existing {
						if err := StartWatching(stack.ID, stack.RepoPath, "", nil, WatchConfig{
							Enabled: true,
							Rules:   []WatchRule{{Pattern: "*", Action: watchActionIgnore}},
						}); err != nil {
							t.Fatal(err)
						}
						original = GetStackWatcher(stack.ID)
					}
					req := httptest.NewRequest(http.MethodPost, "/api/stacks/"+stack.ID+"/watch/start", strings.NewReader(tt.body))
					if tt.noBody {
						req.Body = nil
					}
					rr := httptest.NewRecorder()
					handler.ServeHTTP(rr, req)
					watcher := GetStackWatcher(stack.ID)
					if tt.invalid {
						if rr.Code != http.StatusBadRequest {
							t.Errorf("status = %d, want 400; body: %s", rr.Code, rr.Body.String())
						}
						if watcher != original {
							t.Error("invalid request created or replaced the watcher")
						}
						if original != nil && !original.GetStatus().Watching {
							t.Error("invalid request stopped the existing watcher")
						}
						var response APIError
						if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
							t.Fatalf("decode error response: %v", err)
						}
						if response.Code != "WATCH_REQUEST_INVALID" || response.Message == "" {
							t.Errorf("unexpected error response: %+v", response)
						}
						return
					}
					if rr.Code != http.StatusOK {
						t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
					}
					if watcher == nil || watcher == original {
						t.Fatal("valid request did not start a new watcher")
					}
					status := watcher.GetStatus()
					if !status.Enabled || !status.Watching {
						t.Errorf("watcher not enabled and running: %+v", status)
					}
					wantRules := tt.wantRules
					if wantRules == nil {
						wantRules = defaultRules
					}
					if !reflect.DeepEqual(status.Rules, wantRules) {
						t.Errorf("rules = %+v, want %+v", status.Rules, wantRules)
					}
					if watcher.config.Debounce != tt.wantDebounce {
						t.Errorf("debounce = %v, want %v", watcher.config.Debounce, tt.wantDebounce)
					}
				})
			}
		})
	}
}
