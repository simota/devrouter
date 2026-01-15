<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import type { ConfigFileResponse } from '../api/types';
  import { getStackConfig } from '../api/client';

  export let stackId: string;

  const dispatch = createEventDispatcher<{ close: void }>();

  let config: ConfigFileResponse | null = null;
  let loading = true;
  let error: string | null = null;
  let copied = false;

  onMount(async () => {
    try {
      config = await getStackConfig(stackId);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load config';
    } finally {
      loading = false;
    }
  });

  async function copyContent() {
    if (!config) return;
    await navigator.clipboard.writeText(config.content);
    copied = true;
    setTimeout(() => { copied = false; }, 2000);
  }

  function getFileName(path: string): string {
    return path.split('/').pop() || path;
  }
</script>

<div class="config-viewer">
  <div class="viewer-header">
    <div class="header-left">
      <svg class="file-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
        <polyline points="14,2 14,8 20,8"/>
      </svg>
      <div class="file-info">
        <span class="file-name">{config ? getFileName(config.path) : 'Loading...'}</span>
        {#if config}
          <span class="file-path" title={config.path}>{config.path}</span>
        {/if}
      </div>
    </div>
    <div class="header-right">
      {#if config}
        <button class="action-btn" on:click={copyContent} title="Copy to clipboard">
          {#if copied}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="20,6 9,17 4,12"/>
            </svg>
            Copied!
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="9" y="9" width="13" height="13" rx="2"/>
              <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
            </svg>
            Copy
          {/if}
        </button>
      {/if}
      <button class="close-btn" on:click={() => dispatch('close')} title="Close" aria-label="Close config viewer">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 6L6 18M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>

  <div class="viewer-content">
    {#if loading}
      <div class="loading">
        <div class="spinner"></div>
        <span>Loading configuration...</span>
      </div>
    {:else if error}
      <div class="error">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        <span>{error}</span>
      </div>
    {:else if config}
      <pre class="code-block"><code>{config.content}</code></pre>
    {/if}
  </div>
</div>

<style>
  .config-viewer {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    max-height: 500px;
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

  .viewer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-subtle);
    background: linear-gradient(
      180deg,
      rgba(210, 153, 34, 0.03) 0%,
      transparent 100%
    );
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .file-icon {
    width: 24px;
    height: 24px;
    color: var(--accent-orange);
  }

  .file-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .file-name {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 1rem;
    font-weight: 600;
    color: var(--text-primary);
  }

  .file-path {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    font-family: 'IBM Plex Mono', monospace;
    max-width: 400px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
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

  .action-btn:hover {
    background: var(--bg-hover);
    border-color: var(--border-default);
    color: var(--text-primary);
  }

  .action-btn svg {
    width: 14px;
    height: 14px;
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

  .viewer-content {
    flex: 1;
    overflow: auto;
  }

  .loading,
  .error {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 200px;
    gap: 16px;
    color: var(--text-tertiary);
  }

  .loading .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--border-subtle);
    border-top-color: var(--accent-cyan);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .error svg {
    width: 32px;
    height: 32px;
    color: var(--accent-red);
  }

  .error span {
    color: var(--accent-red);
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
