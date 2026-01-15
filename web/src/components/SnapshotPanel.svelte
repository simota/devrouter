<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type {
    StackResponse,
    StackHealthResponse,
    StackStatsResponse,
    StackDependenciesResponse,
    EnvResponse,
    ConfigFileResponse,
    WatchStatus,
  } from '../api/types';
  import {
    getStackHealth,
    getStackStats,
    getStackDependencies,
    getStackEnv,
    getStackConfig,
  } from '../api/client';

  export let stack: StackResponse;

  const dispatch = createEventDispatcher<{ close: void }>();
  const API_BASE = '/api';

  type SnapshotMeta = {
    createdAt: string;
    stackId: string;
    repoPath: string;
    domain: string;
    source: string;
  };

  type SnapshotBase = {
    meta: SnapshotMeta;
    stack: StackResponse;
    health: StackHealthResponse;
    stats: StackStatsResponse;
    dependencies: StackDependenciesResponse;
    env: EnvResponse;
    config: ConfigFileResponse;
    watch: WatchStatus;
  };

  type StackSnapshot = {
    meta: SnapshotMeta;
    envRedacted: boolean;
    stack: StackResponse;
    health: StackHealthResponse;
    stats: StackStatsResponse;
    dependencies: StackDependenciesResponse;
    env: EnvResponse;
    config: ConfigFileResponse;
    watch: WatchStatus;
  };

  let includeRawEnv = false;
  let generating = false;
  let error: string | null = null;
  let snapshotBase: SnapshotBase | null = null;
  let snapshotJson = '';
  let lastGeneratedAt: Date | null = null;
  let copied = false;

  async function getStackWatch(stackId: string): Promise<WatchStatus> {
    const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/watch`);
    if (!res.ok) throw new Error('Failed to fetch watch status');
    return res.json();
  }

  function redactEnv(env: EnvResponse): EnvResponse {
    return {
      ...env,
      env: env.env.map((item) => ({
        ...item,
        value: item.value ? '******' : '',
      })),
    };
  }

  function buildSnapshot(base: SnapshotBase, envRedacted: boolean): StackSnapshot {
    return {
      meta: base.meta,
      envRedacted,
      stack: base.stack,
      health: base.health,
      stats: base.stats,
      dependencies: base.dependencies,
      env: envRedacted ? redactEnv(base.env) : base.env,
      config: base.config,
      watch: base.watch,
    };
  }

  function formatTime(date: Date | null): string {
    if (!date) return 'Not generated';
    return date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function formatFilename(date: Date): string {
    const timestamp = date.toISOString().replace(/[:.]/g, '-');
    return `devrouter-snapshot-${stack.id}-${timestamp}.json`;
  }

  function settledValue<T>(result: PromiseSettledResult<T>, fallback: T): T {
    return result.status === 'fulfilled' ? result.value : fallback;
  }

  async function generateSnapshot() {
    if (generating) return;
    generating = true;
    error = null;

    const createdAt = new Date().toISOString();

    try {
      const [healthResult, statsResult, depsResult, envResult, configResult, watchResult] = await Promise.allSettled([
        getStackHealth(stack.id),
        getStackStats(stack.id),
        getStackDependencies(stack.id),
        getStackEnv(stack.id),
        getStackConfig(stack.id),
        getStackWatch(stack.id),
      ]);

      const health = settledValue(healthResult, {
        stackId: stack.id,
        services: [],
        checkedAt: createdAt,
      });

      const stats = settledValue(statsResult, {
        stackId: stack.id,
        containers: [],
        collectedAt: createdAt,
      });

      const dependencies = settledValue(depsResult, {
        stackId: stack.id,
        nodes: [],
        edges: [],
        serviceNames: stack.services.map((service) => service.name),
      });

      const env = settledValue(envResult, {
        stackId: stack.id,
        env: [],
      });

      const config = settledValue(configResult, {
        path: stack.composeFilePath,
        content: '',
        type: 'compose' as const,
      });

      const watch = settledValue(watchResult, {
        enabled: false,
        watching: false,
        repoPath: stack.repoPath,
        watchedPaths: 0,
        recentEvents: [],
        rules: [],
      });

      snapshotBase = {
        meta: {
          createdAt,
          stackId: stack.id,
          repoPath: stack.repoPath,
          domain: stack.domain,
          source: stack.source,
        },
        stack,
        health,
        stats,
        dependencies,
        env,
        config,
        watch,
      };

      lastGeneratedAt = new Date();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to generate snapshot';
    } finally {
      generating = false;
    }
  }

  async function copySnapshot() {
    if (!snapshotJson) return;
    await navigator.clipboard.writeText(snapshotJson);
    copied = true;
    setTimeout(() => { copied = false; }, 2000);
  }

  function downloadSnapshot() {
    if (!snapshotJson || !snapshotBase) return;
    const createdAt = new Date(snapshotBase.meta.createdAt);
    const blob = new Blob([snapshotJson], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = formatFilename(createdAt);
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  $: if (snapshotBase) {
    snapshotJson = JSON.stringify(buildSnapshot(snapshotBase, !includeRawEnv), null, 2);
  }
</script>

<div class="snapshot-panel">
  <div class="snapshot-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 3v12"/>
        <path d="M8 11l4 4 4-4"/>
        <path d="M4 21h16"/>
      </svg>
      <h3>Snapshot</h3>
      <span class="last-updated">Updated {formatTime(lastGeneratedAt)}</span>
    </div>
    <div class="header-right">
      <button class="action-btn primary" on:click={generateSnapshot} disabled={generating}>
        {#if generating}
          <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
          </svg>
          Generating...
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M5 12h14"/>
            <path d="M12 5v14"/>
          </svg>
          {snapshotBase ? 'Refresh' : 'Generate'}
        {/if}
      </button>
      <button class="action-btn" on:click={copySnapshot} disabled={!snapshotJson}>
        {#if copied}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="20,6 9,17 4,12"/>
          </svg>
          Copied
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="9" y="9" width="13" height="13" rx="2"/>
            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
          </svg>
          Copy JSON
        {/if}
      </button>
      <button class="action-btn" on:click={downloadSnapshot} disabled={!snapshotJson}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 3v12"/>
          <path d="M8 11l4 4 4-4"/>
          <path d="M4 21h16"/>
        </svg>
        Download
      </button>
      <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close snapshot panel">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 6L6 18M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>

  <div class="snapshot-controls">
    <label class="snapshot-toggle">
      <input type="checkbox" bind:checked={includeRawEnv} />
      <span>Include raw env values</span>
    </label>
    <span class="toggle-hint">
      {includeRawEnv ? 'Raw values will be included in the export' : 'Env values are masked by default'}
    </span>
  </div>

  {#if error}
    <div class="error-banner">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="8" x2="12" y2="12"/>
        <line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
      <span>{error}</span>
    </div>
  {/if}

  <div class="snapshot-content">
    {#if generating && !snapshotJson}
      <div class="loading">
        <div class="spinner"></div>
        <span>Generating snapshot...</span>
      </div>
    {:else if !snapshotJson}
      <div class="empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="3" y="3" width="18" height="18" rx="2"/>
          <path d="M8 10h8"/>
          <path d="M8 14h6"/>
        </svg>
        <span>No snapshot yet</span>
      </div>
    {:else}
      <pre class="code-block"><code>{snapshotJson}</code></pre>
    {/if}
  </div>
</div>

<style>
  .snapshot-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    max-height: 540px;
    animation: slideIn 0.3s ease-out;
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

  .snapshot-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-subtle);
    background: linear-gradient(
      180deg,
      rgba(59, 130, 246, 0.05) 0%,
      transparent 100%
    );
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .header-left svg {
    width: 20px;
    height: 20px;
    color: #3b82f6;
  }

  .header-left h3 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .last-updated {
    font-size: 0.688rem;
    color: var(--text-tertiary);
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .action-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    font-size: 0.75rem;
    font-weight: 500;
    font-family: inherit;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .action-btn svg {
    width: 14px;
    height: 14px;
  }

  .action-btn.primary {
    background: rgba(59, 130, 246, 0.12);
    border-color: rgba(59, 130, 246, 0.3);
    color: #3b82f6;
  }

  .action-btn:hover:not(:disabled) {
    background: var(--bg-hover);
    border-color: var(--border-default);
    color: var(--text-primary);
  }

  .action-btn.primary:hover:not(:disabled) {
    background: rgba(59, 130, 246, 0.22);
    border-color: rgba(59, 130, 246, 0.4);
    color: #60a5fa;
  }

  .action-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
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

  .snapshot-controls {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 16px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border-subtle);
  }

  .snapshot-toggle {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 0.688rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-secondary);
  }

  .snapshot-toggle input {
    accent-color: #3b82f6;
  }

  .toggle-hint {
    font-size: 0.688rem;
    color: var(--text-tertiary);
  }

  .error-banner {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 16px;
    background: rgba(248, 81, 73, 0.1);
    border-bottom: 1px solid rgba(248, 81, 73, 0.2);
    color: var(--accent-red);
    font-size: 0.75rem;
  }

  .error-banner svg {
    width: 18px;
    height: 18px;
  }

  .snapshot-content {
    flex: 1;
    overflow: auto;
  }

  .loading,
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 32px;
    color: var(--text-tertiary);
    font-size: 0.813rem;
  }

  .loading .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--border-subtle);
    border-top-color: #3b82f6;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  .spinner {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .empty svg {
    width: 28px;
    height: 28px;
    color: var(--text-tertiary);
  }

  .code-block {
    margin: 0;
    padding: 20px;
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.75rem;
    line-height: 1.7;
    color: var(--text-secondary);
    background: var(--bg-secondary);
    white-space: pre;
    overflow-x: auto;
  }

  .code-block code {
    color: inherit;
  }
</style>
