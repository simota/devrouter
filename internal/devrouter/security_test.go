package devrouter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	cfg := ServerConfig{Addr: ":8080"}
	s := NewServer(cfg)
	handler := s.setupRoutes()

	req, _ := http.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
	}

	for key, expectedValue := range headers {
		if got := rr.Header().Get(key); got != expectedValue {
			t.Errorf("Security header %s mismatch: got %q, want %q", key, got, expectedValue)
		}
	}

	// Verify Content-Security-Policy
	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Security header Content-Security-Policy missing")
	}
	expectedDirectives := []string{
		"default-src 'self'",
		"frame-ancestors 'self'",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com",
		"script-src 'self'",
		"connect-src 'self' ws: wss:",
		"img-src 'self' data:",
	}
	for _, directive := range expectedDirectives {
		if !containsDirective(csp, directive) {
			t.Errorf("CSP missing directive %q: got %q", directive, csp)
		}
	}
}

func containsDirective(csp, directive string) bool {
	// Simple check, robust enough for this test
	return strings.Contains(csp, directive)
}

func TestCSRFProtection(t *testing.T) {
	cfg := ServerConfig{Addr: ":8080"}
	s := NewServer(cfg)
	handler := s.setupRoutes()

	tests := []struct {
		name           string
		origin         string
		host           string
		expectedStatus int
	}{
		{
			name:           "Valid Origin",
			origin:         "http://localhost:8080",
			host:           "localhost:8080",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Origin",
			origin:         "http://evil.com",
			host:           "localhost:8080",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "No Origin (Non-browser)",
			origin:         "",
			host:           "localhost:8080",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Bracketed IPv4 Origin (Vulnerability Check - Mismatch)",
			origin:         "http://[127.0.0.1]",
			host:           "127.0.0.1",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Bracketed IPv4 Origin (Vulnerability Check - Matching)",
			origin:         "http://[127.0.0.1]",
			host:           "[127.0.0.1]",
			expectedStatus: http.StatusForbidden, // Should fail even if they match because it's invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/health", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			req.Host = tt.host

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestWebSocketCheckOrigin(t *testing.T) {
	// The upgrader variable is package-level in ws_handlers.go
	// We can test the CheckOrigin function directly

	checkOrigin := upgrader.CheckOrigin

	tests := []struct {
		name     string
		host     string
		origin   string
		expected bool
	}{
		{
			name:     "No Origin Header (Non-browser)",
			host:     "localhost:8080",
			origin:   "",
			expected: true,
		},
		{
			name:     "Matching Origin",
			host:     "localhost:8080",
			origin:   "http://localhost:8080",
			expected: true,
		},
		{
			name:     "Mismatched Origin",
			host:     "localhost:8080",
			origin:   "http://evil.com",
			expected: false,
		},
		{
			name:     "Invalid Origin URL",
			host:     "localhost:8080",
			origin:   "://invalid-url",
			expected: false,
		},
		{
			name:     "Matching Origin (Case Insensitive)",
			host:     "LocalHost:8080",
			origin:   "http://localhost:8080",
			expected: true,
		},
		{
			name:     "Subdomain mismatch",
			host:     "app.local",
			origin:   "http://evil.app.local",
			expected: false,
		},
		{
			name:     "Bracketed IPv4 Origin (Vulnerability Check)",
			host:     "127.0.0.1",
			origin:   "http://[127.0.0.1]",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			if got := checkOrigin(req); got != tt.expected {
				t.Errorf("CheckOrigin() = %v, want %v (Host: %s, Origin: %s)", got, tt.expected, tt.host, tt.origin)
			}
		})
	}
}
