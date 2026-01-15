package devrouter

import "testing"

func TestLogServiceResolver(t *testing.T) {
	stack := StackRecord{
		Services: []ServiceRecord{
			{Name: "api", ComposeService: "myapp-api"},
			{Name: "web", ComposeService: "myapp-web"},
		},
	}
	resolver := newLogServiceResolver(stack, "")

	tests := []struct {
		name        string
		line        string
		wantService string
		wantContent string
	}{
		{
			name:        "matches compose prefix",
			line:        "myapp-api  | started",
			wantService: "api",
			wantContent: "started",
		},
		{
			name:        "matches project prefix with index",
			line:        "myapp-1234-myapp-web-1 | GET /health",
			wantService: "web",
			wantContent: "GET /health",
		},
		{
			name:        "no match keeps line",
			line:        "plain log line",
			wantService: "",
			wantContent: "plain log line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotService, gotContent := resolver.resolve(tt.line)
			if gotService != tt.wantService {
				t.Fatalf("service mismatch: got %q want %q", gotService, tt.wantService)
			}
			if gotContent != tt.wantContent {
				t.Fatalf("content mismatch: got %q want %q", gotContent, tt.wantContent)
			}
		})
	}
}

func TestLogServiceResolverDefaultService(t *testing.T) {
	stack := StackRecord{
		Services: []ServiceRecord{
			{Name: "api", ComposeService: "myapp-api"},
		},
	}
	resolver := newLogServiceResolver(stack, "api")
	line := "plain log line"
	service, content := resolver.resolve(line)
	if service != "api" {
		t.Fatalf("service mismatch: got %q want %q", service, "api")
	}
	if content != line {
		t.Fatalf("content mismatch: got %q want %q", content, line)
	}
}
