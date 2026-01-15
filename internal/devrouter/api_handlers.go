package devrouter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

const maxJSONBodySize = 1048576 // 1MB

// healthClient is a shared HTTP client for health checks to enable connection reuse
var healthClient = &http.Client{
	Timeout: 0, // Use request context for timeout
	Transport: func() *http.Transport {
		// Clone DefaultTransport to preserve defaults (proxy, keep-alive, etc.)
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConns = 100
		t.MaxIdleConnsPerHost = 10
		t.IdleConnTimeout = 90 * time.Second
		return t
	}(),
}

// writeJSON writes a JSON response with the given status code
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	s.writeJSON(w, status, APIError{Code: code, Message: message})
}

// handleListStacks returns a list of all stacks
func (s *Server) handleListStacks(w http.ResponseWriter, r *http.Request) {
	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stacks := make([]StackSummary, 0, len(reg.Stacks))
	for _, stack := range reg.Stacks {
		stacks = append(stacks, StackSummary{
			ID:           stack.ID,
			Name:         stack.Name,
			Source:       stack.Source,
			Domain:       stack.Domain,
			ServiceCount: len(stack.Services),
			LastUpAt:     stack.LastUpAt,
		})
	}
	s.writeJSON(w, http.StatusOK, StackListResponse{Stacks: stacks})
}

// handleGetStack returns details of a specific stack
func (s *Server) handleGetStack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	services := make([]ServiceResponse, 0, len(stack.Services))
	for _, svc := range stack.Services {
		services = append(services, ServiceResponse{
			Name:           svc.Name,
			WorkspacePath:  svc.WorkspacePath,
			Host:           svc.Host,
			URL:            svc.URL,
			Port:           svc.Port,
			ComposeService: svc.ComposeService,
			HealthCheck:    svc.HealthCheck,
		})
	}

	s.writeJSON(w, http.StatusOK, StackResponse{
		ID:              stack.ID,
		Name:            stack.Name,
		RepoPath:        stack.RepoPath,
		Source:          stack.Source,
		Domain:          stack.Domain,
		ComposeFilePath: stack.ComposeFilePath,
		LastUpAt:        stack.LastUpAt,
		LastDownAt:      stack.LastDownAt,
		Services:        services,
	})
}

// handleDownStack stops a stack
func (s *Server) handleDownStack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	_, err := Down(DownOptions{Target: stackID})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "DOWN_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, MessageResponse{Message: "stack stopped: " + stackID})
}

// handleGetService returns details of a specific service
func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]
	serviceName := vars["serviceName"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	svc, ok := FindService(stack, serviceName)
	if !ok {
		s.writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found: "+serviceName)
		return
	}

	s.writeJSON(w, http.StatusOK, ServiceResponse{
		Name:           svc.Name,
		WorkspacePath:  svc.WorkspacePath,
		Host:           svc.Host,
		URL:            svc.URL,
		Port:           svc.Port,
		ComposeService: svc.ComposeService,
		HealthCheck:    svc.HealthCheck,
	})
}

// handleGetServiceURL redirects to the service URL
func (s *Server) handleGetServiceURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]
	serviceName := vars["serviceName"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	svc, ok := FindService(stack, serviceName)
	if !ok {
		s.writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found: "+serviceName)
		return
	}

	http.Redirect(w, r, svc.URL, http.StatusFound)
}

