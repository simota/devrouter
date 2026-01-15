<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import type { ServiceResponse, ServiceHealthStatus } from '../api/types';

  export let service: ServiceResponse;
  export let health: ServiceHealthStatus | null = null;

  const dispatch = createEventDispatcher<{ restart: string; buildRestart: string }>();

  let copyFeedback = false;
  let restarting = false;
  let building = false;
  let menuOpen = false;
  let dropdownEl: HTMLDivElement | null = null;

  function openService() {
    window.open(service.url, '_blank', 'noopener,noreferrer');
  }

  async function copyUrl() {
    await navigator.clipboard.writeText(service.url);
    copyFeedback = true;
    setTimeout(() => { copyFeedback = false; }, 2000);
  }

  function handleRestart() {
    if (restarting || building) return;
    restarting = true;
    menuOpen = false;
    dispatch('restart', service.name);
    setTimeout(() => { restarting = false; }, 3000);
  }

  function handleBuildRestart() {
    if (restarting || building) return;
    building = true;
    menuOpen = false;
    dispatch('buildRestart', service.name);
    setTimeout(() => { building = false; }, 10000);
  }

  function toggleMenu() {
    if (restarting || building) return;
    menuOpen = !menuOpen;
  }

  function handleDocumentClick(event: MouseEvent) {
    if (!menuOpen || !dropdownEl) return;
    const target = event.target as Node | null;
    if (target && !dropdownEl.contains(target)) {
      menuOpen = false;
    }
  }

  function handleDocumentKeydown(event: KeyboardEvent) {
    if (!menuOpen) return;
    if (event.key === 'Escape') {
      menuOpen = false;
    }
  }

  onMount(() => {
    document.addEventListener('click', handleDocumentClick);
    document.addEventListener('keydown', handleDocumentKeydown);
    return () => {
      document.removeEventListener('click', handleDocumentClick);
      document.removeEventListener('keydown', handleDocumentKeydown);
    };
  });

  function getHealthClass(status: string | undefined): string {
    switch (status) {
      case 'healthy': return 'healthy';
      case 'unhealthy': return 'unhealthy';
      default: return 'unknown';
    }
  }

  function getContainerStateClass(state: string | undefined): string {
    switch (state) {
      case 'running': return 'running';
      case 'exited':
      case 'stopped': return 'stopped';
      case 'not_found': return 'not-found';
      default: return 'unknown';
    }
  }

  function getContainerStateLabel(state: string | undefined): string {
    switch (state) {
      case 'running': return 'Running';
      case 'exited': return 'Exited';
      case 'stopped': return 'Stopped';
      case 'not_found': return 'Not Found';
      default: return 'Unknown';
    }
  }
</script>

