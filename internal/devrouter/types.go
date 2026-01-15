package devrouter

type StackSpec struct {
	ID       string
	Name     string
	RepoPath string
	Domain   string
	Services []ServiceSpec
}

type ServiceSpec struct {
	Name           string
	WorkspacePath  string
	WorkspaceAbs   string
	Command        string
	Port           int
	Env            map[string]string
	Host           string
	ComposeService string
	HealthCheck    HealthCheckConfig
	// ExtraPorts defines additional HTTP ports exposed via Traefik with host suffix.
	// Example: [{Port: 9001, Suffix: "console"}] -> stack-service-console.domain:443 -> :9001
	ExtraPorts []HTTPPort
}

type Config struct {
	Stack     string                       `yaml:"stack"`
	Compose   string                       `yaml:"compose"`
	Domain    string                       `yaml:"domain"`
	TLS       TLSConfig                    `yaml:"tls"`
	Defaults  DefaultsConfig               `yaml:"defaults"`
	Overrides map[string]ServiceConfigDiff `yaml:"overrides"`
	Watch     WatchConfigYAML              `yaml:"watch"`
}

// WatchConfigYAML is the YAML representation of watch config
type WatchConfigYAML struct {
	Enabled  bool            `yaml:"enabled"`
	Debounce string          `yaml:"debounce"` // e.g., "500ms", "1s"
	Rules    []WatchRuleYAML `yaml:"rules"`
}

// WatchRuleYAML is the YAML representation of a watch rule
type WatchRuleYAML struct {
	Pattern string `yaml:"pattern"`
	Action  string `yaml:"action"`
}

type DefaultsConfig struct {
	Env map[string]string `yaml:"env"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

type ServiceConfigDiff struct {
	Name        string            `yaml:"name"`
	Port        int               `yaml:"port"`
	Command     string            `yaml:"command"`
	Host        string            `yaml:"host"`
	Env         map[string]string `yaml:"env"`
	HealthCheck HealthCheckConfig `yaml:"healthcheck"`
}

// HealthCheckConfig はサービスごとのヘルスチェック設定
type HealthCheckConfig struct {
	Endpoint string `yaml:"endpoint" json:"endpoint,omitempty"` // "/health" など
	Method   string `yaml:"method" json:"method,omitempty"`     // GET, POST, HEAD
	Timeout  int    `yaml:"timeout" json:"timeout,omitempty"`   // ミリ秒
	Status   []int  `yaml:"status" json:"status,omitempty"`     // 正常ステータスコード
}

type Registry struct {
	Version int           `json:"version"`
	Stacks  []StackRecord `json:"stacks"`
}

type StackRecord struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	RepoPath        string          `json:"repoPath"`
	Source          string          `json:"source"`
	Domain          string          `json:"domain"`
	ComposeFilePath string          `json:"composeFilePath"`
	LastUpAt        string          `json:"lastUpAt"`
	LastDownAt      string          `json:"lastDownAt,omitempty"`
	Services        []ServiceRecord `json:"services"`
}

type ServiceRecord struct {
	Name           string            `json:"name"`
	WorkspacePath  string            `json:"workspacePath"`
	Host           string            `json:"host"`
	URL            string            `json:"url"`
	Port           int               `json:"port"`
	ComposeService string            `json:"composeService"`
	HealthCheck    HealthCheckConfig `json:"healthCheck,omitempty"`
}

type ExecOptions struct {
	User    string
	Workdir string
}
