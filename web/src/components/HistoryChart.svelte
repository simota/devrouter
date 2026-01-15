<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import type { StatsEntry } from '../api/types';
  import { getHistoryStats } from '../api/client';

  export let stackId: string;

  const dispatch = createEventDispatcher<{ close: void }>();

  let stats: StatsEntry[] = [];
  let loading = true;
  let error: string | null = null;
  let selectedRange: '1h' | '6h' | '24h' = '1h';
  let selectedMetric: 'cpu' | 'mem' = 'cpu';
  const rangeOptions: ('1h' | '6h' | '24h')[] = ['1h', '6h', '24h'];

  // Group stats by service
  $: serviceStats = groupByService(stats);
  $: services = Object.keys(serviceStats);

  function groupByService(entries: StatsEntry[]): Record<string, StatsEntry[]> {
    const grouped: Record<string, StatsEntry[]> = {};
    for (const entry of entries) {
      if (!grouped[entry.service]) {
        grouped[entry.service] = [];
      }
      grouped[entry.service].push(entry);
    }
    return grouped;
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function getChartPath(entries: StatsEntry[], metric: 'cpu' | 'mem', width: number, height: number): string {
    if (entries.length < 2) return '';

    const values = metric === 'cpu'
      ? entries.map(e => e.cpu)
      : entries.map(e => e.memPct);

    const max = Math.max(...values, 1);
    const step = width / (entries.length - 1);

    const points = values.map((v, i) => {
      const x = i * step;
      const y = height - (v / max) * height * 0.9;
      return `${x},${y}`;
    });

    return `M${points.join(' L')}`;
  }

  function getAreaPath(entries: StatsEntry[], metric: 'cpu' | 'mem', width: number, height: number): string {
    if (entries.length < 2) return '';

    const linePath = getChartPath(entries, metric, width, height);
    if (!linePath) return '';

    return `${linePath} L${width},${height} L0,${height} Z`;
  }

  function getMetricColor(metric: 'cpu' | 'mem'): string {
    return metric === 'cpu' ? 'var(--accent-cyan)' : 'var(--accent-green)';
  }

  async function loadStats() {
    loading = true;
    error = null;
    try {
      const response = await getHistoryStats(stackId, selectedRange);
      stats = response.stats || [];
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load stats';
    } finally {
      loading = false;
    }
  }

  function selectRange(range: '1h' | '6h' | '24h') {
    selectedRange = range;
    loadStats();
  }

  onMount(() => {
    loadStats();
  });
</script>

<div class="history-chart">
  <div class="chart-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M3 3v18h18"/>
        <path d="M18 9l-5 5-4-4-3 3"/>
      </svg>
      <h3>Resource History</h3>
    </div>
    <div class="header-controls">
      <div class="range-selector">
        {#each rangeOptions as range (range)}
          <button
            class:active={selectedRange === range}
            on:click={() => selectRange(range)}
          >
            {range}
          </button>
        {/each}
      </div>
      <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close history chart">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 6L6 18M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>

  <div class="metric-tabs">
    <button class:active={selectedMetric === 'cpu'} on:click={() => selectedMetric = 'cpu'}>
      CPU
    </button>
    <button class:active={selectedMetric === 'mem'} on:click={() => selectedMetric = 'mem'}>
      Memory
    </button>
  </div>

  <div class="chart-content">
    {#if loading}
      <div class="loading">
        <span class="spinner"></span>
        <span>Loading...</span>
      </div>
    {:else if error}
      <div class="error">
        <span>{error}</span>
        <button on:click={loadStats}>Retry</button>
      </div>
    {:else if stats.length === 0}
      <div class="empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M3 3v18h18"/>
          <path d="M18 9l-5 5-4-4-3 3"/>
        </svg>
        <span>No stats in the last {selectedRange}</span>
      </div>
    {:else}
      <div class="charts-grid">
        {#each services as service}
          {@const entries = serviceStats[service]}
          {@const lastEntry = entries[entries.length - 1]}
          <div class="service-chart">
            <div class="service-header">
              <span class="service-name">{service}</span>
              <span class="current-value" style="color: {getMetricColor(selectedMetric)}">
                {selectedMetric === 'cpu'
                  ? `${lastEntry?.cpu.toFixed(1)}%`
                  : formatBytes(lastEntry?.mem || 0)
                }
              </span>
            </div>
            <div class="chart-area">
              <svg viewBox="0 0 200 60" preserveAspectRatio="none">
                <defs>
                  <linearGradient id="gradient-{service}-{selectedMetric}" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" style="stop-color: {getMetricColor(selectedMetric)}; stop-opacity: 0.3"/>
                    <stop offset="100%" style="stop-color: {getMetricColor(selectedMetric)}; stop-opacity: 0.05"/>
                  </linearGradient>
                </defs>
                <path
                  d={getAreaPath(entries, selectedMetric, 200, 60)}
                  fill="url(#gradient-{service}-{selectedMetric})"
                />
                <path
                  d={getChartPath(entries, selectedMetric, 200, 60)}
                  fill="none"
                  stroke={getMetricColor(selectedMetric)}
                  stroke-width="2"
                />
              </svg>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .history-chart {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    overflow: hidden;
  }

  .chart-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-surface);
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .header-left svg {
    width: 18px;
    height: 18px;
    color: var(--accent-cyan);
  }

  .header-left h3 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .header-controls {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .range-selector {
    display: flex;
    background: var(--bg-elevated);
    border-radius: 6px;
    padding: 2px;
  }

  .range-selector button {
    padding: 4px 10px;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--text-tertiary);
    font-size: 0.688rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .range-selector button:hover {
    color: var(--text-secondary);
  }

  .range-selector button.active {
    background: var(--accent-cyan);
    color: white;
  }

  .close-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 6px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .close-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .close-btn svg {
    width: 16px;
    height: 16px;
  }

  .metric-tabs {
    display: flex;
    padding: 8px 18px;
    gap: 8px;
    border-bottom: 1px solid var(--border-subtle);
  }

  .metric-tabs button {
    padding: 6px 14px;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    color: var(--text-secondary);
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .metric-tabs button:hover {
    border-color: var(--text-tertiary);
  }

  .metric-tabs button.active {
    background: var(--bg-surface);
    border-color: var(--accent-cyan);
    color: var(--accent-cyan);
  }

  .chart-content {
    padding: 16px;
    max-height: 400px;
    overflow-y: auto;
  }

  .loading, .error, .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 40px 20px;
    color: var(--text-tertiary);
    font-size: 0.875rem;
  }

  .empty svg {
    width: 32px;
    height: 32px;
    opacity: 0.5;
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--border-subtle);
    border-top-color: var(--accent-cyan);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .error button {
    padding: 6px 12px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 0.75rem;
  }

  .charts-grid {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .service-chart {
    background: var(--bg-surface);
    border-radius: 8px;
    padding: 12px;
  }

  .service-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .service-name {
    font-size: 0.813rem;
    font-weight: 600;
    color: var(--text-primary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .current-value {
    font-size: 0.75rem;
    font-weight: 600;
    font-family: 'IBM Plex Mono', monospace;
  }

  .chart-area {
    height: 60px;
    background: var(--bg-elevated);
    border-radius: 6px;
    overflow: hidden;
  }

  .chart-area svg {
    width: 100%;
    height: 100%;
  }
</style>