// handleHealth returns the health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dockerOK := CheckDocker() == nil

	status := "ok"
	if !dockerOK {
		status = "degraded"
	}

	s.writeJSON(w, http.StatusOK, HealthResponse{
		Status:    status,
		Docker:    dockerOK,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// handleStackHealth checks health of all services in a stack
func (s *Server) handleStackHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	// Get container states
	containerStates, _ := ComposePs(stack.ComposeFilePath)
	if containerStates == nil {
		containerStates = make(map[string]string)
	}

	// Check health of all services concurrently
	var wg sync.WaitGroup
	results := make([]ServiceHealthStatus, len(stack.Services))

	for i, svc := range stack.Services {
		wg.Add(1)
		go func(idx int, service ServiceRecord) {
			defer wg.Done()
			results[idx] = checkServiceHealth(service, containerStates)
		}(i, svc)
	}

	wg.Wait()

	s.writeJSON(w, http.StatusOK, StackHealthResponse{
		StackID:   stackID,
		Services:  results,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// checkServiceHealth performs a health check on a single service
func checkServiceHealth(svc ServiceRecord, containerStates map[string]string) ServiceHealthStatus {
	cfg := svc.HealthCheck

	// Build check URL with optional endpoint
	checkURL := svc.URL
	if cfg.Endpoint != "" {
		checkURL = svc.URL + cfg.Endpoint
	}

	// Determine container state
	containerState := containerStates[svc.ComposeService]
	if containerState == "" {
		containerState = "not_found"
	}

	status := ServiceHealthStatus{
		Name:           svc.Name,
		URL:            checkURL,
		Status:         "unknown",
		ContainerState: containerState,
		Latency:        0,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	// Skip HTTP health check if container is not running
	if containerState != "running" {
		return status
	}

	if svc.URL == "" {
		return status
	}

	// Determine timeout (default: 5 seconds)
	timeout := 5 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Millisecond
	}

	// Determine HTTP method (default: GET)
	method := "GET"
	if cfg.Method != "" {
		method = cfg.Method
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, checkURL, nil)
	if err != nil {
		status.Status = "unhealthy"
		return status
	}

	start := time.Now()
	resp, err := healthClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		status.Status = "unhealthy"
		status.Latency = int(latency.Milliseconds())
		return status
	}
	defer resp.Body.Close()

	status.Latency = int(latency.Milliseconds())

	// Check status code
	if len(cfg.Status) > 0 {
		// Use configured status codes
		for _, acceptedStatus := range cfg.Status {
			if resp.StatusCode == acceptedStatus {
				status.Status = "healthy"
				return status
			}
		}
		status.Status = "unhealthy"
	} else {
		// Default: 200-399 is healthy
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			status.Status = "healthy"
		} else {
			status.Status = "unhealthy"
		}
	}

	return status
}

// handleGetConfig returns the compose file content for a stack
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	content, err := os.ReadFile(stack.ComposeFilePath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "FILE_READ_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, ConfigFileResponse{
		Path:    stack.ComposeFilePath,
		Content: string(content),
		Type:    "compose",
	})
}

func (s *Server) handleInitPreview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	req, err := parseInitConfigRequest(w, r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INIT_REQUEST_INVALID", err.Error())
		return
	}

	preview, err := PreviewInitConfig(InitPreviewOptions{
		RepoPath:    stack.RepoPath,
		ComposePath: composePathForInit(stack),
		Stack:       req.Stack,
		Domain:      req.Domain,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "INIT_PREVIEW_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, InitConfigResponse{
		OutputPath:    preview.OutputPath,
		Content:       preview.Content,
		Warnings:      preview.Warnings,
		ApplyAllowed:  preview.ApplyAllowed,
		RequiresForce: preview.RequiresForce,
		Source:        stack.Source,
	})
}

func (s *Server) handleInitApply(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	req, err := parseInitConfigRequest(w, r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INIT_REQUEST_INVALID", err.Error())
		return
	}

	applied, err := ApplyInitConfig(InitApplyOptions{
		RepoPath:    stack.RepoPath,
		ComposePath: composePathForInit(stack),
		Stack:       req.Stack,
		Domain:      req.Domain,
		Force:       req.Force,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInitComposeMissing):
			s.writeError(w, http.StatusBadRequest, "INIT_COMPOSE_MISSING", "compose file not found; apply is disabled")
		case errors.Is(err, ErrInitOutputExists):
			s.writeError(w, http.StatusConflict, "INIT_OUTPUT_EXISTS", "devrouter.yaml already exists; use force to overwrite")
		default:
			s.writeError(w, http.StatusInternalServerError, "INIT_APPLY_ERROR", err.Error())
		}
		return
	}

	s.writeJSON(w, http.StatusOK, InitConfigResponse{
		OutputPath:    applied.OutputPath,
		Content:       applied.Content,
		Warnings:      applied.Warnings,
		ApplyAllowed:  applied.ApplyAllowed,
		RequiresForce: applied.RequiresForce,
		Source:        stack.Source,
		Applied:       true,
	})
}

func parseInitConfigRequest(w http.ResponseWriter, r *http.Request) (InitConfigRequest, error) {
	var req InitConfigRequest
	if r.Body == nil {
		return req, nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return InitConfigRequest{}, err
	}
	req.Stack = strings.TrimSpace(req.Stack)
	req.Domain = strings.TrimSpace(req.Domain)
	return req, nil
}

