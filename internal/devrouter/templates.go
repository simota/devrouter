package devrouter

// ServiceTemplate defines a pre-configured service that can be added to a stack
type ServiceTemplate struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Category    string            `json:"category" yaml:"category"`
	Image       string            `json:"image" yaml:"image"`
	Ports       []string          `json:"ports,omitempty" yaml:"ports,omitempty"`
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	HealthCheck *TemplateHealth   `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
	// HTTPPorts defines which ports should be exposed via Traefik reverse proxy.
	// If empty, only the first port from Ports will be exposed.
	// Example: [{Port: 9000}, {Port: 9001, Suffix: "console"}] generates:
	//   - stack-minio.domain -> :9000
	//   - stack-minio-console.domain -> :9001
	HTTPPorts []HTTPPort `json:"httpPorts,omitempty" yaml:"httpPorts,omitempty"`
	// UI represents the web UI for this service (if any)
	UI *ServiceUI `json:"ui,omitempty" yaml:"ui,omitempty"`
}

// TemplateHealth defines health check config for a template
type TemplateHealth struct {
	Test     []string `json:"test,omitempty" yaml:"test,omitempty"`
	Interval string   `json:"interval,omitempty" yaml:"interval,omitempty"`
	Timeout  string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries  int      `json:"retries,omitempty" yaml:"retries,omitempty"`
}

// ServiceUI represents a web UI service bundled with a data service
type ServiceUI struct {
	Name        string            `json:"name" yaml:"name"`
	Image       string            `json:"image" yaml:"image"`
	Ports       []string          `json:"ports,omitempty" yaml:"ports,omitempty"`
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
}

// HTTPPort defines a port that should be exposed via Traefik reverse proxy.
// Each HTTPPort generates a separate Traefik router with its own Host rule.
type HTTPPort struct {
	Port   int    `json:"port" yaml:"port"`                         // Container port to expose
	Suffix string `json:"suffix,omitempty" yaml:"suffix,omitempty"` // Host suffix (e.g., "console" -> stack-service-console.domain)
}

// TemplateListResponse is the API response for listing templates
type TemplateListResponse struct {
	Templates []ServiceTemplate `json:"templates"`
}

// BuiltInTemplates contains all built-in service templates
var BuiltInTemplates = []ServiceTemplate{
	{
		ID:          "postgres",
		Name:        "PostgreSQL",
		Description: "Powerful open-source relational database",
		Category:    "database",
		Image:       "postgres:16-alpine",
		Ports:       []string{"5432:5432"},
		Environment: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "devrouter",
		},
		Volumes: []string{
			"postgres_data:/var/lib/postgresql/data",
		},
		HealthCheck: &TemplateHealth{
			Test:     []string{"CMD-SHELL", "pg_isready -U postgres"},
			Interval: "10s",
			Timeout:  "5s",
			Retries:  5,
		},
		UI: &ServiceUI{
			Name:  "pgadmin",
			Image: "dpage/pgadmin4:latest",
			Ports: []string{"5050:80"},
			Environment: map[string]string{
				"PGADMIN_DEFAULT_EMAIL":    "admin@local.dev",
				"PGADMIN_DEFAULT_PASSWORD": "admin",
			},
		},
	},
	{
		ID:          "redis",
		Name:        "Redis",
		Description: "In-memory data store for caching and messaging",
		Category:    "cache",
		Image:       "redis:7-alpine",
		Ports:       []string{"6379:6379"},
		HealthCheck: &TemplateHealth{
			Test:     []string{"CMD", "redis-cli", "ping"},
			Interval: "10s",
			Timeout:  "5s",
			Retries:  5,
		},
		UI: &ServiceUI{
			Name:  "redis-commander",
			Image: "rediscommander/redis-commander:latest",
			Ports: []string{"8081:8081"},
			Environment: map[string]string{
				"REDIS_HOSTS": "local:redis:6379",
			},
		},
	},
	{
		ID:          "mongodb",
		Name:        "MongoDB",
		Description: "NoSQL document database",
		Category:    "database",
		Image:       "mongo:7",
		Ports:       []string{"27017:27017"},
		Environment: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": "mongo",
			"MONGO_INITDB_ROOT_PASSWORD": "mongo",
		},
		Volumes: []string{
			"mongo_data:/data/db",
		},
		HealthCheck: &TemplateHealth{
			Test:     []string{"CMD", "mongosh", "--eval", "db.adminCommand('ping')"},
			Interval: "10s",
			Timeout:  "5s",
			Retries:  5,
		},
		UI: &ServiceUI{
			Name:  "mongo-express",
			Image: "mongo-express:latest",
			Ports: []string{"8082:8081"},
			Environment: map[string]string{
				"ME_CONFIG_MONGODB_ADMINUSERNAME": "mongo",
				"ME_CONFIG_MONGODB_ADMINPASSWORD": "mongo",
				"ME_CONFIG_MONGODB_SERVER":        "mongodb",
			},
		},
	},
	{
		ID:          "mysql",
		Name:        "MySQL",
		Description: "Popular open-source relational database",
		Category:    "database",
		Image:       "mysql:8",
		Ports:       []string{"3306:3306"},
		Environment: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root",
			"MYSQL_DATABASE":      "devrouter",
			"MYSQL_USER":          "user",
			"MYSQL_PASSWORD":      "password",
		},
		Volumes: []string{
			"mysql_data:/var/lib/mysql",
		},
		HealthCheck: &TemplateHealth{
			Test:     []string{"CMD", "mysqladmin", "ping", "-h", "localhost"},
			Interval: "10s",
			Timeout:  "5s",
			Retries:  5,
		},
	},
	{
		ID:          "mailhog",
		Name:        "MailHog",
		Description: "Email testing tool for development",
		Category:    "email",
		Image:       "mailhog/mailhog:latest",
		Ports:       []string{"1025:1025", "8025:8025"},
		HTTPPorts: []HTTPPort{
			{Port: 8025}, // Web UI: stack-mailhog.domain
		},
	},
	{
		ID:          "minio",
		Name:        "MinIO",
		Description: "S3-compatible object storage",
		Category:    "storage",
		Image:       "minio/minio:latest",
		Ports:       []string{"9000:9000", "9001:9001"},
		Environment: map[string]string{
			"MINIO_ROOT_USER":     "minio",
			"MINIO_ROOT_PASSWORD": "miniosecret",
		},
		Volumes: []string{
			"minio_data:/data",
		},
		HealthCheck: &TemplateHealth{
			Test:     []string{"CMD", "curl", "-f", "http://localhost:9000/minio/health/live"},
			Interval: "10s",
			Timeout:  "5s",
			Retries:  5,
		},
		HTTPPorts: []HTTPPort{
			{Port: 9000},                    // S3 API: stack-minio.domain
			{Port: 9001, Suffix: "console"}, // Console: stack-minio-console.domain
		},
	},
	{
		ID:          "rabbitmq",
		Name:        "RabbitMQ",
		Description: "Message broker with management UI",
		Category:    "messaging",
		Image:       "rabbitmq:3-management-alpine",
		Ports:       []string{"5672:5672", "15672:15672"},
		Environment: map[string]string{
			"RABBITMQ_DEFAULT_USER": "rabbit",
			"RABBITMQ_DEFAULT_PASS": "rabbit",
		},
		Volumes: []string{
			"rabbitmq_data:/var/lib/rabbitmq",
		},
		HealthCheck: &TemplateHealth{
			Test:     []string{"CMD", "rabbitmq-diagnostics", "-q", "ping"},
			Interval: "10s",
			Timeout:  "5s",
			Retries:  5,
		},
		HTTPPorts: []HTTPPort{
			{Port: 15672}, // Management UI: stack-rabbitmq.domain
		},
	},
	{
		ID:          "elasticsearch",
		Name:        "Elasticsearch",
		Description: "Search and analytics engine",
		Category:    "search",
		Image:       "docker.elastic.co/elasticsearch/elasticsearch:8.11.0",
		Ports:       []string{"9200:9200", "9300:9300"},
		Environment: map[string]string{
			"discovery.type":         "single-node",
			"xpack.security.enabled": "false",
			"ES_JAVA_OPTS":           "-Xms512m -Xmx512m",
		},
		Volumes: []string{
			"elasticsearch_data:/usr/share/elasticsearch/data",
		},
		HealthCheck: &TemplateHealth{
			Test:     []string{"CMD-SHELL", "curl -f http://localhost:9200/_cluster/health || exit 1"},
			Interval: "30s",
			Timeout:  "10s",
			Retries:  5,
		},
		HTTPPorts: []HTTPPort{
			{Port: 9200}, // HTTP API: stack-elasticsearch.domain
		},
	},
}

// GetTemplate returns a template by ID
func GetTemplate(id string) *ServiceTemplate {
	for _, t := range BuiltInTemplates {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

// ListTemplates returns all available templates
func ListTemplates() []ServiceTemplate {
	return BuiltInTemplates
}