<div class="service-row">
  <div class="service-indicator {getHealthClass(health?.status)}"></div>

  <div class="service-info">
    <span class="service-name">{service.name}</span>
    <span class="service-path">{service.workspacePath}</span>
  </div>

  <div class="service-health">
    {#if health}
      {#if health.containerState && health.containerState !== 'running'}
        <span class="container-badge {getContainerStateClass(health.containerState)}">
          {getContainerStateLabel(health.containerState)}
        </span>
      {:else}
        <span class="health-badge {getHealthClass(health.status)}">
          {health.status}
        </span>
        {#if health.latency > 0}
          <span class="latency">{health.latency}ms</span>
        {/if}
      {/if}
    {:else}
      <span class="health-badge unknown">checking...</span>
    {/if}
    {#if health?.url || service.healthCheck?.endpoint}
      <a
        class="health-url"
        href={health?.url || `${service.url}${service.healthCheck?.endpoint || ''}`}
        target="_blank"
        rel="noopener noreferrer"
        title={health?.url || `${service.url}${service.healthCheck?.endpoint || ''}`}
      >
        {service.healthCheck?.endpoint || '/'}
      </a>
    {/if}
  </div>

  <div class="service-url" title={service.url}>
    <span class="url-text">{service.host}</span>
    <button class="copy-btn" class:copied={copyFeedback} on:click={copyUrl} title={copyFeedback ? 'Copied!' : 'Copy URL'} aria-label={copyFeedback ? 'Copied URL' : `Copy URL for ${service.name}`}>
      {#if copyFeedback}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="20,6 9,17 4,12"/>
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="9" y="9" width="13" height="13" rx="2"/>
          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
        </svg>
      {/if}
    </button>
  </div>

  <div class="service-port">
    <span class="port-label">PORT</span>
    <span class="port-value">{service.port}</span>
  </div>

  <div class="restart-dropdown" role="menu" tabindex="-1" bind:this={dropdownEl}>
    <button
      class="restart-btn"
      on:click={toggleMenu}
      disabled={restarting || building}
      title="Restart options"
      aria-label="Restart options for {service.name}"
      aria-expanded={menuOpen}
      aria-haspopup="true"
    >
      {#if restarting || building}
        <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M23 4v6h-6"/>
          <path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
        </svg>
      {/if}
    </button>
    {#if menuOpen}
      <div class="restart-menu">
        <button class="menu-item" on:click={handleRestart}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M23 4v6h-6"/>
            <path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
          </svg>
          Restart
        </button>
        <button class="menu-item build" on:click={handleBuildRestart}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M22 11.08V12a10 10 0 11-5.93-9.14"/>
            <polyline points="22 4 12 14.01 9 11.01"/>
          </svg>
          Build & Restart
        </button>
      </div>
    {/if}
  </div>

  <button class="open-btn" on:click={openService} aria-label="Open {service.name} in new tab">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6"/>
      <polyline points="15,3 21,3 21,9"/>
      <line x1="10" y1="14" x2="21" y2="3"/>
    </svg>
    Open
  </button>
</div>

<style>
  .service-row {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 14px 16px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    transition: all 0.15s ease;
  }

  .service-row:hover {
    border-color: var(--border-default);
    background: var(--bg-hover);
  }

  .service-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-tertiary);
    flex-shrink: 0;
    transition: all 0.3s ease;
  }

  .service-indicator.healthy {
    background: var(--accent-green);
    box-shadow: var(--glow-green);
  }

  .service-indicator.unhealthy {
    background: var(--accent-red);
    box-shadow: 0 0 8px rgba(248, 81, 73, 0.4);
  }

  .service-indicator.unknown {
    background: var(--accent-orange);
    animation: pulse 2s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 0.5; }
    50% { opacity: 1; }
  }

  .service-info {
    flex: 0 0 120px;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .service-health {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }

  .health-badge {
    font-size: 0.625rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 3px 8px;
    border-radius: 4px;
    transition: all 0.3s ease;
  }

  .health-badge.healthy {
    background: rgba(63, 185, 80, 0.15);
    color: var(--accent-green);
    border: 1px solid rgba(63, 185, 80, 0.3);
  }

  .health-badge.unhealthy {
    background: rgba(248, 81, 73, 0.15);
    color: var(--accent-red);
    border: 1px solid rgba(248, 81, 73, 0.3);
  }

  .health-badge.unknown {
    background: rgba(210, 153, 34, 0.15);
    color: var(--accent-orange);
    border: 1px solid rgba(210, 153, 34, 0.3);
  }

  .container-badge {
    font-size: 0.625rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 3px 8px;
    border-radius: 4px;
    transition: all 0.3s ease;
  }

  .container-badge.running {
    background: rgba(63, 185, 80, 0.15);
    color: var(--accent-green);
    border: 1px solid rgba(63, 185, 80, 0.3);
  }

  .container-badge.stopped {
    background: rgba(139, 148, 158, 0.15);
    color: var(--text-secondary);
    border: 1px solid rgba(139, 148, 158, 0.3);
  }

  .container-badge.not-found {
    background: rgba(248, 81, 73, 0.15);
    color: var(--accent-red);
    border: 1px solid rgba(248, 81, 73, 0.3);
  }

  .container-badge.unknown {
    background: rgba(210, 153, 34, 0.15);
    color: var(--accent-orange);
    border: 1px solid rgba(210, 153, 34, 0.3);
  }

  .latency {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .health-url {
    font-size: 0.625rem;
    font-family: 'IBM Plex Mono', monospace;
    color: var(--text-tertiary);
    text-decoration: none;
    padding: 2px 6px;
    border-radius: 3px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    transition: all 0.15s ease;
    white-space: nowrap;
    max-width: 100px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .health-url:hover {
    color: var(--accent-cyan);
    border-color: rgba(0, 229, 255, 0.3);
    background: rgba(0, 229, 255, 0.1);
  }

  .service-name {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .service-path {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .service-url {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
  }

  .url-text {
    flex: 1;
    font-size: 0.75rem;
    color: var(--accent-cyan);
    font-family: 'IBM Plex Mono', monospace;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .copy-btn {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all 0.15s ease;
    flex-shrink: 0;
  }

  .copy-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .copy-btn.copied {
    color: var(--accent-green);
  }

  .copy-btn svg {
    width: 14px;
    height: 14px;
  }

  .service-port {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    padding: 6px 14px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    flex-shrink: 0;
  }

  .port-label {
    font-size: 0.563rem;
    color: var(--text-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.1em;
  }

  .port-value {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--accent-orange);
    font-family: 'IBM Plex Mono', monospace;
  }

  .restart-dropdown {
    position: relative;
    flex-shrink: 0;
  }

  .restart-btn {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all 0.15s ease;
    flex-shrink: 0;
  }

  .restart-btn:hover:not(:disabled) {
    background: rgba(0, 229, 255, 0.1);
    border-color: rgba(0, 229, 255, 0.3);
    color: var(--accent-cyan);
  }

  .restart-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .restart-btn svg {
    width: 14px;
    height: 14px;
  }

  .restart-btn .spinner {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .restart-menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 4px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-default);
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    z-index: 100;
    min-width: 160px;
    overflow: hidden;
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 10px 12px;
    font-size: 0.8125rem;
    font-family: inherit;
    background: transparent;
    border: none;
    color: var(--text-primary);
    cursor: pointer;
    transition: all 0.15s ease;
    text-align: left;
  }

  .menu-item:hover {
    background: var(--bg-hover);
  }

  .menu-item svg {
    width: 14px;
    height: 14px;
    color: var(--text-tertiary);
    flex-shrink: 0;
  }

  .menu-item:hover svg {
    color: var(--accent-cyan);
  }

  .menu-item.build:hover {
    background: rgba(210, 153, 34, 0.1);
  }

  .menu-item.build:hover svg {
    color: var(--accent-orange);
  }

  .open-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    font-size: 0.75rem;
    font-weight: 500;
    font-family: inherit;
    background: rgba(0, 229, 255, 0.1);
    border: 1px solid rgba(0, 229, 255, 0.2);
    border-radius: 6px;
    color: var(--accent-cyan);
    cursor: pointer;
    transition: all 0.15s ease;
    flex-shrink: 0;
  }

  .open-btn:hover {
    background: rgba(0, 229, 255, 0.15);
    border-color: rgba(0, 229, 255, 0.3);
    box-shadow: var(--glow-cyan);
  }

  .open-btn svg {
    width: 14px;
    height: 14px;
  }
</style>
