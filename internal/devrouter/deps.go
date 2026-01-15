package devrouter

import (
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServiceDependency represents a dependency relationship between services
type ServiceDependency struct {
	From    string `json:"from"`    // Service that depends on another
	To      string `json:"to"`      // Service being depended on
	EnvVar  string `json:"envVar"`  // Environment variable that defines the dependency
	URL     string `json:"url"`     // The URL or value in the env var
	Type    string `json:"type"`    // Type of dependency: "url", "hostname", "env_ref"
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	Name         string   `json:"name"`
	Dependencies []string `json:"dependencies"` // Services this node depends on
	Dependents   []string `json:"dependents"`   // Services that depend on this node
}

// StackDependenciesResponse represents the dependency graph for a stack
type StackDependenciesResponse struct {
	StackID      string              `json:"stackId"`
	Nodes        []DependencyNode    `json:"nodes"`
	Edges        []ServiceDependency `json:"edges"`
	ServiceNames []string            `json:"serviceNames"`
}

// URL pattern to detect service references in environment variables.
// We only match compose-style service names (no dots) + port to avoid
// flagging external domains or IPs as internal dependencies.
var serviceURLPattern = regexp.MustCompile(`(?i)(?:https?://)?([a-z][a-z0-9_-]*):(\d+)`)

// Hostname pattern to detect service references.
// This keeps hostname-only checks conservative (service names only).
var hostnamePattern = regexp.MustCompile(`(?i)^([a-z][a-z0-9_-]*)$`)

// AnalyzeDependencies analyzes the compose file and extracts service dependencies
func AnalyzeDependencies(composePath string) (*StackDependenciesResponse, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, err
	}

	var compose struct {
		Services map[string]struct {
			DependsOn   interface{} `yaml:"depends_on"`
			Environment interface{} `yaml:"environment"`
			Links       []string    `yaml:"links"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, err
	}

	serviceNames := make([]string, 0, len(compose.Services))
	serviceSet := make(map[string]bool)
	for name := range compose.Services {
		serviceNames = append(serviceNames, name)
		serviceSet[name] = true
	}

	var edges []ServiceDependency
	nodeDeps := make(map[string][]string)
	nodeRefs := make(map[string][]string)

	// Initialize maps for all services
	for name := range compose.Services {
		nodeDeps[name] = []string{}
		nodeRefs[name] = []string{}
	}

	for serviceName, svc := range compose.Services {
		// Check depends_on
		deps := parseDependsOn(svc.DependsOn)
		for _, dep := range deps {
			if serviceSet[dep] {
				edges = append(edges, ServiceDependency{
					From: serviceName,
					To:   dep,
					Type: "depends_on",
				})
				nodeDeps[serviceName] = appendUnique(nodeDeps[serviceName], dep)
				nodeRefs[dep] = appendUnique(nodeRefs[dep], serviceName)
			}
		}

		// Check links
		for _, link := range svc.Links {
			// Links can be "service" or "service:alias"
			parts := strings.SplitN(link, ":", 2)
			target := parts[0]
			if serviceSet[target] {
				edges = append(edges, ServiceDependency{
					From: serviceName,
					To:   target,
					Type: "link",
				})
				nodeDeps[serviceName] = appendUnique(nodeDeps[serviceName], target)
				nodeRefs[target] = appendUnique(nodeRefs[target], serviceName)
			}
		}

		// Check environment variables for service references
		envMap := parseEnvField(svc.Environment)
		for envName, envValue := range envMap {
			// Look for URL patterns that might reference other services
			matches := serviceURLPattern.FindAllStringSubmatch(envValue, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					targetService := match[1]
					if serviceSet[targetService] && targetService != serviceName {
						edges = append(edges, ServiceDependency{
							From:   serviceName,
							To:     targetService,
							EnvVar: envName,
							URL:    envValue,
							Type:   "url",
						})
						nodeDeps[serviceName] = appendUnique(nodeDeps[serviceName], targetService)
						nodeRefs[targetService] = appendUnique(nodeRefs[targetService], serviceName)
					}
				}
			}

			// Check for hostname-only references in common env vars
			if isServiceRefEnvVar(envName) && hostnamePattern.MatchString(envValue) {
				if serviceSet[envValue] && envValue != serviceName {
					edges = append(edges, ServiceDependency{
						From:   serviceName,
						To:     envValue,
						EnvVar: envName,
						URL:    envValue,
						Type:   "hostname",
					})
					nodeDeps[serviceName] = appendUnique(nodeDeps[serviceName], envValue)
					nodeRefs[envValue] = appendUnique(nodeRefs[envValue], serviceName)
				}
			}
		}
	}

	// Build nodes
	nodes := make([]DependencyNode, 0, len(compose.Services))
	for name := range compose.Services {
		nodes = append(nodes, DependencyNode{
			Name:         name,
			Dependencies: nodeDeps[name],
			Dependents:   nodeRefs[name],
		})
	}

	return &StackDependenciesResponse{
		Nodes:        nodes,
		Edges:        edges,
		ServiceNames: serviceNames,
	}, nil
}

// parseDependsOn handles both list and map formats of depends_on
func parseDependsOn(dependsOn interface{}) []string {
	if dependsOn == nil {
		return nil
	}

	var result []string
	switch v := dependsOn.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	case map[string]interface{}:
		for key := range v {
			result = append(result, key)
		}
	case map[interface{}]interface{}:
		for key := range v {
			if s, ok := key.(string); ok {
				result = append(result, s)
			}
		}
	}
	return result
}

// isServiceRefEnvVar checks if an environment variable name typically contains a service reference
func isServiceRefEnvVar(name string) bool {
	name = strings.ToUpper(name)
	patterns := []string{
		"_HOST", "_HOSTNAME", "_SERVER", "_SERVICE",
		"DATABASE_HOST", "DB_HOST", "REDIS_HOST", "CACHE_HOST",
		"API_HOST", "BACKEND_HOST", "FRONTEND_HOST",
	}
	for _, pattern := range patterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}
	return false
}

// appendUnique appends an item to a slice only if it doesn't already exist
func appendUnique(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}
