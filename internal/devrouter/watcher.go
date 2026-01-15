package devrouter

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	watchActionRestart    = "restart"
	watchActionRestartAll = "restart-all"
	watchActionIgnore     = "ignore"

	watchEventCreated  = "created"
	watchEventModified = "modified"
	watchEventDeleted  = "deleted"
)

// WatchConfig holds file watching configuration
type WatchConfig struct {
	Enabled  bool          `yaml:"enabled" json:"enabled"`
	Debounce time.Duration `yaml:"debounce" json:"debounce"`
	Rules    []WatchRule   `yaml:"rules" json:"rules"`
}

// WatchRule defines a pattern and action for file changes
type WatchRule struct {
	Pattern string `yaml:"pattern" json:"pattern"` // glob pattern like "apps/web/**/*.ts"
	Action  string `yaml:"action" json:"action"`   // restart, restart-all, ignore
}

// WatchEvent represents a file change event
type WatchEvent struct {
	Path          string    `json:"path"`
	Action        string    `json:"action"` // created, modified, deleted
	Timestamp     time.Time `json:"timestamp"`
	MatchedRule   string    `json:"matchedRule,omitempty"`
	TargetService string    `json:"targetService,omitempty"`
}

// WatchStatus represents the current status of file watching
type WatchStatus struct {
	Enabled      bool         `json:"enabled"`
	Watching     bool         `json:"watching"`
	RepoPath     string       `json:"repoPath"`
	WatchedPaths int          `json:"watchedPaths"`
	RecentEvents []WatchEvent `json:"recentEvents"`
	Rules        []WatchRule  `json:"rules"`
}

// StackWatcher watches files for a specific stack
type StackWatcher struct {
	stackID         string
	repoPath        string
	composePath     string
	config          WatchConfig
	watcher         *fsnotify.Watcher
	mu              sync.RWMutex
	events          []WatchEvent
	maxEvents       int
	debounceMap     map[string]*time.Timer
	debounceMu      sync.Mutex
	subscribers     map[chan WatchEvent]bool
	subMu           sync.RWMutex
	stopCh          chan struct{}
	running         bool
	serviceMatchers []serviceMatcher
}

// Global watcher registry
var (
	watcherRegistry   = make(map[string]*StackWatcher)
	watcherRegistryMu sync.RWMutex
)

// NewStackWatcher creates a new file watcher for a stack
func NewStackWatcher(stackID, repoPath, composePath string, services []ServiceRecord, config WatchConfig) (*StackWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	sw := &StackWatcher{
		stackID:         stackID,
		repoPath:        repoPath,
		composePath:     composePath,
		config:          config,
		watcher:         watcher,
		events:          make([]WatchEvent, 0),
		maxEvents:       100,
		debounceMap:     make(map[string]*time.Timer),
		subscribers:     make(map[chan WatchEvent]bool),
		stopCh:          make(chan struct{}),
		serviceMatchers: buildServiceMatchers(services),
	}

	return sw, nil
}

