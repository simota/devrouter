package devrouter

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	// StatsSampleInterval is the interval between stats samples
	StatsSampleInterval = 1 * time.Minute
)

// StatsCollector collects and persists stats for all active stacks
type StatsCollector struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	running bool
}

var (
	globalCollector   *StatsCollector
	globalCollectorMu sync.Mutex
)

// NewStatsCollector creates a new stats collector
func NewStatsCollector() *StatsCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &StatsCollector{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins collecting stats in the background
func (sc *StatsCollector) Start() {
	sc.mu.Lock()
	if sc.running {
		sc.mu.Unlock()
		return
	}
	sc.running = true
	sc.mu.Unlock()

	sc.wg.Add(1)
	go sc.collectLoop()

	log.Printf("Stats collector started (interval: %v)", StatsSampleInterval)
}

// Stop stops the stats collector
func (sc *StatsCollector) Stop() {
	sc.mu.Lock()
	if !sc.running {
		sc.mu.Unlock()
		return
	}
	sc.running = false
	sc.mu.Unlock()

	sc.cancel()
	sc.wg.Wait()

	log.Printf("Stats collector stopped")
}

func (sc *StatsCollector) collectLoop() {
	defer sc.wg.Done()

	// Initial collection after a short delay
	select {
	case <-time.After(5 * time.Second):
		sc.collectAllStacks()
	case <-sc.ctx.Done():
		return
	}

	ticker := time.NewTicker(StatsSampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sc.collectAllStacks()
		case <-sc.ctx.Done():
			return
		}
	}
}

func (sc *StatsCollector) collectAllStacks() {
	reg, err := LoadRegistry()
	if err != nil {
		log.Printf("Stats collector: failed to load registry: %v", err)
		return
	}

	if len(reg.Stacks) == 0 {
		return
	}

	for _, stack := range reg.Stacks {
		if err := sc.collectStackStats(stack); err != nil {
			// Log but continue with other stacks
			log.Printf("Stats collector: failed to collect stats for %s: %v", stack.ID, err)
		}
	}

	// Prune old history periodically (every hour worth of samples)
	// This is a simple approach - could be optimized with a separate timer
	if time.Now().Minute() == 0 {
		if err := PruneOldHistory(); err != nil {
			log.Printf("Stats collector: failed to prune old history: %v", err)
		}
	}
}

func (sc *StatsCollector) collectStackStats(stack StackRecord) error {
	if stack.ComposeFilePath == "" {
		return nil
	}

	stats, err := GetStackStats(stack.ComposeFilePath)
	if err != nil {
		return err
	}

	if len(stats.Containers) == 0 {
		return nil
	}

	// Convert to StatsEntry format
	now := time.Now().UTC().Format(time.RFC3339)
	entries := make([]StatsEntry, 0, len(stats.Containers))
	for _, c := range stats.Containers {
		entries = append(entries, StatsEntry{
			Timestamp: now,
			Service:   c.Service,
			CPU:       c.CPUPercent,
			Mem:       c.MemoryUsage,
			MemPct:    c.MemPercent,
			NetIO:     c.NetIO,
			BlockIO:   c.BlockIO,
		})
	}

	return AppendStats(stack.ID, entries)
}

// StartGlobalStatsCollector starts the global stats collector
func StartGlobalStatsCollector() {
	globalCollectorMu.Lock()
	defer globalCollectorMu.Unlock()

	if globalCollector != nil {
		return
	}

	globalCollector = NewStatsCollector()
	globalCollector.Start()
}

// StopGlobalStatsCollector stops the global stats collector
func StopGlobalStatsCollector() {
	globalCollectorMu.Lock()
	defer globalCollectorMu.Unlock()

	if globalCollector == nil {
		return
	}

	globalCollector.Stop()
	globalCollector = nil
}
