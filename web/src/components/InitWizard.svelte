<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import type { StackResponse, InitConfigRequest, InitConfigResponse } from '../api/types';
  import { previewInitConfig, applyInitConfig } from '../api/client';

  export let stack: StackResponse;

  const dispatch = createEventDispatcher<{ close: void }>();

  const COPY_FEEDBACK_TIMEOUT_MS = 2000;
  const OVERWRITE_CONFIRM_MESSAGE = 'Overwrite existing devrouter.yaml?';

  let stackName = stack.name;
  let domain = stack.domain;
  let preview: InitConfigResponse | null = null;
  let loading = true;
  let applying = false;
  let error: string | null = null;
  let success: string | null = null;
  let copied = false;

  function buildRequest(force = false): InitConfigRequest {
    return {
      stack: stackName,
      domain,
      force,
    };
  }

  async function loadPreview() {
    loading = true;
    error = null;
    success = null;
    try {
      preview = await previewInitConfig(stack.id, buildRequest());
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load init preview';
    } finally {
      loading = false;
    }
  }

  async function copyPreview() {
    if (!preview) return;
    try {
      await navigator.clipboard.writeText(preview.content);
      copied = true;
      setTimeout(() => { copied = false; }, COPY_FEEDBACK_TIMEOUT_MS);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to copy preview';
    }
  }

  async function applyConfig(force: boolean) {
    if (applying) return;
    applying = true;
    error = null;
    success = null;
    try {
      const response = await applyInitConfig(stack.id, buildRequest(force));
      preview = response;
      success = force ? 'Config overwritten successfully' : 'Config created successfully';
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to apply init config';
    } finally {
      applying = false;
    }
  }

  function confirmOverwrite(): boolean {
    return window.confirm(OVERWRITE_CONFIRM_MESSAGE);
  }

  onMount(() => {
    loadPreview();
  });
</script>

