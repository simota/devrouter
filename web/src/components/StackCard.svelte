<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import ServiceRow from './ServiceRow.svelte';
  import ConfigViewer from './ConfigViewer.svelte';
  import EnvViewer from './EnvViewer.svelte';
  import ResourceMonitor from './ResourceMonitor.svelte';
  import DependencyGraph from './DependencyGraph.svelte';
  import TemplateGallery from './TemplateGallery.svelte';
  import WatchStatus from './WatchStatus.svelte';
  import SnapshotPanel from './SnapshotPanel.svelte';
  import InitWizard from './InitWizard.svelte';
  import Timeline from './Timeline.svelte';
  import HistoryChart from './HistoryChart.svelte';
  import Insights from './Insights.svelte';
  import type { StackResponse, ServiceHealthStatus, ServiceTemplate } from '../api/types';
  import { stopStack, restartStack, restartService, buildRestartService, getStackHealth } from '../api/client';
  import { notifyServiceDown, notifyServiceRecovered, notifyContainerStopped } from '../lib/notifications';

  export let stack: StackResponse;

  const dispatch = createEventDispatcher<{ close: void }>();
  const HEALTH_POLL_INTERVAL_MS = 10000;
  const HEALTH_REFRESH_DELAY_MS = 2000;
  const PATH_COPY_FEEDBACK_TIMEOUT_MS = 2000;

  let stopping = false;
  let restarting = false;
  let healthMap: Map<string, ServiceHealthStatus> = new Map();
  let prevHealthMap: Map<string, ServiceHealthStatus> = new Map();
  let healthInterval: ReturnType<typeof setInterval> | null = null;
  let isFirstHealthCheck = true;
  let healthVersion = 0; // Force reactivity
  let showConfig = false;
  let showEnv = false;
  let showStats = false;
  let showDeps = false;
  let showTemplates = false;
  let showWatch = false;
  let showSnapshot = false;
  let showInitWizard = false;
  let showTimeline = false;
  let showHistoryChart = false;
  let showInsights = false;
  let selectedTemplate: ServiceTemplate | null = null;
  let lastHealthCheck: Date | null = null;
  let pathCopyFeedback = false;
  let templateCopyFeedback = false;

  function formatDate(isoString: string): string {
    return new Date(isoString).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  async function fetchHealth() {
    try {
      const response = await getStackHealth(stack.id);
      const newMap = new Map<string, ServiceHealthStatus>();
      for (const svc of response.services) {
        newMap.set(svc.name, svc);
      }

      // Check for health changes and send notifications (skip first check)
      if (!isFirstHealthCheck) {
        for (const [name, newHealth] of newMap) {
          const prevHealth = prevHealthMap.get(name);

          // Container stopped
          if (prevHealth?.containerState === 'running' && newHealth.containerState !== 'running') {
            notifyContainerStopped(stack.name, name);
          }

          // Service went unhealthy
          if (prevHealth?.status === 'healthy' && newHealth.status === 'unhealthy') {
            notifyServiceDown(stack.name, name);
          }

          // Service recovered
          if (prevHealth?.status === 'unhealthy' && newHealth.status === 'healthy') {
            notifyServiceRecovered(stack.name, name);
          }
        }
      }

      prevHealthMap = healthMap;
      healthMap = newMap;
      healthVersion++; // Trigger reactivity
      lastHealthCheck = new Date();
      isFirstHealthCheck = false;
    } catch (e) {
      console.error('Failed to fetch health:', e);
    }
  }

  function scheduleHealthRefresh() {
    setTimeout(fetchHealth, HEALTH_REFRESH_DELAY_MS);
  }

  function formatTime(date: Date | null): string {
    if (!date) return '—';
    return date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  onMount(() => {
    fetchHealth();
    healthInterval = setInterval(fetchHealth, HEALTH_POLL_INTERVAL_MS);
  });

  onDestroy(() => {
    if (healthInterval) {
      clearInterval(healthInterval);
    }
  });

  async function handleStop() {
    if (stopping) return;
    stopping = true;
    try {
      await stopStack(stack.id);
      dispatch('close');
    } catch (e) {
      console.error('Failed to stop stack:', e);
    } finally {
      stopping = false;
    }
  }

  async function handleRestart() {
    if (restarting) return;
    restarting = true;
    try {
      await restartStack(stack.id);
      // Refresh health after restart
      scheduleHealthRefresh();
    } catch (e) {
      console.error('Failed to restart stack:', e);
    } finally {
      restarting = false;
    }
  }

  function getServiceHealth(serviceName: string): ServiceHealthStatus | null {
    // healthVersion is used to trigger reactivity
    void healthVersion;
    return healthMap.get(serviceName) || null;
  }

  async function handleServiceRestart(event: CustomEvent<string>) {
    const serviceName = event.detail;
    try {
      await restartService(stack.id, serviceName);
      // Refresh health after restart
      scheduleHealthRefresh();
    } catch (e) {
      console.error('Failed to restart service:', e);
    }
  }

  async function handleServiceBuildRestart(event: CustomEvent<string>) {
    const serviceName = event.detail;
    try {
      await buildRestartService(stack.id, serviceName);
      // Refresh health after build restart
      scheduleHealthRefresh();
    } catch (e) {
      console.error('Failed to build and restart service:', e);
    }
  }

  function areAllServicesStopped(): boolean {
    if (healthMap.size === 0) return false;
    for (const health of healthMap.values()) {
      if (health.containerState === 'running') {
        return false;
      }
    }
    return true;
  }

  function handleTemplateSelect(event: CustomEvent<ServiceTemplate>) {
    selectedTemplate = event.detail;
    templateCopyFeedback = false;
    showTemplates = false;
  }

  function clearSelectedTemplate() {
    selectedTemplate = null;
    templateCopyFeedback = false;
  }

  async function copyTemplateCommand() {
    if (!selectedTemplate) return;
    try {
      await navigator.clipboard.writeText(`devrouter add ${selectedTemplate.id}`);
      templateCopyFeedback = true;
    } catch (e) {
      console.error('Failed to copy template command:', e);
    }
  }

  async function copyPath() {
    await navigator.clipboard.writeText(stack.repoPath);
    pathCopyFeedback = true;
    setTimeout(() => { pathCopyFeedback = false; }, PATH_COPY_FEEDBACK_TIMEOUT_MS);
  }

  function formatRelativeTime(isoString: string): string {
    const date = new Date(isoString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  }

  $: allStopped = areAllServicesStopped();
</script>

<div class="stack-card">
  <div class="card-header">
    <div class="header-left">
      <div class="stack-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
        </svg>
      </div>
      <div class="stack-title">
        <h1>{stack.name}</h1>
        <span class="stack-id">{stack.id}</span>
      </div>
    </div>
    <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close stack details">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  <div class="card-meta">
    <div class="meta-item">
      <span class="meta-label">Source</span>
      <span class="meta-value source">{stack.source}</span>
    </div>
    <div class="meta-item">
      <span class="meta-label">Domain</span>
      <span class="meta-value">{stack.domain}</span>
    </div>
    <div class="meta-item">
      <span class="meta-label">Started</span>
      <span class="meta-value">{formatDate(stack.lastUpAt)}</span>
    </div>
    <div class="meta-item">
      <span class="meta-label">Path</span>
      <div class="path-container">
        <span class="meta-value path" title={stack.repoPath}>{stack.repoPath}</span>
        <button class="copy-btn-small" class:copied={pathCopyFeedback} on:click={copyPath} title={pathCopyFeedback ? 'Copied!' : 'Copy Path'} aria-label={pathCopyFeedback ? 'Copied path' : 'Copy path'}>
          {#if pathCopyFeedback}
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
    </div>
  </div>

  {#if allStopped || stack.lastDownAt}
    <div class="stopped-banner">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/>
        <line x1="12" y1="9" x2="12" y2="13"/>
        <line x1="12" y1="17" x2="12.01" y2="17"/>
      </svg>
      <span>
        {#if stack.lastDownAt}
          Containers stopped {formatRelativeTime(stack.lastDownAt)}
        {:else}
          All containers are stopped
        {/if}
      </span>
    </div>
  {/if}

  <div class="services-section">
    <div class="section-header">
      <h2>Services</h2>
      <span class="count">{stack.services.length}</span>
      <span class="health-updated">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <polyline points="12,6 12,12 16,14"/>
        </svg>
        {formatTime(lastHealthCheck)}
      </span>
    </div>

    <div class="services-list">
      {#each stack.services as service (service.composeService)}
        {@const health = ((_v) => healthMap.get(service.name) || null)(healthVersion)}
        <ServiceRow {service} {health} on:restart={handleServiceRestart} on:buildRestart={handleServiceBuildRestart} />
      {/each}
    </div>
  </div>

  {#if showConfig}
    <div class="config-section">
      <ConfigViewer stackId={stack.id} on:close={() => showConfig = false} />
    </div>
  {/if}

  {#if showInitWizard}
    <div class="config-section">
      <InitWizard {stack} on:close={() => showInitWizard = false} />
    </div>
  {/if}

  {#if showSnapshot}
    <div class="config-section">
      <SnapshotPanel {stack} on:close={() => showSnapshot = false} />
    </div>
  {/if}

  {#if showEnv}
    <div class="config-section">
      <EnvViewer stackId={stack.id} on:close={() => showEnv = false} />
    </div>
  {/if}

  {#if showStats}
    <div class="config-section">
      <ResourceMonitor stackId={stack.id} stackName={stack.name} on:close={() => showStats = false} />
    </div>
  {/if}

  {#if showDeps}
    <div class="config-section">
      <DependencyGraph stackId={stack.id} on:close={() => showDeps = false} />
    </div>
  {/if}

  {#if showTemplates}
    <div class="config-section">
      <TemplateGallery on:close={() => showTemplates = false} on:select={handleTemplateSelect} />
    </div>
  {/if}

  {#if showWatch}
    <div class="config-section">
      <WatchStatus stackId={stack.id} on:close={() => showWatch = false} />
    </div>
  {/if}

  {#if showTimeline}
    <div class="config-section">
      <Timeline stackId={stack.id} on:close={() => showTimeline = false} />
    </div>
  {/if}

  {#if showHistoryChart}
    <div class="config-section">
      <HistoryChart stackId={stack.id} on:close={() => showHistoryChart = false} />
    </div>
  {/if}

  {#if showInsights}
    <div class="config-section">
      <Insights stackId={stack.id} on:close={() => showInsights = false} />
    </div>
  {/if}

  {#if selectedTemplate}
    <div class="template-info">
      <div class="template-info-header">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        <span>Add "{selectedTemplate.name}" to your stack</span>
        <button class="close-template-btn" on:click={clearSelectedTemplate} aria-label="Close template info">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M18 6L6 18M6 6l12 12"/>
          </svg>
        </button>
      </div>
      <div class="template-command">
        <code>devrouter add {selectedTemplate.id}</code>
        <button class="copy-btn" on:click={copyTemplateCommand} aria-label="Copy add command">
          {#if templateCopyFeedback}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="20,6 9,17 4,12"/>
            </svg>
            Copied
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="9" y="9" width="13" height="13" rx="2"/>
              <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
            </svg>
            Copy
          {/if}
        </button>
      </div>
      <p class="template-desc">{selectedTemplate.description}</p>
      <p class="template-desc" aria-live="polite">
        {templateCopyFeedback
          ? '✓ Step 1: Command copied to clipboard'
          : 'Step 1: Copy command and run in terminal'}
      </p>
      <p class="template-desc">Step 2: Click Restart to apply changes.</p>
    </div>
  {/if}

  <div class="card-actions">
    <button class="action-btn timeline" on:click={() => showTimeline = !showTimeline} aria-pressed={showTimeline ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10"/>
        <polyline points="12 6 12 12 16 14"/>
      </svg>
      {showTimeline ? 'Hide' : 'Timeline'}
    </button>
    <button class="action-btn history" on:click={() => showHistoryChart = !showHistoryChart} aria-pressed={showHistoryChart ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M3 3v18h18"/>
        <path d="M18 9l-5 5-4-4-3 3"/>
      </svg>
      {showHistoryChart ? 'Hide' : 'History'}
    </button>
    <button class="action-btn insights" on:click={() => showInsights = !showInsights} aria-pressed={showInsights ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
      </svg>
      {showInsights ? 'Hide' : 'Insights'}
    </button>
    <button class="action-btn watch" on:click={() => showWatch = !showWatch} aria-pressed={showWatch ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
        <path d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/>
      </svg>
      {showWatch ? 'Hide' : 'Watch'}
    </button>
    <button class="action-btn templates" on:click={() => showTemplates = !showTemplates} aria-pressed={showTemplates ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="3" y="3" width="7" height="7"/>
        <rect x="14" y="3" width="7" height="7"/>
        <rect x="14" y="14" width="7" height="7"/>
        <rect x="3" y="14" width="7" height="7"/>
      </svg>
      {showTemplates ? 'Hide' : 'Add Service'}
    </button>
    <button class="action-btn deps" on:click={() => showDeps = !showDeps} aria-pressed={showDeps ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="5" r="3"/>
        <line x1="12" y1="8" x2="12" y2="14"/>
        <circle cx="6" cy="19" r="3"/>
        <circle cx="18" cy="19" r="3"/>
        <line x1="12" y1="14" x2="6" y2="16"/>
        <line x1="12" y1="14" x2="18" y2="16"/>
      </svg>
      {showDeps ? 'Hide Deps' : 'Dependencies'}
    </button>
    <button class="action-btn stats" on:click={() => showStats = !showStats} aria-pressed={showStats ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="2" y="3" width="20" height="14" rx="2"/>
        <line x1="8" y1="21" x2="16" y2="21"/>
        <line x1="12" y1="17" x2="12" y2="21"/>
      </svg>
      {showStats ? 'Hide Stats' : 'Resources'}
    </button>
    <button class="action-btn env" on:click={() => showEnv = !showEnv} aria-pressed={showEnv ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 2L2 7l10 5 10-5-10-5z"/>
        <path d="M2 17l10 5 10-5"/>
        <path d="M2 12l10 5 10-5"/>
      </svg>
      {showEnv ? 'Hide Env' : 'View Env'}
    </button>
    <button class="action-btn config" on:click={() => showConfig = !showConfig} aria-pressed={showConfig ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
        <polyline points="14,2 14,8 20,8"/>
      </svg>
      {showConfig ? 'Hide Config' : 'View Config'}
    </button>
    <button class="action-btn init" on:click={() => showInitWizard = !showInitWizard} aria-pressed={showInitWizard ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 3v12"/>
        <path d="M8 11l4 4 4-4"/>
        <path d="M4 21h16"/>
      </svg>
      {showInitWizard ? 'Hide Init' : 'Init Config'}
    </button>
    <button class="action-btn snapshot" on:click={() => showSnapshot = !showSnapshot} aria-pressed={showSnapshot ? 'true' : 'false'}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 3v12"/>
        <path d="M8 11l4 4 4-4"/>
        <path d="M4 21h16"/>
      </svg>
      {showSnapshot ? 'Hide Snapshot' : 'Snapshot'}
    </button>
    <button class="action-btn restart" on:click={handleRestart} disabled={restarting}>
      {#if restarting}
        <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
        </svg>
        Restarting...
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M23 4v6h-6"/>
          <path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
        </svg>
        Restart
      {/if}
    </button>
    <button class="action-btn stop" on:click={handleStop} disabled={stopping}>
      {#if stopping}
        <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
        </svg>
        Stopping...
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="6" y="6" width="12" height="12" rx="1"/>
        </svg>
        Stop Stack
      {/if}
    </button>
  </div>
</div>

<style>
  .stack-card {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    overflow: visible;
    animation: slideIn 0.3s ease-out;
    display: flex;
    flex-direction: column;
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 20px 24px;
    border-bottom: 1px solid var(--border-subtle);
    background: linear-gradient(
      180deg,
      rgba(0, 229, 255, 0.03) 0%,
      transparent 100%
    );
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .stack-icon {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0, 229, 255, 0.1);
    border: 1px solid rgba(0, 229, 255, 0.2);
    border-radius: 10px;
    color: var(--accent-cyan);
  }

  .stack-icon svg {
    width: 22px;
    height: 22px;
  }

  .stack-title {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .stack-title h1 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 1.375rem;
    font-weight: 600;
    color: var(--text-primary);
    letter-spacing: -0.02em;
    margin: 0;
  }

  .stack-id {
    font-size: 0.75rem;
    color: var(--text-tertiary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .close-btn {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .close-btn:hover {
    background: var(--bg-hover);
    border-color: var(--border-default);
    color: var(--text-primary);
  }

  .close-btn svg {
    width: 18px;
    height: 18px;
  }

  .card-meta {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 16px;
    padding: 20px 24px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-secondary);
  }

  .meta-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .meta-label {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .meta-value {
    font-size: 0.875rem;
    color: var(--text-primary);
  }

  .meta-value.source {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .meta-value.source::before {
    content: '';
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent-green);
    box-shadow: var(--glow-green);
  }

  .meta-value.path {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.75rem;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 300px;
  }

  .path-container {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .copy-btn-small {
    width: 20px;
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all 0.15s ease;
    padding: 0;
    flex-shrink: 0;
  }

  .copy-btn-small:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .copy-btn-small.copied {
    color: var(--accent-green);
  }

  .copy-btn-small svg {
    width: 14px;
    height: 14px;
  }

  .stopped-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 0 24px;
    padding: 12px 16px;
    background: rgba(210, 153, 34, 0.1);
    border: 1px solid rgba(210, 153, 34, 0.2);
    border-radius: 8px;
    color: var(--accent-orange);
    font-size: 0.813rem;
    font-weight: 500;
  }

  .stopped-banner svg {
    width: 18px;
    height: 18px;
    flex-shrink: 0;
  }

  .services-section {
    padding: 20px 24px;
  }

  .section-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
  }

  .section-header h2 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
  }

  .section-header .count {
    font-size: 0.625rem;
    color: var(--accent-cyan);
    background: rgba(0, 229, 255, 0.1);
    padding: 2px 8px;
    border-radius: 10px;
    font-weight: 600;
  }

  .health-updated {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.688rem;
    color: var(--text-tertiary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .health-updated svg {
    width: 12px;
    height: 12px;
  }

  .services-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .config-section {
    padding: 0 24px 20px;
  }

  .card-actions {
    padding: 16px 24px;
    border-top: 1px solid var(--border-subtle);
    background: var(--bg-secondary);
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
  }

  .action-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 10px 18px;
    font-size: 0.813rem;
    font-weight: 500;
    font-family: inherit;
    border-radius: 6px;
    cursor: pointer;
    transform: translateY(0);
    transition:
      transform 0.15s cubic-bezier(0, 0, 0.2, 1),
      opacity 0.15s ease,
      background 0.15s ease,
      border-color 0.15s ease,
      color 0.15s ease;
  }

  .action-btn svg {
    width: 16px;
    height: 16px;
  }

  .action-btn.deps {
    background: rgba(56, 189, 248, 0.1);
    border: 1px solid rgba(56, 189, 248, 0.2);
    color: #38bdf8;
  }

  .action-btn.deps:hover {
    background: rgba(56, 189, 248, 0.2);
    border-color: rgba(56, 189, 248, 0.3);
  }

  .action-btn.stats {
    background: rgba(139, 92, 246, 0.1);
    border: 1px solid rgba(139, 92, 246, 0.2);
    color: #a78bfa;
  }

  .action-btn.stats:hover {
    background: rgba(139, 92, 246, 0.2);
    border-color: rgba(139, 92, 246, 0.3);
  }

  .action-btn.env {
    background: rgba(63, 185, 80, 0.1);
    border: 1px solid rgba(63, 185, 80, 0.2);
    color: var(--accent-green);
  }

  .action-btn.env:hover {
    background: rgba(63, 185, 80, 0.2);
    border-color: rgba(63, 185, 80, 0.3);
  }

  .action-btn.config {
    background: rgba(210, 153, 34, 0.1);
    border: 1px solid rgba(210, 153, 34, 0.2);
    color: var(--accent-orange);
  }

  .action-btn.config:hover {
    background: rgba(210, 153, 34, 0.2);
    border-color: rgba(210, 153, 34, 0.3);
  }

  .action-btn.init {
    background: rgba(99, 102, 241, 0.1);
    border: 1px solid rgba(99, 102, 241, 0.2);
    color: #818cf8;
  }

  .action-btn.init:hover {
    background: rgba(99, 102, 241, 0.2);
    border-color: rgba(99, 102, 241, 0.3);
  }

  .action-btn.snapshot {
    background: rgba(59, 130, 246, 0.1);
    border: 1px solid rgba(59, 130, 246, 0.2);
    color: #3b82f6;
  }

  .action-btn.snapshot:hover {
    background: rgba(59, 130, 246, 0.2);
    border-color: rgba(59, 130, 246, 0.3);
  }

  .action-btn.restart {
    background: rgba(0, 229, 255, 0.1);
    border: 1px solid rgba(0, 229, 255, 0.2);
    color: var(--accent-cyan);
  }

  .action-btn.restart:hover:not(:disabled) {
    background: rgba(0, 229, 255, 0.2);
    border-color: rgba(0, 229, 255, 0.3);
  }

  .action-btn.stop {
    background: rgba(248, 81, 73, 0.1);
    border: 1px solid rgba(248, 81, 73, 0.2);
    color: var(--accent-red);
  }

  .action-btn.stop:hover:not(:disabled) {
    background: rgba(248, 81, 73, 0.2);
    border-color: rgba(248, 81, 73, 0.3);
  }

  .action-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .action-btn:active:not(:disabled) {
    transform: translateY(1px) scale(0.98);
  }

  @media (prefers-reduced-motion: reduce) {
    .action-btn {
      transition: none;
    }

    .action-btn:active:not(:disabled) {
      transform: none;
    }
  }

  .spinner {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .action-btn.timeline {
    background: rgba(163, 113, 247, 0.1);
    border: 1px solid rgba(163, 113, 247, 0.2);
    color: var(--accent-purple);
  }

  .action-btn.timeline:hover {
    background: rgba(163, 113, 247, 0.2);
    border-color: rgba(163, 113, 247, 0.3);
  }

  .action-btn.history {
    background: rgba(6, 182, 212, 0.1);
    border: 1px solid rgba(6, 182, 212, 0.2);
    color: var(--accent-cyan);
  }

  .action-btn.history:hover {
    background: rgba(6, 182, 212, 0.2);
    border-color: rgba(6, 182, 212, 0.3);
  }

  .action-btn.insights {
    background: rgba(249, 115, 22, 0.1);
    border: 1px solid rgba(249, 115, 22, 0.2);
    color: var(--accent-orange);
  }

  .action-btn.insights:hover {
    background: rgba(249, 115, 22, 0.2);
    border-color: rgba(249, 115, 22, 0.3);
  }

  .action-btn.watch {
    background: rgba(168, 85, 247, 0.1);
    border: 1px solid rgba(168, 85, 247, 0.2);
    color: #a855f7;
  }

  .action-btn.watch:hover {
    background: rgba(168, 85, 247, 0.2);
    border-color: rgba(168, 85, 247, 0.3);
  }

  .action-btn.templates {
    background: rgba(34, 197, 94, 0.1);
    border: 1px solid rgba(34, 197, 94, 0.2);
    color: #22c55e;
  }

  .action-btn.templates:hover {
    background: rgba(34, 197, 94, 0.2);
    border-color: rgba(34, 197, 94, 0.3);
  }

  .template-info {
    margin: 0 24px 20px;
    padding: 16px;
    background: rgba(34, 197, 94, 0.05);
    border: 1px solid rgba(34, 197, 94, 0.2);
    border-radius: 8px;
  }

  .template-info-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
    color: #22c55e;
    font-size: 0.875rem;
    font-weight: 500;
  }

  .template-info-header svg {
    width: 18px;
    height: 18px;
    flex-shrink: 0;
  }

  .template-info-header span {
    flex: 1;
  }

  .close-template-btn {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .close-template-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .close-template-btn svg {
    width: 14px;
    height: 14px;
  }

  .template-command {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    margin-bottom: 10px;
  }

  .template-command code {
    flex: 1;
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.813rem;
    color: var(--accent-cyan);
  }

  .copy-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    background: rgba(0, 229, 255, 0.1);
    border: 1px solid rgba(0, 229, 255, 0.2);
    border-radius: 4px;
    color: var(--accent-cyan);
    font-size: 0.75rem;
    font-weight: 500;
    font-family: inherit;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .copy-btn:hover {
    background: rgba(0, 229, 255, 0.2);
  }

  .copy-btn svg {
    width: 14px;
    height: 14px;
  }

  .template-desc {
    font-size: 0.75rem;
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.5;
  }
</style>
