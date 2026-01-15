<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import type { WatchEvent, WatchStatus } from '../api/types';

  export let stackId: string;

  const dispatch = createEventDispatcher<{ close: void }>();

  let status: WatchStatus | null = null;
  let events: WatchEvent[] = [];
  let loading = true;
  let error: string | null = null;
  let ws: WebSocket | null = null;
  let starting = false;
  let stopping = false;
  const emptyRuleLabel = 'no rule';
  const unmappedLabel = 'no service';

  const API_BASE = '/api';

  async function fetchStatus() {
    try {
      const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/watch`);
      if (!res.ok) throw new Error('Failed to fetch watch status');
      status = await res.json();
      if (status?.recentEvents) {
        events = status.recentEvents;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load status';
    } finally {
      loading = false;
    }
  }

  async function startWatching() {
    if (starting) return;
    starting = true;
    error = null;
    try {
      const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/watch/start`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error('Failed to start watching');
      await fetchStatus();
      connectWebSocket();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to start';
    } finally {
      starting = false;
    }
  }

  async function stopWatching() {
    if (stopping) return;
    stopping = true;
    error = null;
    try {
      const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/watch/stop`, {
        method: 'POST',
      });
      if (!res.ok) throw new Error('Failed to stop watching');
      disconnectWebSocket();
      await fetchStatus();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to stop';
    } finally {
      stopping = false;
    }
  }

  function connectWebSocket() {
    if (ws) return;
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}${API_BASE}/ws/watch/${encodeURIComponent(stackId)}`);

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'event' && data.event) {
          events = [data.event, ...events].slice(0, 50);
        } else if (data.type === 'status' && data.status) {
          status = data.status;
        }
      } catch (e) {
        // Ignore parse errors
      }
    };

    ws.onerror = () => {
      ws = null;
    };

    ws.onclose = () => {
      ws = null;
    };
  }

  function disconnectWebSocket() {
    if (ws) {
      ws.close();
      ws = null;
    }
  }

  function formatTime(timestamp: string): string {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function getActionIcon(action: string): string {
    switch (action) {
      case 'created': return 'M12 4v16m-8-8h16';
      case 'deleted': return 'M18 6L6 18M6 6l12 12';
      case 'modified': return 'M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z';
      default: return 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z';
    }
  }

  function getActionColor(action: string): string {
    switch (action) {
      case 'created': return '#22c55e';
      case 'deleted': return '#ef4444';
      case 'modified': return '#f59e0b';
      default: return '#6b7280';
    }
  }

  onMount(() => {
    fetchStatus().then(() => {
      if (status?.watching) {
        connectWebSocket();
      }
    });
  });

  onDestroy(() => {
    disconnectWebSocket();
  });
</script>

<div class="watch-status">
  <div class="watch-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
        <path d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/>
      </svg>
      <h3>File Watcher</h3>
      {#if status?.watching}
        <span class="status-badge active">Watching</span>
      {:else}
        <span class="status-badge inactive">Inactive</span>
      {/if}
    </div>
    <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close watch status">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  {#if loading}
    <div class="loading">
      <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
      </svg>
      <span>Loading status...</span>
    </div>
  {:else if error}
    <div class="error">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="8" x2="12" y2="12"/>
        <line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
      <span>{error}</span>
    </div>
  {:else}
    <div class="watch-content">
      <div class="watch-controls">
        {#if status?.watching}
          <button class="control-btn stop" on:click={stopWatching} disabled={stopping}>
            {#if stopping}
              <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
              </svg>
            {:else}
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="6" y="6" width="12" height="12" rx="1"/>
              </svg>
            {/if}
            Stop Watching
          </button>
          <span class="paths-info">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
            </svg>
            {status.watchedPaths} paths
          </span>
        {:else}
          <button class="control-btn start" on:click={startWatching} disabled={starting}>
            {#if starting}
              <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
              </svg>
            {:else}
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polygon points="5 3 19 12 5 21 5 3"/>
              </svg>
            {/if}
            Start Watching
          </button>
          <p class="watch-hint">
            Watch for file changes and auto-restart services
          </p>
        {/if}
      </div>

      {#if status?.rules && status.rules.length > 0}
        <div class="rules-section">
          <h4>Watch Rules</h4>
          <div class="rules-list">
            {#each status.rules as rule}
              <div class="rule-item">
                <code class="pattern">{rule.pattern}</code>
                <span class="arrow">→</span>
                <span class="action">{rule.action}</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <div class="events-section">
        <h4>Recent Changes</h4>
        {#if events.length === 0}
          <div class="no-events">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/>
            </svg>
            <span>No file changes detected yet</span>
          </div>
        {:else}
          <div class="events-list">
            {#each events as event (event.timestamp + event.path)}
              <div class="event-item">
                <svg viewBox="0 0 24 24" fill="none" stroke={getActionColor(event.action)} stroke-width="2">
                  <path d={getActionIcon(event.action)}/>
                </svg>
                <span class="event-path">{event.path}</span>
                <span class="event-action" style="color: {getActionColor(event.action)}">{event.action}</span>
                <span class="event-target" class:unmapped={!event.targetService}>
                  {event.targetService || unmappedLabel}
                </span>
                <span class="event-rule">{event.matchedRule || emptyRuleLabel}</span>
                <span class="event-time">{formatTime(event.timestamp)}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .watch-status {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    overflow: visible;
  }

  .watch-header {
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

  .status-badge {
    font-size: 0.625rem;
    font-weight: 600;
    padding: 3px 8px;
    border-radius: 4px;
  }

  .status-badge.active {
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
  }

  .status-badge.inactive {
    background: rgba(107, 114, 128, 0.15);
    color: var(--text-tertiary);
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

  .loading, .error, .no-events {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 40px 20px;
    color: var(--text-tertiary);
    font-size: 0.875rem;
  }

  .loading svg, .error svg, .no-events svg {
    width: 32px;
    height: 32px;
    opacity: 0.5;
  }

  .spinner {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .watch-content {
    padding: 16px 18px;
  }

  .watch-controls {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 20px;
  }

  .control-btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 10px 18px;
    font-size: 0.813rem;
    font-weight: 500;
    font-family: inherit;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .control-btn svg {
    width: 16px;
    height: 16px;
  }

  .control-btn.start {
    background: rgba(34, 197, 94, 0.1);
    border: 1px solid rgba(34, 197, 94, 0.2);
    color: #22c55e;
  }

  .control-btn.start:hover:not(:disabled) {
    background: rgba(34, 197, 94, 0.2);
  }

  .control-btn.stop {
    background: rgba(248, 81, 73, 0.1);
    border: 1px solid rgba(248, 81, 73, 0.2);
    color: var(--accent-red);
  }

  .control-btn.stop:hover:not(:disabled) {
    background: rgba(248, 81, 73, 0.2);
  }

  .control-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .watch-hint {
    font-size: 0.75rem;
    color: var(--text-tertiary);
    margin: 0;
  }

  .paths-info {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.75rem;
    color: var(--text-secondary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .paths-info svg {
    width: 14px;
    height: 14px;
    color: var(--text-tertiary);
  }

  .rules-section, .events-section {
    margin-top: 16px;
  }

  .rules-section h4, .events-section h4 {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-secondary);
    margin: 0 0 10px 0;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .rules-list {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .rule-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    font-size: 0.75rem;
  }

  .rule-item .pattern {
    font-family: 'IBM Plex Mono', monospace;
    color: var(--accent-cyan);
  }

  .rule-item .arrow {
    color: var(--text-tertiary);
  }

  .rule-item .action {
    color: var(--text-secondary);
    font-weight: 500;
  }

  .events-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 200px;
    overflow-y: auto;
  }

  .event-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: var(--bg-surface);
    border-radius: 6px;
    font-size: 0.75rem;
  }

  .event-item svg {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
  }

  .event-path {
    flex: 1;
    font-family: 'IBM Plex Mono', monospace;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .event-action {
    font-weight: 500;
    text-transform: capitalize;
  }

  .event-target {
    font-weight: 600;
    color: var(--accent-green);
    text-transform: lowercase;
  }

  .event-target.unmapped {
    color: var(--accent-orange);
  }

  .event-rule {
    font-family: 'IBM Plex Mono', monospace;
    color: var(--text-tertiary);
    text-transform: lowercase;
  }

  .event-time {
    color: var(--text-tertiary);
    font-family: 'IBM Plex Mono', monospace;
  }
</style>