<div class="init-wizard">
  <div class="wizard-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
        <polyline points="14,2 14,8 20,8"/>
      </svg>
      <h3>Init Config</h3>
      {#if preview?.applied}
        <span class="status-badge success">Applied</span>
      {:else}
        <span class="status-badge">Preview</span>
      {/if}
    </div>
    <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close init wizard">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  <div class="wizard-body">
    <div class="settings">
      <div class="field">
        <label for="init-stack-name">Stack Name</label>
        <input id="init-stack-name" type="text" bind:value={stackName} placeholder={stack.name} />
      </div>
      <div class="field">
        <label for="init-domain">Domain</label>
        <input id="init-domain" type="text" bind:value={domain} placeholder={stack.domain} />
      </div>
      <button class="preview-btn" on:click={loadPreview} disabled={loading}>
        {#if loading}
          <span class="spinner"></span>
          Loading...
        {:else}
          Refresh Preview
        {/if}
      </button>
    </div>

    {#if error}
      <div class="message error">
        <span>{error}</span>
      </div>
    {/if}

    {#if success}
      <div class="message success">
        <span>{success}</span>
      </div>
    {/if}

    {#if preview}
      <div class="meta">
        <div class="meta-row">
          <span class="meta-label">Output</span>
          <code>{preview.outputPath}</code>
        </div>
        {#if !preview.applyAllowed}
          <div class="warning">Compose file not found in repository. Apply is disabled.</div>
        {/if}
        {#if preview.requiresForce}
          <div class="warning">devrouter.yaml already exists. Overwrite requires confirmation.</div>
        {/if}
      </div>

      {#if preview.warnings.length > 0}
        <div class="warnings">
          <h4>Warnings</h4>
          <ul>
            {#each preview.warnings as warning}
              <li>{warning}</li>
            {/each}
          </ul>
        </div>
      {/if}

      <div class="preview">
        <pre><code>{preview.content}</code></pre>
      </div>

      <div class="actions">
        <button class="copy-btn" on:click={copyPreview} disabled={!preview}>
          {#if copied}
            Copied
          {:else}
            Copy YAML
          {/if}
        </button>
        {#if preview.requiresForce}
          <button
            class="apply-btn danger"
            on:click={() => confirmOverwrite() && applyConfig(true)}
            disabled={!preview.applyAllowed || applying}
          >
            {applying ? 'Applying...' : 'Overwrite file'}
          </button>
        {:else}
          <button
            class="apply-btn"
            on:click={() => applyConfig(false)}
            disabled={!preview.applyAllowed || applying}
          >
            {applying ? 'Applying...' : 'Create file'}
          </button>
        {/if}
      </div>
    {:else if loading}
      <div class="loading-state">
        <span class="spinner"></span>
        <span>Loading preview...</span>
      </div>
    {/if}
  </div>
</div>

<style>
  .init-wizard {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    max-height: 600px;
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

  .wizard-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-subtle);
    background: linear-gradient(
      180deg,
      rgba(59, 130, 246, 0.08) 0%,
      transparent 100%
    );
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .header-left svg {
    width: 18px;
    height: 18px;
    color: #3b82f6;
  }

  .header-left h3 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 0.938rem;
    font-weight: 600;
    margin: 0;
    color: var(--text-primary);
  }

  .status-badge {
    font-size: 0.625rem;
    font-weight: 600;
    padding: 3px 8px;
    border-radius: 4px;
    background: rgba(59, 130, 246, 0.15);
    color: #3b82f6;
  }

  .status-badge.success {
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
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

  .wizard-body {
    padding: 16px 20px 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .settings {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 12px;
    align-items: end;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  label {
    font-size: 0.688rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-secondary);
    font-weight: 600;
  }

  input {
    padding: 8px 10px;
    border-radius: 6px;
    border: 1px solid var(--border-subtle);
    background: var(--bg-elevated);
    color: var(--text-primary);
    font-size: 0.813rem;
    font-family: inherit;
  }

  .preview-btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 9px 14px;
    border-radius: 6px;
    border: 1px solid rgba(59, 130, 246, 0.3);
    background: rgba(59, 130, 246, 0.12);
    color: #3b82f6;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    transition: transform 0.15s ease-out, opacity 0.2s ease-out;
  }

  .preview-btn:hover:not(:disabled) {
    background: rgba(59, 130, 246, 0.2);
  }

  .preview-btn:active:not(:disabled) {
    transform: scale(0.98);
  }

  .preview-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .message {
    padding: 10px 12px;
    border-radius: 6px;
    font-size: 0.813rem;
  }

  .message.error {
    border: 1px solid rgba(248, 81, 73, 0.3);
    background: rgba(248, 81, 73, 0.08);
    color: var(--accent-red);
  }

  .message.success {
    border: 1px solid rgba(34, 197, 94, 0.3);
    background: rgba(34, 197, 94, 0.08);
    color: #22c55e;
  }

  .meta {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  .meta-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .meta-label {
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-size: 0.625rem;
    font-weight: 600;
    color: var(--text-tertiary);
  }

  .meta code {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.75rem;
    color: var(--text-primary);
  }

  .warning {
    padding: 8px 10px;
    border-radius: 6px;
    border: 1px solid rgba(249, 115, 22, 0.3);
    background: rgba(249, 115, 22, 0.08);
    color: var(--accent-orange);
  }

  .warnings h4 {
    margin: 0 0 8px 0;
    font-size: 0.688rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-secondary);
  }

  .warnings ul {
    margin: 0;
    padding-left: 16px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  .preview {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    padding: 12px;
    max-height: 240px;
    overflow: auto;
  }

  .preview pre {
    margin: 0;
  }

  .preview code {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.75rem;
    color: var(--text-primary);
    white-space: pre;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .copy-btn,
  .apply-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 9px 14px;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    border: 1px solid transparent;
    transition: transform 0.15s ease-out, opacity 0.2s ease-out;
  }

  .copy-btn {
    background: rgba(0, 229, 255, 0.12);
    border-color: rgba(0, 229, 255, 0.25);
    color: var(--accent-cyan);
  }

  .copy-btn:hover:not(:disabled) {
    background: rgba(0, 229, 255, 0.2);
  }

  .copy-btn:active:not(:disabled),
  .apply-btn:active:not(:disabled) {
    transform: scale(0.98);
  }

  .apply-btn {
    background: rgba(34, 197, 94, 0.12);
    border-color: rgba(34, 197, 94, 0.25);
    color: #22c55e;
  }

  .apply-btn:hover:not(:disabled) {
    background: rgba(34, 197, 94, 0.2);
  }

  .apply-btn.danger {
    background: rgba(248, 81, 73, 0.12);
    border-color: rgba(248, 81, 73, 0.25);
    color: var(--accent-red);
  }

  .apply-btn.danger:hover:not(:disabled) {
    background: rgba(248, 81, 73, 0.2);
  }

  button:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .loading-state {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--text-tertiary);
    font-size: 0.813rem;
  }

  .spinner {
    width: 14px;
    height: 14px;
    border: 2px solid rgba(59, 130, 246, 0.2);
    border-top-color: #3b82f6;
    border-radius: 999px;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  @media (prefers-reduced-motion: reduce) {
    .init-wizard {
      animation: none;
    }

    .preview-btn,
    .copy-btn,
    .apply-btn {
      transition: none;
    }
  }
</style>
