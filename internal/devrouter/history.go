package devrouter

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// HistoryEventType defines the type of stack lifecycle event
type HistoryEventType string

const (
	EventTypeUp      HistoryEventType = "up"
	EventTypeDown    HistoryEventType = "down"
	EventTypeRestart HistoryEventType = "restart"
)

// HistoryEvent represents a stack lifecycle event
type HistoryEvent struct {
	Timestamp string           `json:"ts"`
	Type      HistoryEventType `json:"type"`
	Stack     string           `json:"stack"`
	StackID   string           `json:"stackId,omitempty"`
	Services  []string         `json:"services,omitempty"`
	Service   string           `json:"service,omitempty"`
	Trigger   string           `json:"trigger,omitempty"`
	Duration  string           `json:"duration,omitempty"`
}

// StatsEntry represents a sampled stats record
type StatsEntry struct {
	Timestamp string  `json:"ts"`
	Service   string  `json:"service"`
	CPU       float64 `json:"cpu"`
	Mem       int64   `json:"mem"`
	MemPct    float64 `json:"memPct"`
	NetIO     string  `json:"netIO,omitempty"`
	BlockIO   string  `json:"blockIO,omitempty"`
}

// StackInsights represents computed insights for a stack
type StackInsights struct {
	StackID       string         `json:"stackId"`
	Uptime        string         `json:"uptime"`
	UptimeSeconds int64          `json:"uptimeSeconds"`
	RestartCount  int            `json:"restartCount"`
	RestartsByService map[string]int `json:"restartsByService"`
	PeakMemory    int64          `json:"peakMemory"`
	PeakMemoryAt  string         `json:"peakMemoryAt,omitempty"`
	PeakMemorySvc string         `json:"peakMemorySvc,omitempty"`
	AvgCPU        float64        `json:"avgCpu"`
	ComputedAt    string         `json:"computedAt"`
}

// History retention policies
const (
	EventRetentionDays = 30
	StatsRetentionDays = 7
)

var historyMu sync.Mutex

// HistoryDir returns the path to the history directory
func HistoryDir() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "history"), nil
}

// EventsPath returns the path to the events log file
func EventsPath() (string, error) {
	dir, err := HistoryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "events.jsonl"), nil
}

// StatsDir returns the path to the stats directory for a stack
func StatsDir(stackID string) (string, error) {
	dir, err := HistoryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "stats", stackID), nil
}

// StatsPath returns the path to the stats file for a specific date
func StatsPath(stackID string, date time.Time) (string, error) {
	dir, err := StatsDir(stackID)
	if err != nil {
		return "", err
	}
	filename := date.UTC().Format("2006-01-02") + ".jsonl"
	return filepath.Join(dir, filename), nil
}

// AppendEvent appends a history event to the events log
func AppendEvent(event HistoryEvent) error {
	historyMu.Lock()
	defer historyMu.Unlock()

	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	path, err := EventsPath()
	if err != nil {
		return fmt.Errorf("get events path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write event: %w", err)
	}

	return nil
}

// LoadEvents loads history events, optionally filtered by stack and time range
func LoadEvents(stackID string, since time.Time) ([]HistoryEvent, error) {
	path, err := EventsPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []HistoryEvent{}, nil
		}
		return nil, fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	var events []HistoryEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event HistoryEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // skip malformed lines
		}

		// Filter by stack if specified
		if stackID != "" && event.StackID != stackID && event.Stack != stackID {
			continue
		}

		// Filter by time if specified
		if !since.IsZero() {
			eventTime, err := time.Parse(time.RFC3339, event.Timestamp)
			if err != nil {
				continue
			}
			if eventTime.Before(since) {
				continue
			}
		}

		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read events file: %w", err)
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp > events[j].Timestamp
	})

	return events, nil
}

// AppendStats appends stats entries to the daily stats file
func AppendStats(stackID string, entries []StatsEntry) error {
	if len(entries) == 0 {
		return nil
	}

	historyMu.Lock()
	defer historyMu.Unlock()

	now := time.Now().UTC()
	path, err := StatsPath(stackID, now)
	if err != nil {
		return fmt.Errorf("get stats path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create stats dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open stats file: %w", err)
	}
	defer f.Close()

	for _, entry := range entries {
		if entry.Timestamp == "" {
			entry.Timestamp = now.Format(time.RFC3339)
		}
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write stats: %w", err)
		}
	}

	return nil
}

// LoadStats loads stats entries for a stack within a time range
func LoadStats(stackID string, since time.Time, until time.Time) ([]StatsEntry, error) {
	if _, err := StatsDir(stackID); err != nil {
		return nil, err
	}

	if until.IsZero() {
		until = time.Now().UTC()
	}

	var allEntries []StatsEntry

	// Iterate through date range
	for d := since; !d.After(until); d = d.AddDate(0, 0, 1) {
		path, err := StatsPath(stackID, d)
		if err != nil {
			continue
		}

		entries, err := loadStatsFile(path, since, until)
		if err != nil {
			continue // skip files that don't exist or are unreadable
		}
		allEntries = append(allEntries, entries...)
	}

	// Also check today's file if not already included
	todayPath, _ := StatsPath(stackID, time.Now().UTC())
	if _, err := os.Stat(todayPath); err == nil {
		// Check if we already loaded it
		alreadyLoaded := false
		for d := since; !d.After(until); d = d.AddDate(0, 0, 1) {
			p, _ := StatsPath(stackID, d)
			if p == todayPath {
				alreadyLoaded = true
				break
			}
		}
		if !alreadyLoaded {
			entries, err := loadStatsFile(todayPath, since, until)
			if err == nil {
				allEntries = append(allEntries, entries...)
			}
		}
	}

	// Sort by timestamp ascending
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp < allEntries[j].Timestamp
	})

	// Remove duplicates (same timestamp + service)
	seen := make(map[string]bool)
	unique := make([]StatsEntry, 0, len(allEntries))
	for _, e := range allEntries {
		key := e.Timestamp + "|" + e.Service
		if !seen[key] {
			seen[key] = true
			unique = append(unique, e)
		}
	}

	return unique, nil
}

