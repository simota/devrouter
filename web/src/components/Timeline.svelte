<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import type { HistoryEvent } from '../api/types';
  import { getHistoryEvents } from '../api/client';

  export let stackId: string;

  const dispatch = createEventDispatcher<{ close: void }>();

  let events: HistoryEvent[] = [];
  let loading = true;
  let error: string | null = null;

  function formatTime(ts: string): string {
    const date = new Date(ts);
    return date.toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit' });
  }

  function formatDate(ts: string): string {
    const date = new Date(ts);
    return date.toLocaleDateString('ja-JP', { month: 'short', day: 'numeric' });
  }

  function getEventIcon(type: string): string {
    switch (type) {
      case 'up': return 'play';
      case 'down': return 'stop';
      case 'restart': return 'refresh';
      default: return 'circle';
    }
  }

  function getEventColor(type: string): string {
    switch (type) {
      case 'up': return 'var(--accent-green)';
      case 'down': return 'var(--accent-red)';
      case 'restart': return 'var(--accent-orange)';
      default: return 'var(--text-tertiary)';
    }
  }

  function getEventLabel(event: HistoryEvent): string {
    switch (event.type) {
      case 'up':
        const services = event.services?.join(', ') || '-';
        return `Started (${services})`;
      case 'down':
        return `Stopped${event.duration ? ` (${event.duration})` : ''}`;
      case 'restart':
        return `Restarted ${event.service || 'all'}${event.trigger ? ` (${event.trigger})` : ''}`;
      default:
        return event.type;
    }
  }

  async function loadEvents() {
    loading = true;
    error = null;
    try {
      const response = await getHistoryEvents(stackId);
      events = response.events || [];
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load events';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadEvents();
  });
</script>

<div class="timeline">
  <div class="timeline-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10"/>
        <polyline points="12 6 12 12 16 14"/>
      </svg>
      <h3>Activity Timeline</h3>
    </div>
    <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close timeline">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  <div class="timeline-content">
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
        <button on:click={loadEvents}>Retry</button>
      </div>
    {:else if events.length === 0}
      <div class="empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="3" y="4" width="18" height="18" rx="2" ry="2"/>
          <line x1="16" y1="2" x2="16" y2="6"/>
          <line x1="8" y1="2" x2="8" y2="6"/>
          <line x1="3" y1="10" x2="21" y2="10"/>
        </svg>
        <span>No events in the last 24 hours</span>
      </div>
    {:else}
      <div class="events-list">
        {#each events as event, i}
          <div class="event-item">
            <div class="event-time">
              <span class="time">{formatTime(event.ts)}</span>
              <span class="date">{formatDate(event.ts)}</span>
            </div>
            <div class="event-line">
              <div class="event-dot" style="background: {getEventColor(event.type)}">
                {#if event.type === 'up'}
                  <svg viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                {:else if event.type === 'down'}
                  <svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="12" height="16"/></svg>
                {:else}
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                    <path d="M23 4v6h-6M1 20v-6h6"/>
                    <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                  </svg>
                {/if}
              </div>
              {#if i < events.length - 1}
                <div class="connector"></div>
              {/if}
            </div>
            <div class="event-details">
              <span class="event-type" style="color: {getEventColor(event.type)}">{event.type.toUpperCase()}</span>
              <span class="event-label">{getEventLabel(event)}</span>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .timeline {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    overflow: hidden;
  }

  .timeline-header {
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
    color: var(--accent-purple);
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

  .timeline-content {
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

  .events-list {
    display: flex;
    flex-direction: column;
  }

  .event-item {
    display: flex;
    gap: 12px;
    min-height: 48px;
  }

  .event-time {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    width: 60px;
    flex-shrink: 0;
    padding-top: 2px;
  }

  .event-time .time {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-primary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .event-time .date {
    font-size: 0.625rem;
    color: var(--text-tertiary);
  }

  .event-line {
    display: flex;
    flex-direction: column;
    align-items: center;
    width: 24px;
    flex-shrink: 0;
  }

  .event-dot {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .event-dot svg {
    width: 10px;
    height: 10px;
    color: white;
  }

  .connector {
    width: 2px;
    flex: 1;
    min-height: 20px;
    background: var(--border-subtle);
    margin: 4px 0;
  }

  .event-details {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-top: 2px;
  }

  .event-type {
    font-size: 0.625rem;
    font-weight: 700;
    letter-spacing: 0.05em;
  }

  .event-label {
    font-size: 0.813rem;
    color: var(--text-secondary);
  }
</style>
