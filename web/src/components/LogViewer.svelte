<script lang="ts">
  import { createEventDispatcher, afterUpdate, onMount, onDestroy } from 'svelte';
  import type { LogMessage } from '../api/types';
  import { notifications } from '../lib/notifications';

  export let logs: LogMessage[] = [];
  export let expanded = false;
  export let connected = false;
  export let stackName = '';

  const dispatch = createEventDispatcher<{ toggle: void; clear: void }>();

  let logContainer: HTMLElement;
  let logViewer: HTMLElement;
  let autoScroll = true;
  let searchQuery = '';
  let selectedService = '';
  let showFilters = false;
  let selectedLevel = '';
  let lastNotifiedError = '';

  // Log level patterns
  const ERROR_PATTERNS = [
    /\b(error|err|exception|fatal|critical|fail|failed|failure)\b/i,
    /\[ERROR\]/i,
    /\[ERR\]/i,
    /Error:/i,
    /Exception:/i,
    /ECONNREFUSED/i,
    /ENOENT/i,
    /ETIMEDOUT/i,
  ];

  const WARN_PATTERNS = [
    /\b(warn|warning)\b/i,
    /\[WARN\]/i,
    /\[WARNING\]/i,
    /deprecated/i,
  ];

  const INFO_PATTERNS = [
    /\[INFO\]/i,
    /\binfo:/i,
  ];

  const DEBUG_PATTERNS = [
    /\[DEBUG\]/i,
    /\[TRACE\]/i,
    /\bdebug:/i,
  ];

  function detectLogLevel(content: string): 'error' | 'warn' | 'info' | 'debug' | 'log' {
    for (const pattern of ERROR_PATTERNS) {
      if (pattern.test(content)) return 'error';
    }
    for (const pattern of WARN_PATTERNS) {
      if (pattern.test(content)) return 'warn';
    }
    for (const pattern of INFO_PATTERNS) {
      if (pattern.test(content)) return 'info';
    }
    for (const pattern of DEBUG_PATTERNS) {
      if (pattern.test(content)) return 'debug';
    }
    return 'log';
  }

  // Detect stack traces (lines starting with 'at ' or containing file:line format)
  function isStackTraceLine(content: string): boolean {
    return /^\s*at\s+/.test(content) || /^\s*\d+\s*\|/.test(content) || /^\s+/.test(content) && /:\d+:\d+/.test(content);
  }

  // Resizable height
  const MIN_HEIGHT = 150;
  const MAX_HEIGHT = 600;
  const DEFAULT_HEIGHT = 300;
  let expandedHeight = DEFAULT_HEIGHT;
  let isResizing = false;
  let startY = 0;
  let startHeight = 0;

  function handleResizeStart(e: MouseEvent) {
    if (!expanded) return;
    isResizing = true;
    startY = e.clientY;
    startHeight = expandedHeight;
    document.body.style.cursor = 'ns-resize';
    document.body.style.userSelect = 'none';
  }

  function handleResizeMove(e: MouseEvent) {
    if (!isResizing) return;
    const delta = startY - e.clientY;
    const newHeight = Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, startHeight + delta));
    expandedHeight = newHeight;
  }

  function handleResizeEnd() {
    if (!isResizing) return;
    isResizing = false;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }

  onMount(() => {
    window.addEventListener('mousemove', handleResizeMove);
    window.addEventListener('mouseup', handleResizeEnd);
  });

  onDestroy(() => {
    window.removeEventListener('mousemove', handleResizeMove);
    window.removeEventListener('mouseup', handleResizeEnd);
  });

  // Get unique services from logs
  $: availableServices = [...new Set(logs.filter(l => l.service).map(l => l.service!))].sort();

  // Enrich logs with detected levels
  $: enrichedLogs = logs.map(log => ({
    ...log,
    detectedLevel: log.type === 'error' ? 'error' as const :
                   log.type === 'warning' ? 'warn' as const :
                   detectLogLevel(log.content),
    isStackTrace: isStackTraceLine(log.content),
  }));

  // Log statistics
  $: logStats = {
    total: enrichedLogs.length,
    errors: enrichedLogs.filter(l => l.detectedLevel === 'error').length,
    warnings: enrichedLogs.filter(l => l.detectedLevel === 'warn').length,
    info: enrichedLogs.filter(l => l.detectedLevel === 'info').length,
  };

  // Filter logs
  $: filteredLogs = enrichedLogs.filter(log => {
    // Service filter
    if (selectedService && log.service !== selectedService) {
      return false;
    }
    // Level filter
    if (selectedLevel && log.detectedLevel !== selectedLevel) {
      return false;
    }
    // Search query filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      const matchesContent = log.content.toLowerCase().includes(query);
      const matchesService = log.service?.toLowerCase().includes(query);
      if (!matchesContent && !matchesService) {
        return false;
      }
    }
    return true;
  });

  // Check for new errors and send notifications
  $: {
    const latestError = enrichedLogs.filter(l => l.detectedLevel === 'error').slice(-1)[0];
    if (latestError && latestError.content !== lastNotifiedError) {
      lastNotifiedError = latestError.content;
      const service = latestError.service || 'Unknown';
      notifications.error(
        'Error in Logs',
        `${service}: ${latestError.content.slice(0, 100)}`,
        `log-error:${stackName}:${Date.now()}`
      );
    }
  }

  afterUpdate(() => {
    if (autoScroll && logContainer) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  });

  function handleScroll() {
    if (!logContainer) return;
    const { scrollTop, scrollHeight, clientHeight } = logContainer;
    autoScroll = scrollHeight - scrollTop - clientHeight < 50;
  }

  function formatTime(isoString: string): string {
    const date = new Date(isoString);
    return date.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function getLogTypeClass(log: { type: string; detectedLevel: string; isStackTrace: boolean }): string {
    if (log.type === 'connected' || log.type === 'disconnected') return 'system';
    if (log.isStackTrace) return 'stacktrace';
    switch (log.detectedLevel) {
      case 'error': return 'error';
      case 'warn': return 'warning';
      case 'info': return 'info';
      case 'debug': return 'debug';
      default: return '';
    }
  }

  function clearFilters() {
    searchQuery = '';
    selectedService = '';
    selectedLevel = '';
  }

  function toggleFilters() {
    showFilters = !showFilters;
  }

  function logsToCSV(logs: LogMessage[]): string {
    const header = 'timestamp,type,service,content\n';
    const rows = logs.map(l =>
      `"${l.timestamp}","${l.type}","${l.service || ''}","${l.content.replace(/"/g, '""')}"`
    ).join('\n');
    return header + rows;
  }

  function exportLogs(format: 'json' | 'csv') {
    const data = format === 'json'
      ? JSON.stringify(filteredLogs, null, 2)
      : logsToCSV(filteredLogs);

    const mimeType = format === 'json' ? 'application/json' : 'text/csv';
    const blob = new Blob([data], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `devrouter-logs-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.${format}`;
    a.click();
    URL.revokeObjectURL(url);
  }
</script>

<div
  class="log-viewer"
  class:expanded
  class:resizing={isResizing}
  bind:this={logViewer}
  style={expanded ? `height: ${expandedHeight}px` : ''}
>
  {#if expanded}
    <!-- svelte-ignore a11y-no-noninteractive-tabindex -->
    <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
    <div
      class="resize-handle"
      on:mousedown={handleResizeStart}
      role="separator"
      aria-orientation="horizontal"
      aria-valuenow={expandedHeight}
      aria-valuemin={MIN_HEIGHT}
      aria-valuemax={MAX_HEIGHT}
      tabindex="0"
      on:keydown={(e) => {
        if (e.key === 'ArrowUp') { expandedHeight = Math.min(MAX_HEIGHT, expandedHeight + 20); }
        if (e.key === 'ArrowDown') { expandedHeight = Math.max(MIN_HEIGHT, expandedHeight - 20); }
      }}
    >
      <div class="resize-bar"></div>
    </div>
  {/if}
  <div class="log-header" role="button" tabindex="0" on:click={() => dispatch('toggle')} on:keydown={(e) => e.key === 'Enter' && dispatch('toggle')}>
    <div class="header-left">
      <svg class="chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <polyline points="6,9 12,15 18,9"/>
      </svg>
      <span class="title">Logs</span>
      <span class="log-count">{filteredLogs.length}{#if filteredLogs.length !== logs.length}/{logs.length}{/if}</span>
      <span class="connection-status" class:connected>
        <span class="status-dot"></span>
        {connected ? 'Connected' : 'Disconnected'}
      </span>
    </div>
    <div class="header-right">
      {#if expanded}
        <button
          class="header-btn"
          class:active={showFilters || searchQuery || selectedService}
          on:click|stopPropagation={toggleFilters}
          title="Toggle filters"
          aria-label="Toggle log filters"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polygon points="22,3 2,3 10,12.46 10,19 14,21 14,12.46"/>
          </svg>
        </button>
        <button
          class="header-btn"
          on:click|stopPropagation={() => { autoScroll = true; }}
          title="Scroll to bottom"
          aria-label="Scroll logs to bottom"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="7,13 12,18 17,13"/>
            <line x1="12" y1="6" x2="12" y2="18"/>
          </svg>
        </button>
        <button
          class="header-btn"
          on:click|stopPropagation={() => exportLogs('json')}
          title="Export as JSON"
          aria-label="Export logs as JSON"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
            <polyline points="14,2 14,8 20,8"/>
            <path d="M12 18v-6"/>
            <path d="M9 15l3 3 3-3"/>
          </svg>
        </button>
        <button
          class="header-btn"
          on:click|stopPropagation={() => dispatch('clear')}
          title="Clear logs"
          aria-label="Clear log entries"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="3,6 5,6 21,6"/>
            <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/>
          </svg>
        </button>
      {/if}
    </div>
  </div>

  {#if expanded && showFilters}
    <div class="filter-bar">
      <div class="log-stats">
        <button
          class="stat-badge"
          class:active={selectedLevel === ''}
          on:click={() => selectedLevel = ''}
        >
          All <span class="stat-count">{logStats.total}</span>
        </button>
        <button
          class="stat-badge errors"
          class:active={selectedLevel === 'error'}
          on:click={() => selectedLevel = selectedLevel === 'error' ? '' : 'error'}
        >
          Errors <span class="stat-count">{logStats.errors}</span>
        </button>
        <button
          class="stat-badge warnings"
          class:active={selectedLevel === 'warn'}
          on:click={() => selectedLevel = selectedLevel === 'warn' ? '' : 'warn'}
        >
          Warnings <span class="stat-count">{logStats.warnings}</span>
        </button>
      </div>
      <div class="filter-group">
        <label for="service-filter">Service</label>
        <select id="service-filter" bind:value={selectedService}>
          <option value="">All services</option>
          {#each availableServices as service}
            <option value={service}>{service}</option>
          {/each}
        </select>
      </div>
      <div class="filter-group search">
        <label for="search-input">Search</label>
        <input
          id="search-input"
          type="text"
          placeholder="Filter logs..."
          bind:value={searchQuery}
        />
      </div>
      {#if searchQuery || selectedService || selectedLevel}
        <button class="clear-filters-btn" on:click={clearFilters}>
          Clear
        </button>
      {/if}
    </div>
  {/if}

  {#if expanded}
    <div
      class="log-content"
      bind:this={logContainer}
      on:scroll={handleScroll}
    >
      {#if logs.length === 0}
        <div class="empty-logs">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
            <polyline points="14,2 14,8 20,8"/>
            <line x1="16" y1="13" x2="8" y2="13"/>
            <line x1="16" y1="17" x2="8" y2="17"/>
          </svg>
          <span>No logs yet. Select a stack to stream logs.</span>
        </div>
      {:else if filteredLogs.length === 0}
        <div class="empty-logs">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <circle cx="11" cy="11" r="8"/>
            <line x1="21" y1="21" x2="16.65" y2="16.65"/>
          </svg>
          <span>No logs match your filter criteria.</span>
        </div>
      {:else}
        {#each filteredLogs as log, i (i)}
          <div class="log-line {getLogTypeClass(log)}">
            <span class="log-time">{formatTime(log.timestamp)}</span>
            {#if log.detectedLevel !== 'log' && !log.isStackTrace}
              <span class="log-level {log.detectedLevel}">{log.detectedLevel.toUpperCase()}</span>
            {/if}
            {#if log.service}
              <span class="log-service">[{log.service}]</span>
            {/if}
            <span class="log-content-text">{log.content}</span>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .log-viewer {
    background: var(--bg-secondary);
    border-top: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    transition: height 0.2s ease;
    height: var(--log-collapsed-height);
    position: relative;
  }

  .log-viewer.resizing {
    transition: none;
  }

  .resize-handle {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 8px;
    cursor: ns-resize;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .resize-handle:hover .resize-bar,
  .log-viewer.resizing .resize-bar {
    background: var(--accent-cyan);
    opacity: 1;
  }

  .resize-bar {
    width: 40px;
    height: 3px;
    background: var(--text-tertiary);
    border-radius: 2px;
    opacity: 0.5;
    transition: all 0.15s ease;
  }

  .log-header {
    height: var(--log-collapsed-height);
    padding: 0 20px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    cursor: pointer;
    user-select: none;
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }

  .log-header:hover {
    background: var(--bg-hover);
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .chevron {
    width: 16px;
    height: 16px;
    color: var(--text-tertiary);
    transition: transform 0.2s ease;
  }

  .log-viewer.expanded .chevron {
    transform: rotate(180deg);
  }

  .title {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-secondary);
  }

  .log-count {
    font-size: 0.625rem;
    color: var(--text-tertiary);
    background: var(--bg-surface);
    padding: 2px 8px;
    border-radius: 10px;
  }

  .connection-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.688rem;
    color: var(--text-tertiary);
    padding: 4px 10px;
    background: var(--bg-surface);
    border-radius: 12px;
  }

  .connection-status.connected {
    color: var(--accent-green);
  }

  .connection-status .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--text-tertiary);
  }

  .connection-status.connected .status-dot {
    background: var(--accent-green);
    box-shadow: var(--glow-green);
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .header-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .header-btn:hover {
    background: var(--bg-surface);
    border-color: var(--border-default);
    color: var(--text-primary);
  }

  .header-btn.active {
    background: rgba(0, 229, 255, 0.1);
    border-color: rgba(0, 229, 255, 0.3);
    color: var(--accent-cyan);
  }

  .header-btn svg {
    width: 14px;
    height: 14px;
  }

  .filter-bar {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 10px 20px;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border-subtle);
  }

  .filter-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .filter-group label {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .filter-group select,
  .filter-group input {
    font-family: inherit;
    font-size: 0.75rem;
    padding: 6px 10px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    color: var(--text-primary);
    outline: none;
    transition: border-color 0.15s ease;
  }

  .filter-group select:focus,
  .filter-group input:focus {
    border-color: var(--accent-cyan);
  }

  .filter-group select {
    min-width: 120px;
    cursor: pointer;
  }

  .filter-group.search {
    flex: 1;
    max-width: 300px;
  }

  .filter-group.search input {
    width: 100%;
  }

  .clear-filters-btn {
    font-family: inherit;
    font-size: 0.688rem;
    font-weight: 500;
    padding: 6px 12px;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .clear-filters-btn:hover {
    background: var(--bg-hover);
    border-color: var(--border-default);
    color: var(--text-primary);
  }

  .log-content {
    flex: 1;
    overflow-y: auto;
    padding: 12px 20px;
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.75rem;
    line-height: 1.6;
  }

  .empty-logs {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-tertiary);
    gap: 12px;
    text-align: center;
  }

  .empty-logs svg {
    width: 32px;
    height: 32px;
    opacity: 0.5;
  }

  .log-line {
    display: flex;
    gap: 12px;
    padding: 2px 0;
    border-left: 2px solid transparent;
    padding-left: 8px;
    margin-left: -10px;
  }

  .log-line.error {
    border-left-color: var(--accent-red);
    background: rgba(248, 81, 73, 0.05);
  }

  .log-line.system {
    border-left-color: var(--accent-purple);
    color: var(--accent-purple);
  }

  .log-line.warning {
    border-left-color: var(--accent-orange);
    background: rgba(210, 153, 34, 0.05);
  }

  .log-line.warning .log-content-text {
    color: var(--accent-orange);
  }

  .log-time {
    color: var(--text-tertiary);
    flex-shrink: 0;
    font-size: 0.688rem;
  }

  .log-service {
    color: var(--accent-cyan);
    flex-shrink: 0;
  }

  .log-content-text {
    color: var(--text-secondary);
    word-break: break-all;
  }

  .log-line.error .log-content-text {
    color: var(--accent-red);
  }

  .log-line.info {
    border-left-color: var(--accent-cyan);
  }

  .log-line.debug {
    border-left-color: var(--text-tertiary);
    opacity: 0.7;
  }

  .log-line.stacktrace {
    padding-left: 24px;
    opacity: 0.8;
    font-size: 0.688rem;
  }

  .log-stats {
    display: flex;
    gap: 6px;
  }

  .stat-badge {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 10px;
    font-size: 0.688rem;
    font-weight: 500;
    font-family: inherit;
    background: var(--bg-secondary);
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .stat-badge:hover {
    border-color: var(--border-default);
  }

  .stat-badge.active {
    background: rgba(0, 229, 255, 0.1);
    border-color: rgba(0, 229, 255, 0.3);
    color: var(--accent-cyan);
  }

  .stat-badge.errors {
    color: var(--text-secondary);
  }

  .stat-badge.errors:hover,
  .stat-badge.errors.active {
    background: rgba(248, 81, 73, 0.1);
    border-color: rgba(248, 81, 73, 0.3);
    color: var(--accent-red);
  }

  .stat-badge.warnings {
    color: var(--text-secondary);
  }

  .stat-badge.warnings:hover,
  .stat-badge.warnings.active {
    background: rgba(210, 153, 34, 0.1);
    border-color: rgba(210, 153, 34, 0.3);
    color: var(--accent-orange);
  }

  .stat-count {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.625rem;
    padding: 1px 5px;
    background: var(--bg-surface);
    border-radius: 8px;
  }

  .log-level {
    font-size: 0.563rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 2px 5px;
    border-radius: 3px;
    flex-shrink: 0;
  }

  .log-level.error {
    background: rgba(248, 81, 73, 0.2);
    color: var(--accent-red);
  }

  .log-level.warn {
    background: rgba(210, 153, 34, 0.2);
    color: var(--accent-orange);
  }

  .log-level.info {
    background: rgba(0, 229, 255, 0.2);
    color: var(--accent-cyan);
  }

  .log-level.debug {
    background: rgba(139, 148, 158, 0.2);
    color: var(--text-tertiary);
  }
</style>