// Start begins watching files
func (sw *StackWatcher) Start() error {
	sw.mu.Lock()
	if sw.running {
		sw.mu.Unlock()
		return nil
	}
	sw.running = true
	sw.mu.Unlock()

	// Add directories to watch
	err := filepath.Walk(sw.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}
		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == ".next" || name == "dist" || name == "build" ||
				name == "__pycache__" || name == ".cache" {
				return filepath.SkipDir
			}
			if err := sw.watcher.Add(path); err != nil {
				log.Printf("Failed to watch path %s: %v", path, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	go sw.eventLoop()
	return nil
}

// Stop stops watching files
func (sw *StackWatcher) Stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if !sw.running {
		return
	}

	close(sw.stopCh)
	sw.watcher.Close()
	sw.running = false

	// Cancel pending debounce timers
	sw.debounceMu.Lock()
	for _, timer := range sw.debounceMap {
		timer.Stop()
	}
	sw.debounceMap = make(map[string]*time.Timer)
	sw.debounceMu.Unlock()
}

// Subscribe adds a subscriber for watch events
func (sw *StackWatcher) Subscribe() chan WatchEvent {
	ch := make(chan WatchEvent, 100)
	sw.subMu.Lock()
	sw.subscribers[ch] = true
	sw.subMu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber
func (sw *StackWatcher) Unsubscribe(ch chan WatchEvent) {
	sw.subMu.Lock()
	delete(sw.subscribers, ch)
	close(ch)
	sw.subMu.Unlock()
}

// GetStatus returns the current watch status
func (sw *StackWatcher) GetStatus() WatchStatus {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	return WatchStatus{
		Enabled:      sw.config.Enabled,
		Watching:     sw.running,
		RepoPath:     sw.repoPath,
		WatchedPaths: len(sw.watcher.WatchList()),
		RecentEvents: append([]WatchEvent{}, sw.events...),
		Rules:        sw.config.Rules,
	}
}

func (sw *StackWatcher) eventLoop() {
	for {
		select {
		case <-sw.stopCh:
			return
		case event, ok := <-sw.watcher.Events:
			if !ok {
				return
			}
			sw.handleEvent(event)
		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error for stack %s: %v", sw.stackID, err)
		}
	}
}

func (sw *StackWatcher) handleEvent(event fsnotify.Event) {
	// Skip non-change events
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	// Get relative path
	relPath, err := filepath.Rel(sw.repoPath, event.Name)
	if err != nil {
		relPath = event.Name
	}

	// Find matching rule
	matchedRule := sw.findMatchingRule(relPath)
	if matchedRule != nil && matchedRule.Action == watchActionIgnore {
		return
	}

	// Debounce events
	sw.debounceMu.Lock()
	if timer, exists := sw.debounceMap[event.Name]; exists {
		timer.Stop()
	}

	debounce := sw.config.Debounce
	if debounce == 0 {
		debounce = 500 * time.Millisecond
	}

	sw.debounceMap[event.Name] = time.AfterFunc(debounce, func() {
		sw.processEvent(event, relPath, matchedRule)
	})
	sw.debounceMu.Unlock()
}

func (sw *StackWatcher) processEvent(event fsnotify.Event, relPath string, rule *WatchRule) {
	action := watchEventModified
	if event.Op&fsnotify.Create != 0 {
		action = watchEventCreated
	} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		action = watchEventDeleted
	}

	matchedRuleName := ""
	if rule != nil {
		matchedRuleName = rule.Pattern
	}

	normalizedPath := NormalizeRelPath(relPath)
	targetService := sw.matchService(normalizedPath)
	targetServiceName := ""
	if targetService != nil {
		targetServiceName = targetService.name
	}

	watchEvent := WatchEvent{
		Path:          normalizedPath,
		Action:        action,
		Timestamp:     time.Now(),
		MatchedRule:   matchedRuleName,
		TargetService: targetServiceName,
	}

	// Add to history
	sw.mu.Lock()
	sw.events = append(sw.events, watchEvent)
	if len(sw.events) > sw.maxEvents {
		sw.events = sw.events[1:]
	}
	sw.mu.Unlock()

	// Notify subscribers
	sw.subMu.RLock()
	for ch := range sw.subscribers {
		select {
		case ch <- watchEvent:
		default:
			// Channel full, skip
		}
	}
	sw.subMu.RUnlock()

	// Execute action if rule matched
	if rule != nil && rule.Action != "" && rule.Action != watchActionIgnore {
		sw.executeAction(rule.Action, targetService)
	}
}

func (sw *StackWatcher) findMatchingRule(path string) *WatchRule {
	for i := range sw.config.Rules {
		rule := &sw.config.Rules[i]
		if matchGlob(rule.Pattern, path) {
			return rule
		}
	}
	return nil
}

func (sw *StackWatcher) executeAction(action string, target *serviceMatcher) {
	log.Printf("Executing action '%s' for stack %s", action, sw.stackID)

	switch action {
	case watchActionRestart:
		if target == nil {
			log.Printf("Watch restart skipped: no matching service for stack %s", sw.stackID)
			return
		}
		if target.composeService == "" {
			log.Printf("Watch restart skipped: missing compose service for %s in stack %s", target.name, sw.stackID)
			return
		}
		composePath := sw.composePath
		if composePath == "" {
			log.Printf("Watch restart skipped: missing compose path for stack %s", sw.stackID)
			return
		}
		go func(serviceName, composeService string) {
			if err := ComposeRestart(composePath, composeService); err != nil {
				log.Printf("Failed to restart service %s in stack %s: %v", serviceName, sw.stackID, err)
				return
			}
			// Record restart event
			_ = RecordServiceRestart(sw.stackID, sw.stackID, serviceName, "file-change")
		}(target.name, target.composeService)
	case watchActionRestartAll:
		composePath := sw.composePath
		if composePath == "" {
			log.Printf("Watch restart-all skipped: missing compose path for stack %s", sw.stackID)
			return
		}
		go func() {
			if err := ComposeRestart(composePath, ""); err != nil {
				log.Printf("Failed to restart stack %s: %v", sw.stackID, err)
				return
			}
			// Record restart event for all services
			_ = RecordServiceRestart(sw.stackID, sw.stackID, "*", "file-change")
		}()
	}
}

// matchGlob performs simple glob matching
func matchGlob(pattern, path string) bool {
	// Convert glob pattern to work with filepath.Match
	// Handle ** for recursive matching
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			// Check if path starts with prefix
			if prefix != "" && !strings.HasPrefix(path, prefix) {
				return false
			}

			// Check if path ends with suffix pattern
			if suffix != "" {
				pathSuffix := path
				if prefix != "" {
					pathSuffix = strings.TrimPrefix(path, prefix)
					pathSuffix = strings.TrimPrefix(pathSuffix, "/")
				}
				matched, _ := filepath.Match(suffix, filepath.Base(pathSuffix))
				return matched
			}
			return true
		}
	}

	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Try matching against basename
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}