func loadStatsFile(path string, since, until time.Time) ([]StatsEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []StatsEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry StatsEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Filter by time range
		entryTime, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			continue
		}
		if !since.IsZero() && entryTime.Before(since) {
			continue
		}
		if !until.IsZero() && entryTime.After(until) {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// ComputeInsights computes insights for a stack based on history data
func ComputeInsights(stackID string) (*StackInsights, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Load today's events
	events, err := LoadEvents(stackID, today)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}

	// Load today's stats
	stats, err := LoadStats(stackID, today, now)
	if err != nil {
		return nil, fmt.Errorf("load stats: %w", err)
	}

	insights := &StackInsights{
		StackID:           stackID,
		RestartsByService: make(map[string]int),
		ComputedAt:        now.Format(time.RFC3339),
	}

	// Calculate restart count
	for _, e := range events {
		if e.Type == EventTypeRestart {
			insights.RestartCount++
			if e.Service != "" {
				insights.RestartsByService[e.Service]++
			}
		}
	}

	// Calculate uptime from last "up" event
	for _, e := range events {
		if e.Type == EventTypeUp {
			upTime, err := time.Parse(time.RFC3339, e.Timestamp)
			if err == nil {
				uptime := now.Sub(upTime)
				insights.UptimeSeconds = int64(uptime.Seconds())
				insights.Uptime = formatDuration(uptime)
			}
			break // events are sorted newest first
		}
	}

	// Calculate peak memory and average CPU from stats
	if len(stats) > 0 {
		var totalCPU float64
		var cpuCount int

		for _, s := range stats {
			if s.Mem > insights.PeakMemory {
				insights.PeakMemory = s.Mem
				insights.PeakMemoryAt = s.Timestamp
				insights.PeakMemorySvc = s.Service
			}
			totalCPU += s.CPU
			cpuCount++
		}

		if cpuCount > 0 {
			insights.AvgCPU = totalCPU / float64(cpuCount)
		}
	}

	return insights, nil
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// PruneOldHistory removes history data older than retention policies
func PruneOldHistory() error {
	historyMu.Lock()
	defer historyMu.Unlock()

	// Prune events
	if err := pruneEvents(); err != nil {
		return fmt.Errorf("prune events: %w", err)
	}

	// Prune stats
	if err := pruneStats(); err != nil {
		return fmt.Errorf("prune stats: %w", err)
	}

	return nil
}

func pruneEvents() error {
	path, err := EventsPath()
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	cutoff := time.Now().UTC().AddDate(0, 0, -EventRetentionDays)
	var kept [][]byte

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event HistoryEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		eventTime, err := time.Parse(time.RFC3339, event.Timestamp)
		if err != nil {
			continue
		}

		if !eventTime.Before(cutoff) {
			kept = append(kept, append([]byte{}, line...))
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Rewrite file with kept events
	tmp, err := os.CreateTemp(filepath.Dir(path), ".events-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	for _, line := range kept {
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			tmp.Close()
			return err
		}
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), path)
}

func pruneStats() error {
	dir, err := HistoryDir()
	if err != nil {
		return err
	}

	statsDir := filepath.Join(dir, "stats")
	if _, err := os.Stat(statsDir); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -StatsRetentionDays)
	cutoffDate := cutoff.Format("2006-01-02")

	// Walk through all stack directories
	entries, err := os.ReadDir(statsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stackDir := filepath.Join(statsDir, entry.Name())
		files, err := os.ReadDir(stackDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			name := file.Name()
			if !strings.HasSuffix(name, ".jsonl") {
				continue
			}

			// Extract date from filename (2006-01-02.jsonl)
			dateStr := strings.TrimSuffix(name, ".jsonl")
			if dateStr < cutoffDate {
				os.Remove(filepath.Join(stackDir, name))
			}
		}

		// Remove empty stack directories
		remaining, _ := os.ReadDir(stackDir)
		if len(remaining) == 0 {
			os.Remove(stackDir)
		}
	}

	return nil
}

// RecordStackUp records a stack up event
func RecordStackUp(stackID, stackName string, services []string) error {
	return AppendEvent(HistoryEvent{
		Type:     EventTypeUp,
		Stack:    stackName,
		StackID:  stackID,
		Services: services,
	})
}

// RecordStackDown records a stack down event with duration
func RecordStackDown(stackID, stackName string, upSince time.Time) error {
	var duration string
	if !upSince.IsZero() {
		duration = formatDuration(time.Since(upSince))
	}
	return AppendEvent(HistoryEvent{
		Type:     EventTypeDown,
		Stack:    stackName,
		StackID:  stackID,
		Duration: duration,
	})
}

// RecordServiceRestart records a service restart event
func RecordServiceRestart(stackID, stackName, service, trigger string) error {
	return AppendEvent(HistoryEvent{
		Type:    EventTypeRestart,
		Stack:   stackName,
		StackID: stackID,
		Service: service,
		Trigger: trigger,
	})
}
