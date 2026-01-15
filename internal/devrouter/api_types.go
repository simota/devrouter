package devrouter

// APIError represents an error response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MessageResponse represents a simple message response
type MessageResponse struct {
	Message string `json:"message"`
}

// StackListResponse represents the response for listing stacks
type StackListResponse struct {
	Stacks []StackSummary `json:"stacks"`
}

// StackSummary represents a brief overview of a stack
type StackSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"`
	Domain       string `json:"domain"`
	ServiceCount int    `json:"serviceCount"`
	// LastUpAt is stored as string to avoid unnecessary parsing/formatting round-trips
	LastUpAt string `json:"lastUpAt"`
}

// StackResponse represents a detailed stack response
type StackResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	RepoPath        string `json:"repoPath"`
	Source          string `json:"source"`
	Domain          string `json:"domain"`
	ComposeFilePath string `json:"composeFilePath"`
	// Timestamps are stored as string to avoid unnecessary parsing/formatting round-trips
	LastUpAt   string            `json:"lastUpAt"`
	LastDownAt string            `json:"lastDownAt,omitempty"`
	Services   []ServiceResponse `json:"services"`
}

// ServiceResponse represents a service in API responses
type ServiceResponse struct {
	Name           string            `json:"name"`
	WorkspacePath  string            `json:"workspacePath"`
	Host           string            `json:"host"`
	URL            string            `json:"url"`
	Port           int               `json:"port"`
	ComposeService string            `json:"composeService"`
	HealthCheck    HealthCheckConfig `json:"healthCheck,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Docker    bool   `json:"docker"`
	Timestamp string `json:"timestamp"`
}

// LogMessage represents a log entry sent via WebSocket
type LogMessage struct {
	Type      string `json:"type"`
	Service   string `json:"service,omitempty"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
}

// ServiceHealthStatus represents the health status of a service
type ServiceHealthStatus struct {
	Name           string `json:"name"`
	Status         string `json:"status"`         // "healthy", "unhealthy", "unknown"
	ContainerState string `json:"containerState"` // "running", "stopped", "exited", "not_found"
	URL            string `json:"url"`
	Latency        int    `json:"latency"` // in milliseconds
	CheckedAt      string `json:"checkedAt"`
}

// StackHealthResponse represents health status for all services in a stack
type StackHealthResponse struct {
	StackID   string                `json:"stackId"`
	Services  []ServiceHealthStatus `json:"services"`
	CheckedAt string                `json:"checkedAt"`
}

// ConfigFileResponse represents a config file content
type ConfigFileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Type    string `json:"type"` // "compose", "devrouter"
}

// InitConfigRequest represents init config input from the UI.
type InitConfigRequest struct {
	Stack  string `json:"stack,omitempty"`
	Domain string `json:"domain,omitempty"`
	Force  bool   `json:"force,omitempty"`
}

// InitConfigResponse represents init config preview/apply result.
type InitConfigResponse struct {
	OutputPath    string   `json:"outputPath"`
	Content       string   `json:"content"`
	Warnings      []string `json:"warnings"`
	ApplyAllowed  bool     `json:"applyAllowed"`
	RequiresForce bool     `json:"requiresForce"`
	Source        string   `json:"source"`
	Applied       bool     `json:"applied,omitempty"`
}

// EnvVarItem represents a single environment variable
type EnvVarItem struct {
	Service string `json:"service"`
	Name    string `json:"name"`
	Value   string `json:"value"`
}

// EnvResponse represents environment variables for a stack
type EnvResponse struct {
	StackID string       `json:"stackId"`
	Env     []EnvVarItem `json:"env"`
}

// HistoryEventsResponse represents the response for stack history events
type HistoryEventsResponse struct {
	StackID string         `json:"stackId"`
	Events  []HistoryEvent `json:"events"`
}

// HistoryStatsResponse represents the response for stack stats history
type HistoryStatsResponse struct {
	StackID string       `json:"stackId"`
	Stats   []StatsEntry `json:"stats"`
	Range   string       `json:"range"`
}