// GetStackWatcher returns an existing watcher for a stack
func GetStackWatcher(stackID string) *StackWatcher {
	watcherRegistryMu.RLock()
	defer watcherRegistryMu.RUnlock()
	return watcherRegistry[stackID]
}

// StartWatching starts file watching for a stack
func StartWatching(stackID, repoPath, composePath string, services []ServiceRecord, config WatchConfig) error {
	if !config.Enabled {
		return nil
	}

	watcherRegistryMu.Lock()
	defer watcherRegistryMu.Unlock()

	// Stop existing watcher if any
	if existing, ok := watcherRegistry[stackID]; ok {
		existing.Stop()
		delete(watcherRegistry, stackID)
	}

	watcher, err := NewStackWatcher(stackID, repoPath, composePath, services, config)
	if err != nil {
		return err
	}

	if err := watcher.Start(); err != nil {
		return err
	}

	watcherRegistry[stackID] = watcher
	return nil
}

type serviceMatcher struct {
	name           string
	composeService string
	workspacePath  string
}

func buildServiceMatchers(services []ServiceRecord) []serviceMatcher {
	matchers := make([]serviceMatcher, 0, len(services))
	for _, svc := range services {
		workspace := NormalizeRelPath(svc.WorkspacePath)
		if workspace == "" {
			continue
		}
		matchers = append(matchers, serviceMatcher{
			name:           svc.Name,
			composeService: svc.ComposeService,
			workspacePath:  workspace,
		})
	}

	sort.Slice(matchers, func(i, j int) bool {
		return len(matchers[i].workspacePath) > len(matchers[j].workspacePath)
	})

	return matchers
}

func (sw *StackWatcher) matchService(relPath string) *serviceMatcher {
	if relPath == "" {
		return nil
	}
	for i := range sw.serviceMatchers {
		matcher := &sw.serviceMatchers[i]
		if pathHasPrefix(relPath, matcher.workspacePath) {
			return matcher
		}
	}
	return nil
}

func pathHasPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

// StopWatching stops file watching for a stack
func StopWatching(stackID string) {
	watcherRegistryMu.Lock()
	defer watcherRegistryMu.Unlock()

	if watcher, ok := watcherRegistry[stackID]; ok {
		watcher.Stop()
		delete(watcherRegistry, stackID)
	}
}

// GetWatchStatus returns the watch status for a stack
func GetWatchStatus(stackID string) *WatchStatus {
	watcherRegistryMu.RLock()
	watcher := watcherRegistry[stackID]
	watcherRegistryMu.RUnlock()

	if watcher == nil {
		return nil
	}

	status := watcher.GetStatus()
	return &status
}

// StreamWatchEvents streams watch events via a channel
func StreamWatchEvents(stackID string) (<-chan WatchEvent, func()) {
	watcherRegistryMu.RLock()
	watcher := watcherRegistry[stackID]
	watcherRegistryMu.RUnlock()

	if watcher == nil {
		ch := make(chan WatchEvent)
		close(ch)
		return ch, func() {}
	}

	ch := watcher.Subscribe()
	cleanup := func() {
		watcher.Unsubscribe(ch)
	}
	return ch, cleanup
}

// WatchEventJSON returns watch event as JSON bytes
func WatchEventJSON(event WatchEvent) []byte {
	data, _ := json.Marshal(event)
	return data
}
