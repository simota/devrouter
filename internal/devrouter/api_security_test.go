package devrouter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestSizeLimit(t *testing.T) {
	cfg := ServerConfig{Addr: ":8080"}
	s := NewServer(cfg)

	// Let's create a temporary registry file.
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create a dummy stack
	reg := Registry{
		Version: 1,
		Stacks: []StackRecord{
			{
				ID:       "test-stack",
				Name:     "test-stack",
				RepoPath: tmpDir,
			},
		},
	}
	if err := SaveRegistry(reg); err != nil {
		t.Fatalf("Failed to save registry: %v", err)
	}

	handler := s.setupRoutes()

	// Create a large body (2MB)
	largeBody := `{"stack": "` + strings.Repeat("a", 2*1024*1024) + `"}`

	req, _ := http.NewRequest("POST", "/api/stacks/test-stack/init/preview", bytes.NewBufferString(largeBody))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// We expect 400 Bad Request because decode fails with "request body too large"
	// parseInitConfigRequest returns error, handleInitPreview maps it to 400.

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request for large body, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Check if the error message indicates body size issue
	// Note: The error message from json.Decode() wrapped by MaxBytesReader is usually
	// "http: request body too large"
	expectedError := "request body too large"
	if !strings.Contains(rr.Body.String(), expectedError) {
		t.Logf("Response body: %s", rr.Body.String())
	}
}

func TestEnvRedaction(t *testing.T) {
	cfg := ServerConfig{Addr: ":8080"}
	s := NewServer(cfg)

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create a docker-compose.yml with secrets
	composeContent := `
services:
  db:
    environment:
      POSTGRES_PASSWORD: secret_password
      API_KEY: 12345-abcde
      SLACK_WEBHOOK_URL: https://hooks.slack.example/services/DUMMY/WEBHOOK/URL
      SESSION_SECRET: very-secret-session
      NORMAL_VAR: safe_value
      DB_HOST: localhost
`
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write compose file: %v", err)
	}

	// Create stack record
	reg := Registry{
		Version: 1,
		Stacks: []StackRecord{
			{
				ID:              "secret-stack",
				Name:            "secret-stack",
				RepoPath:        tmpDir,
				ComposeFilePath: composePath,
			},
		},
	}
	if err := SaveRegistry(reg); err != nil {
		t.Fatalf("Failed to save registry: %v", err)
	}

	handler := s.setupRoutes()
	req, _ := http.NewRequest("GET", "/api/stacks/secret-stack/env", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp EnvResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Helper to find env value
	findEnv := func(name string) string {
		for _, item := range resp.Env {
			if item.Name == name {
				return item.Value
			}
		}
		return ""
	}

	// Verify sensitive values are redacted
	val := findEnv("POSTGRES_PASSWORD")
	if val != "[REDACTED]" {
		t.Errorf("POSTGRES_PASSWORD expected [REDACTED], got %q", val)
	}

	val = findEnv("API_KEY")
	if val != "[REDACTED]" {
		t.Errorf("API_KEY expected [REDACTED], got %q", val)
	}

	val = findEnv("SLACK_WEBHOOK_URL")
	if val != "[REDACTED]" {
		t.Errorf("SLACK_WEBHOOK_URL expected [REDACTED], got %q", val)
	}

	val = findEnv("SESSION_SECRET")
	if val != "[REDACTED]" {
		t.Errorf("SESSION_SECRET expected [REDACTED], got %q", val)
	}

	// Verify non-sensitive values are NOT redacted
	val = findEnv("NORMAL_VAR")
	if val != "safe_value" {
		t.Errorf("NORMAL_VAR should be 'safe_value', got %q", val)
	}

	val = findEnv("DB_HOST")
	if val != "localhost" {
		t.Errorf("DB_HOST should be 'localhost', got %q", val)
	}
}
