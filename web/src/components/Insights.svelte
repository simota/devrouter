<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import type { StackInsights } from '../api/types';
  import { getStackInsights } from '../api/client';

  export let stackId: string;

  const dispatch = createEventDispatcher<{ close: void }>();

  let insights: StackInsights | null = null;
  let loading = true;
  let error: string | null = null;

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatUptime(seconds: number): string {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    if (seconds < 86400) {
      const h = Math.floor(seconds / 3600);
      const m = Math.floor((seconds % 3600) / 60);
      return `${h}h ${m}m`;
    }
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    return `${d}d ${h}h`;
  }

  function getRestartServices(): { name: string; count: number }[] {
    if (!insights?.restartsByService) return [];
    return Object.entries(insights.restartsByService)
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count);
  }

  async function loadInsights() {
    loading = true;
    error = null;
    try {
      insights = await getStackInsights(stackId);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load insights';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadInsights();
  });
</script>

<div class="insights">
  <div class="insights-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
        <polyline points="7.5 4.21 12 6.81 16.5 4.21"/>
        <polyline points="7.5 19.79 7.5 14.6 3 12"/>
        <polyline points="21 12 16.5 14.6 16.5 19.79"/>
        <polyline points="3.27 6.96 12 12.01 20.73 6.96"/>
        <line x1="12" y1="22.08" x2="12" y2="12"/>
      </svg>
      <h3>Stack Insights</h3>
    </div>
    <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close insights panel">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  <div class="insights-content">
    {#if loading}
      <div class="loading">
        <span class="spinner"></span>
        <span>Loading...</span>
      </div>
    {:else if error}
      <div class="error">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        <span>{error}</span>
        <button on:click={loadInsights}>Retry</button>
      </div>
    {:else if insights}
      <div class="metrics-grid">
        <div class="metric-card uptime">
          <div class="metric-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"/>
              <polyline points="12 6 12 12 16 14"/>
            </svg>
          </div>
          <div class="metric-info">
            <span class="metric-label">Uptime</span>
            <span class="metric-value">{formatUptime(insights.uptimeSeconds)}</span>
          </div>
        </div>

        <div class="metric-card restarts">
          <div class="metric-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M23 4v6h-6M1 20v-6h6"/>
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
            </svg>
          </div>
          <div class="metric-info">
            <span class="metric-label">Total Restarts</span>
            <span class="metric-value">{insights.restartCount}</span>
          </div>
        </div>

        <div class="metric-card memory">
          <div class="metric-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="4" y="4" width="16" height="16" rx="2" ry="2"/>
              <rect x="9" y="9" width="6" height="6"/>
              <line x1="9" y1="1" x2="9" y2="4"/>
              <line x1="15" y1="1" x2="15" y2="4"/>
              <line x1="9" y1="20" x2="9" y2="23"/>
              <line x1="15" y1="20" x2="15" y2="23"/>
              <line x1="20" y1="9" x2="23" y2="9"/>
              <line x1="20" y1="14" x2="23" y2="14"/>
              <line x1="1" y1="9" x2="4" y2="9"/>
              <line x1="1" y1="14" x2="4" y2="14"/>
            </svg>
          </div>
          <div class="metric-info">
            <span class="metric-label">Peak Memory</span>
            <span class="metric-value">{formatBytes(insights.peakMemory)}</span>
            {#if insights.peakMemorySvc}
              <span class="metric-sub">{insights.peakMemorySvc}</span>
            {/if}
          </div>
        </div>

        <div class="metric-card cpu">
          <div class="metric-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="4" y="4" width="16" height="16" rx="2" ry="2"/>
              <line x1="4" y1="12" x2="20" y2="12"/>
              <line x1="12" y1="4" x2="12" y2="20"/>
            </svg>
          </div>
          <div class="metric-info">
            <span class="metric-label">Avg CPU</span>
            <span class="metric-value">{insights.avgCpu.toFixed(1)}%</span>
          </div>
        </div>
      </div>

      {#if getRestartServices().length > 0}
        <div class="restarts-section">
          <h4>Restarts by Service</h4>
          <div class="restarts-list">
            {#each getRestartServices() as svc}
              <div class="restart-item">
                <span class="service-name">{svc.name}</span>
                <div class="restart-bar">
                  <div
                    class="bar-fill"
                    style="width: {Math.min(100, (svc.count / Math.max(...getRestartServices().map(s => s.count))) * 100)}%"
                  ></div>
                </div>
                <span class="restart-count">{svc.count}</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <div class="computed-at">
        Last computed: {new Date(insights.computedAt).toLocaleString('ja-JP')}
      </div>
    {:else}
      <div class="empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
        </svg>
        <span>No insights available</span>
      </div>
    {/if}
  </div>
</div>

<style>
  .insights {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    overflow: hidden;
  }

  .insights-header {
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
    color: var(--accent-orange);
  }

  .header-left h3 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
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

  .insights-content {
    padding: 16px;
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

  .error svg, .empty svg {
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

  .error button:hover {
    background: var(--bg-hover);
  }

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
    margin-bottom: 16px;
  }

  .metric-card {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px;
    background: var(--bg-surface);
    border-radius: 8px;
    border: 1px solid var(--border-subtle);
  }

  .metric-icon {
    width: 36px;
    height: 36px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .metric-icon svg {
    width: 18px;
    height: 18px;
  }

  .uptime .metric-icon {
    background: rgba(34, 197, 94, 0.1);
    color: var(--accent-green);
  }

  .restarts .metric-icon {
    background: rgba(249, 115, 22, 0.1);
    color: var(--accent-orange);
  }

  .memory .metric-icon {
    background: rgba(168, 85, 247, 0.1);
    color: var(--accent-purple);
  }

  .cpu .metric-icon {
    background: rgba(6, 182, 212, 0.1);
    color: var(--accent-cyan);
  }

  .metric-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .metric-label {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .metric-value {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--text-primary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .metric-sub {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .restarts-section {
    background: var(--bg-surface);
    border-radius: 8px;
    padding: 14px;
    margin-bottom: 12px;
  }

  .restarts-section h4 {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-secondary);
    margin: 0 0 12px 0;
  }

  .restarts-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .restart-item {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .restart-item .service-name {
    width: 100px;
    font-size: 0.75rem;
    font-family: 'IBM Plex Mono', monospace;
    color: var(--text-primary);
    flex-shrink: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .restart-bar {
    flex: 1;
    height: 6px;
    background: var(--bg-elevated);
    border-radius: 3px;
    overflow: hidden;
  }

  .bar-fill {
    height: 100%;
    background: var(--accent-orange);
    border-radius: 3px;
    transition: width 0.3s ease;
  }

  .restart-count {
    width: 30px;
    text-align: right;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-secondary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .computed-at {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    text-align: right;
  }
</style>