func composePathForInit(stack StackRecord) string {
	if stack.Source != "compose" {
		return ""
	}
	return stack.ComposeFilePath
}

// handleGetEnv returns environment variables for a stack
func (s *Server) handleGetEnv(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	envItems, err := extractEnvFromCompose(stack.ComposeFilePath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "PARSE_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, EnvResponse{
		StackID: stackID,
		Env:     envItems,
	})
}

// extractEnvFromCompose parses a compose file and extracts environment variables
func extractEnvFromCompose(composePath string) ([]EnvVarItem, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, err
	}

	var compose struct {
		Services map[string]struct {
			Environment interface{} `yaml:"environment"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, err
	}

	var items []EnvVarItem
	for svcName, svc := range compose.Services {
		envMap := parseEnvField(svc.Environment)
		for k, v := range envMap {
			items = append(items, EnvVarItem{
				Service: svcName,
				Name:    k,
				Value:   redactSensitiveValue(k, v),
			})
		}
	}

	// Sort by service name, then by env name
	sort.Slice(items, func(i, j int) bool {
		if items[i].Service != items[j].Service {
			return items[i].Service < items[j].Service
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

// parseEnvField handles both map and list formats of environment variables
// redactSensitiveValue checks if the key matches sensitive patterns and redacts the value
func redactSensitiveValue(key, value string) string {
	if value == "" {
		return ""
	}
	// Case-insensitive check for sensitive keywords
	k := strings.ToUpper(key)
	if strings.Contains(k, "PASSWORD") ||
		strings.Contains(k, "SECRET") ||
		strings.Contains(k, "KEY") ||
		strings.Contains(k, "TOKEN") ||
		strings.Contains(k, "CREDENTIAL") ||
		strings.Contains(k, "AUTH") ||
		strings.Contains(k, "PRIVATE") ||
		strings.Contains(k, "CERT") ||
		strings.Contains(k, "SIGNATURE") ||
		strings.Contains(k, "WEBHOOK") ||
		strings.Contains(k, "SESSION") ||
		strings.Contains(k, "COOKIE") ||
		strings.Contains(k, "BEARER") {
		return "[REDACTED]"
	}
	return value
}

func parseEnvField(env interface{}) map[string]string {
	result := make(map[string]string)
	if env == nil {
		return result
	}

	switch v := env.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if s, ok := val.(string); ok {
				result[key] = s
			} else {
				result[key] = ""
			}
		}
	case map[interface{}]interface{}:
		for key, val := range v {
			keyStr, ok := key.(string)
			if !ok {
				continue
			}
			if s, ok := val.(string); ok {
				result[keyStr] = s
			} else {
				result[keyStr] = ""
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts := strings.SplitN(s, "=", 2)
				if len(parts) == 2 {
					result[parts[0]] = parts[1]
				} else if len(parts) == 1 {
					result[parts[0]] = ""
				}
			}
		}
	}

	return result
}

// handleRestartStack restarts all services in a stack
func (s *Server) handleRestartStack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	if err := ComposeRestart(stack.ComposeFilePath, ""); err != nil {
		s.writeError(w, http.StatusInternalServerError, "RESTART_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, MessageResponse{Message: "stack restarted: " + stackID})
}

// handleRestartService restarts a specific service in a stack
func (s *Server) handleRestartService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]
	serviceName := vars["serviceName"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	svc, ok := FindService(stack, serviceName)
	if !ok {
		s.writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found: "+serviceName)
		return
	}

	if err := ComposeRestart(stack.ComposeFilePath, svc.ComposeService); err != nil {
		s.writeError(w, http.StatusInternalServerError, "RESTART_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, MessageResponse{Message: "service restarted: " + serviceName})
}

// handleBuildRestartStack rebuilds and restarts all services in a stack
func (s *Server) handleBuildRestartStack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	if err := ComposeBuildRestart(stack.ComposeFilePath, ""); err != nil {
		s.writeError(w, http.StatusInternalServerError, "BUILD_RESTART_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, MessageResponse{Message: "stack rebuilt and restarted: " + stackID})
}

// handleBuildRestartService rebuilds and restarts a specific service in a stack
func (s *Server) handleBuildRestartService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]
	serviceName := vars["serviceName"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	svc, ok := FindService(stack, serviceName)
	if !ok {
		s.writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found: "+serviceName)
		return
	}

	if err := ComposeBuildRestart(stack.ComposeFilePath, svc.ComposeService); err != nil {
		s.writeError(w, http.StatusInternalServerError, "BUILD_RESTART_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, MessageResponse{Message: "service rebuilt and restarted: " + serviceName})
}

// handleStackStats returns resource usage statistics for a stack
func (s *Server) handleStackStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	stats, err := GetStackStats(stack.ComposeFilePath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "STATS_ERROR", err.Error())
		return
	}

	stats.StackID = stackID
	s.writeJSON(w, http.StatusOK, stats)
}

// handleListTemplates returns all available service templates
func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := ListTemplates()
	s.writeJSON(w, http.StatusOK, TemplateListResponse{Templates: templates})
}

// handleGetTemplate returns a specific template by ID
func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateID := vars["templateID"]

	template := GetTemplate(templateID)
	if template == nil {
		s.writeError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "template not found: "+templateID)
		return
	}

	s.writeJSON(w, http.StatusOK, template)
}

// handleStackDependencies returns the dependency graph for a stack
func (s *Server) handleStackDependencies(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	deps, err := AnalyzeDependencies(stack.ComposeFilePath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "DEPS_ERROR", err.Error())
		return
	}

	deps.StackID = stackID
	s.writeJSON(w, http.StatusOK, deps)
}

// handleWatchStatus returns the file watching status for a stack
func (s *Server) handleWatchStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	status := GetWatchStatus(stackID)
	if status == nil {
		s.writeJSON(w, http.StatusOK, WatchStatus{
			Enabled:  false,
			Watching: false,
		})
		return
	}

	s.writeJSON(w, http.StatusOK, status)
}

// handleStartWatch starts file watching for a stack
func (s *Server) handleStartWatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	reg, err := LoadRegistry()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "REGISTRY_ERROR", err.Error())
		return
	}

	stack, ok := FindStack(reg, stackID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "STACK_NOT_FOUND", "stack not found: "+stackID)
		return
	}

	// Parse watch config from request body (optional)
	var config WatchConfig
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
		json.NewDecoder(r.Body).Decode(&config)
	}
	config.Enabled = true

	// Set default rules if none provided
	if len(config.Rules) == 0 {
		config.Rules = []WatchRule{
			{Pattern: "*.go", Action: watchActionRestart},
			{Pattern: "*.ts", Action: watchActionRestart},
			{Pattern: "*.tsx", Action: watchActionRestart},
			{Pattern: "*.js", Action: watchActionRestart},
			{Pattern: "*.jsx", Action: watchActionRestart},
			{Pattern: "*.py", Action: watchActionRestart},
		}
	}

	if err := StartWatching(stackID, stack.RepoPath, stack.ComposeFilePath, stack.Services, config); err != nil {
		s.writeError(w, http.StatusInternalServerError, "WATCH_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, MessageResponse{Message: "watching started for " + stackID})
}

// handleStopWatch stops file watching for a stack
func (s *Server) handleStopWatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	StopWatching(stackID)
	s.writeJSON(w, http.StatusOK, MessageResponse{Message: "watching stopped for " + stackID})
}

// handleHistoryEvents returns the history events for a stack
func (s *Server) handleHistoryEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	// Parse optional 'since' query parameter (default: 24 hours ago)
	since := time.Now().UTC().Add(-24 * time.Hour)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	events, err := LoadEvents(stackID, since)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "HISTORY_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, HistoryEventsResponse{
		StackID: stackID,
		Events:  events,
	})
}

// handleHistoryStats returns the stats history for a stack
func (s *Server) handleHistoryStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	// Parse 'range' query parameter (1h, 6h, 24h)
	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "1h"
	}

	var duration time.Duration
	switch rangeStr {
	case "1h":
		duration = 1 * time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	default:
		duration = 1 * time.Hour
		rangeStr = "1h"
	}

	now := time.Now().UTC()
	since := now.Add(-duration)

	stats, err := LoadStats(stackID, since, now)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "HISTORY_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, HistoryStatsResponse{
		StackID: stackID,
		Stats:   stats,
		Range:   rangeStr,
	})
}

// handleInsights returns computed insights for a stack
func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["stackID"]

	insights, err := ComputeInsights(stackID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "INSIGHTS_ERROR", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, insights)
}
