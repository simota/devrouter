<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import type { EnvVarItem } from '../api/types';
  import { loadStackEnv } from '../lib/envService';
  import {
    buildEnvConsistencyRows,
    deriveEnvServices,
    formatEnvValue,
    getEnvRowStatus,
    type EnvConsistencyRow,
  } from '../lib/envConsistency';

  export let stackId: string;

  const dispatch = createEventDispatcher<{ close: void }>();

  let envItems: EnvVarItem[] = [];
  let loading = true;
  let error = '';
  let filterService = '';
  let filterName = '';
  let activeTab: 'list' | 'consistency' = 'list';
  let showOnlyIssues = false;
  let consistencyRows: EnvConsistencyRow[] = [];
  let filteredConsistencyRows: EnvConsistencyRow[] = [];
  let copiedKey: string | null = null;
  let copiedTimeout: ReturnType<typeof setTimeout> | null = null;

  onMount(async () => {
    loading = true;
    error = '';
    try {
      const result = await loadStackEnv(stackId);
      if (result.ok) {
        envItems = result.value;
      } else {
        error = result.error.message;
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load environment variables';
    }
    loading = false;
  });

  $: services = deriveEnvServices(envItems);

  $: filteredEnv = envItems.filter(item => {
    if (filterService && item.service !== filterService) return false;
    if (!matchesNameFilter(item.name)) return false;
    return true;
  });

  $: consistencyRows = buildEnvConsistencyRows(envItems, services);
  $: filteredConsistencyRows = consistencyRows.filter((row) => {
    if (!matchesNameFilter(row.name)) return false;
    if (showOnlyIssues && !row.hasMissing && !row.hasInconsistent) return false;
    return true;
  });

  function matchesNameFilter(name: string): boolean {
    if (!filterName) return true;
    return name.toLowerCase().includes(filterName.toLowerCase());
  }

  async function copyValue(key: string, value: string) {
    await navigator.clipboard.writeText(value);
    copiedKey = key;
    if (copiedTimeout) {
      clearTimeout(copiedTimeout);
    }
    copiedTimeout = setTimeout(() => {
      copiedKey = null;
      copiedTimeout = null;
    }, 2000);
  }
</script>

<div class="env-viewer">
  <div class="env-header">
    <div class="header-title">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 2L2 7l10 5 10-5-10-5z"/>
        <path d="M2 17l10 5 10-5"/>
        <path d="M2 12l10 5 10-5"/>
      </svg>
      <h3>Environment Variables</h3>
      <span class="count">{activeTab === 'list' ? filteredEnv.length : filteredConsistencyRows.length}</span>
    </div>
    <div class="header-actions">
      <div class="env-tabs">
        <button class:active={activeTab === 'list'} on:click={() => (activeTab = 'list')}>
          List
        </button>
        <button class:active={activeTab === 'consistency'} on:click={() => (activeTab = 'consistency')}>
          Consistency
        </button>
      </div>
      <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close environment viewer">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 6L6 18M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>

  <div class="env-filters">
    {#if activeTab === 'list'}
      <div class="filter-group">
        <label for="env-service-filter">Service</label>
        <select id="env-service-filter" bind:value={filterService}>
          <option value="">All services</option>
          {#each services as svc}
            <option value={svc}>{svc}</option>
          {/each}
        </select>
      </div>
      <div class="filter-group search">
        <label for="env-name-filter">Name</label>
        <input
          id="env-name-filter"
          type="text"
          placeholder="Filter by name..."
          bind:value={filterName}
        />
      </div>
    {:else}
      <div class="filter-group search">
        <label for="env-name-filter">Name</label>
        <input
          id="env-name-filter"
          type="text"
          placeholder="Filter by name..."
          bind:value={filterName}
        />
      </div>
      <label class="issue-toggle">
        <input type="checkbox" bind:checked={showOnlyIssues} />
        Show issues only
      </label>
    {/if}
  </div>

  <div class="env-content">
    {#if loading}
      <div class="loading">
        <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
        </svg>
        Loading environment variables...
      </div>
    {:else if error}
      <div class="error">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        {error}
      </div>
    {:else}
      {#if activeTab === 'list'}
        {#if filteredEnv.length === 0}
          <div class="empty">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/>
              <circle cx="12" cy="7" r="4"/>
            </svg>
            No environment variables found
          </div>
        {:else}
          <table class="env-table">
            <thead>
              <tr>
                <th>Service</th>
                <th>Name</th>
                <th>Value</th>
              </tr>
            </thead>
            <tbody>
              {#each filteredEnv as item (item.service + item.name)}
                {@const copyKey = `${item.service}:${item.name}`}
                <tr>
                  <td class="service-cell">{item.service}</td>
                  <td class="name-cell">{item.name}</td>
                  <td class="value-cell">
                    <span class="value-text" title={item.value}>{item.value || '(empty)'}</span>
                    <button
                      class="copy-btn"
                      class:copied={copiedKey === copyKey}
                      on:click={() => copyValue(copyKey, item.value)}
                      title={copiedKey === copyKey ? 'Copied' : 'Copy value'}
                      aria-label={copiedKey === copyKey ? 'Copied value' : 'Copy value'}
                    >
                      {#if copiedKey === copyKey}
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
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {:else}
        {#if consistencyRows.length === 0}
          <div class="empty">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/>
              <circle cx="12" cy="7" r="4"/>
            </svg>
            No environment variables found
          </div>
        {:else if filteredConsistencyRows.length === 0}
          <div class="empty">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M5 12h14M12 5l7 7-7 7"/>
            </svg>
            No issues found
          </div>
        {:else}
          <table class="env-table consistency-table">
            <thead>
              <tr>
                <th>Variable</th>
                {#each services as svc}
                  <th class="service-cell">{svc}</th>
                {/each}
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {#each filteredConsistencyRows as row (row.name)}
                <tr class:row-inconsistent={row.hasInconsistent} class:row-missing={!row.hasInconsistent && row.hasMissing}>
                  <td class="name-cell">{row.name}</td>
                  {#each services as svc}
                    {@const value = row.valuesByService[svc] ?? null}
                    <td class:missing={value === null} class:empty={value === ''}>
                      {formatEnvValue(value)}
                    </td>
                  {/each}
                  <td>
                    <span class={`status-pill ${getEnvRowStatus(row)}`}>{getEnvRowStatus(row)}</span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {/if}
    {/if}
  </div>
</div>

<style>
  .env-viewer {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    overflow: visible;
    margin-top: 16px;
  }

  .env-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border-subtle);
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .header-title {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .header-title svg {
    width: 18px;
    height: 18px;
    color: var(--accent-green);
  }

  .header-title h3 {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .count {
    font-size: 0.625rem;
    color: var(--accent-green);
    background: rgba(63, 185, 80, 0.1);
    padding: 2px 8px;
    border-radius: 10px;
    font-weight: 600;
  }

  .env-tabs {
    display: inline-flex;
    align-items: center;
    border: 1px solid var(--border-subtle);
    border-radius: 999px;
    overflow: hidden;
    background: var(--bg-elevated);
  }

  .env-tabs button {
    border: none;
    background: transparent;
    color: var(--text-tertiary);
    font-size: 0.688rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    padding: 6px 10px;
    cursor: pointer;
  }

  .env-tabs button.active {
    color: var(--text-primary);
    background: rgba(0, 229, 255, 0.12);
  }

  .close-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 4px;
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
    width: 16px;
    height: 16px;
  }

  .env-filters {
    display: flex;
    gap: 16px;
    padding: 12px 16px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border-subtle);
  }

  .issue-toggle {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 0.75rem;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .issue-toggle input {
    accent-color: var(--accent-orange);
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
    background: var(--bg-surface);
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
    max-width: 250px;
  }

  .filter-group.search input {
    width: 100%;
  }

  .env-content {
    max-height: 300px;
    overflow-y: auto;
  }

  .loading, .error, .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px;
    gap: 12px;
    color: var(--text-tertiary);
    text-align: center;
    font-size: 0.813rem;
  }

  .loading svg, .error svg, .empty svg {
    width: 32px;
    height: 32px;
    opacity: 0.5;
  }

  .error {
    color: var(--accent-red);
  }

  .spinner {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .env-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.75rem;
  }

  .env-table th {
    text-align: left;
    padding: 10px 16px;
    font-size: 0.688rem;
    font-weight: 600;
    color: var(--text-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border-subtle);
    position: sticky;
    top: 0;
  }

  .env-table td {
    padding: 10px 16px;
    border-bottom: 1px solid var(--border-subtle);
    vertical-align: middle;
  }

  .consistency-table td {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.688rem;
  }

  .consistency-table tr.row-inconsistent td {
    background: rgba(248, 81, 73, 0.08);
  }

  .consistency-table tr.row-missing td {
    background: rgba(240, 136, 62, 0.08);
  }

  .env-table tr:hover td {
    background: var(--bg-hover);
  }

  .service-cell {
    color: var(--accent-cyan);
    font-weight: 500;
    white-space: nowrap;
  }

  .name-cell {
    color: var(--text-primary);
    font-family: 'IBM Plex Mono', monospace;
    font-weight: 500;
  }

  .value-cell {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .value-text {
    flex: 1;
    font-family: 'IBM Plex Mono', monospace;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 300px;
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
    opacity: 0;
  }

  .env-table tr:hover .copy-btn {
    opacity: 1;
  }

  .copy-btn.copied {
    opacity: 1;
    color: var(--accent-green);
    background: rgba(57, 211, 83, 0.12);
  }

  .copy-btn:hover {
    background: var(--bg-surface);
    color: var(--text-primary);
  }

  .copy-btn svg {
    width: 14px;
    height: 14px;
  }

  .status-pill {
    display: inline-flex;
    align-items: center;
    padding: 3px 8px;
    border-radius: 999px;
    font-size: 0.6rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    border: 1px solid transparent;
  }

  .status-pill.consistent {
    color: var(--accent-green);
    border-color: rgba(57, 211, 83, 0.4);
    background: rgba(57, 211, 83, 0.1);
  }

  .status-pill.inconsistent {
    color: var(--accent-red);
    border-color: rgba(248, 81, 73, 0.4);
    background: rgba(248, 81, 73, 0.1);
  }

  .status-pill.missing {
    color: var(--accent-orange);
    border-color: rgba(240, 136, 62, 0.4);
    background: rgba(240, 136, 62, 0.1);
  }

  td.missing {
    color: var(--accent-orange);
  }

  td.empty {
    color: var(--text-tertiary);
  }
</style>
