package devrouter

import (
	"context"
	"embed"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

//go:embed web/dist/*
var webAssets embed.FS

// ServerConfig holds configuration for the HTTP server
type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Server represents the HTTP server for DevRouter UI
type Server struct {
	config     ServerConfig
	httpServer *http.Server
}

// NewServer creates a new Server with the given configuration
func NewServer(cfg ServerConfig) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":19847"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 15 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 60 * time.Second
	}
	return &Server{config: cfg}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Run initial sync
	SyncRegistry()

	// Start background sync (every 30 seconds)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			SyncRegistry()
		}
	}()

	// Start stats collector for history
	StartGlobalStatsCollector()

	router := s.setupRoutes()
	s.httpServer = &http.Server{
		Addr:         s.config.Addr,
		Handler:      router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop stats collector
	StopGlobalStatsCollector()

	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// Addr returns the configured address
func (s *Server) Addr() string {
	return s.config.Addr
}

func (s *Server) setupRoutes() *mux.Router {
	r := mux.NewRouter()

	// API routes
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/stacks", s.handleListStacks).Methods("GET")
	api.HandleFunc("/stacks/{stackID}", s.handleGetStack).Methods("GET")
	api.HandleFunc("/stacks/{stackID}", s.handleDownStack).Methods("DELETE")
	api.HandleFunc("/stacks/{stackID}/restart", s.handleRestartStack).Methods("POST")
	api.HandleFunc("/stacks/{stackID}/build-restart", s.handleBuildRestartStack).Methods("POST")
	api.HandleFunc("/stacks/{stackID}/health", s.handleStackHealth).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/config", s.handleGetConfig).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/init/preview", s.handleInitPreview).Methods("POST")
	api.HandleFunc("/stacks/{stackID}/init/apply", s.handleInitApply).Methods("POST")
	api.HandleFunc("/stacks/{stackID}/env", s.handleGetEnv).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/services/{serviceName}", s.handleGetService).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/services/{serviceName}/url", s.handleGetServiceURL).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/services/{serviceName}/restart", s.handleRestartService).Methods("POST")
	api.HandleFunc("/stacks/{stackID}/services/{serviceName}/build-restart", s.handleBuildRestartService).Methods("POST")
	api.HandleFunc("/stacks/{stackID}/stats", s.handleStackStats).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/dependencies", s.handleStackDependencies).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/watch", s.handleWatchStatus).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/watch/start", s.handleStartWatch).Methods("POST")
	api.HandleFunc("/stacks/{stackID}/watch/stop", s.handleStopWatch).Methods("POST")
	api.HandleFunc("/stacks/{stackID}/history/events", s.handleHistoryEvents).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/history/stats", s.handleHistoryStats).Methods("GET")
	api.HandleFunc("/stacks/{stackID}/insights", s.handleInsights).Methods("GET")
	api.HandleFunc("/templates", s.handleListTemplates).Methods("GET")
	api.HandleFunc("/templates/{templateID}", s.handleGetTemplate).Methods("GET")
	api.HandleFunc("/health", s.handleHealth).Methods("GET")

	// WebSocket routes
	api.HandleFunc("/ws/logs/{stackID}", s.handleLogsWebSocket).Methods("GET")
	api.HandleFunc("/ws/logs/{stackID}/{serviceName}", s.handleServiceLogsWebSocket).Methods("GET")
	api.HandleFunc("/ws/stats/{stackID}", s.handleStatsWebSocket).Methods("GET")
	api.HandleFunc("/ws/watch/{stackID}", s.handleWatchWebSocket).Methods("GET")

	// Apply CSRF middleware to API routes
	api.Use(s.checkOrigin)

	// Static files (Svelte UI)
	r.PathPrefix("/").Handler(s.spaHandler())

	// Apply security headers to all routes
	r.Use(s.securityHeaders)

	return r
}

// securityHeaders adds security-related headers to all responses
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// CSP allows:
		// - Scripts from self
		// - Styles from self, unsafe-inline (needed for Svelte transitions), and Google Fonts
		// - Fonts from self and Google Fonts (gstatic)
		// - Images from self and data URIs (SVGs)
		// - Connect to self and WebSockets
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:;")
		// Permissions-Policy disables powerful features that are not used
		w.Header().Set("Permissions-Policy", "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}

// validateOrigin validates the Origin URL and ensures no vulnerable IPv4 brackets are used
// See GO-2025-4010: insufficient validation of bracketed IPv6 hostnames
func validateOrigin(origin string) (*url.URL, bool) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return nil, false
	}

	// Check for bracketed host which must be IPv6
	if strings.HasPrefix(u.Host, "[") && strings.HasSuffix(u.Host, "]") {
		hostContent := u.Host[1 : len(u.Host)-1]
		if ip := net.ParseIP(hostContent); ip == nil || ip.To4() != nil {
			// Invalid IP or IPv4 inside brackets - reject
			return nil, false
		}
	}

	return u, true
}

// checkOrigin is a middleware that validates the Origin header to prevent CSRF
func (s *Server) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow non-browser requests (no Origin header)
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Parse and validate Origin
		u, ok := validateOrigin(origin)
		if !ok {
			http.Error(w, "Invalid Origin header", http.StatusForbidden)
			return
		}

		// Allow if Origin host matches the request Host
		if strings.EqualFold(u.Host, r.Host) {
			next.ServeHTTP(w, r)
			return
		}

		// Log warning (could be improved with better logging)
		// fmt.Printf("CSRF: blocked request from %s to %s\n", origin, r.Host)
		http.Error(w, "CSRF: Origin mismatch", http.StatusForbidden)
	})
}

// spaHandler serves static files and falls back to index.html for SPA routing
func (s *Server) spaHandler() http.Handler {
	staticFS, err := fs.Sub(webAssets, "web/dist")
	if err != nil {
		// Return a handler that always returns 404 if embed fails
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "UI assets not found", http.StatusNotFound)
		})
	}

	fileServer := http.FileServer(http.FS(staticFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Try to serve the file
		if path != "/" {
			// Check if file exists
			cleanPath := strings.TrimPrefix(path, "/")
			if _, err := fs.Stat(staticFS, cleanPath); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fallback to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
