<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import type { ServiceTemplate } from '../api/types';
  import { getTemplates } from '../api/client';

  const dispatch = createEventDispatcher<{ close: void; select: ServiceTemplate }>();

  let templates: ServiceTemplate[] = [];
  let loading = true;
  let error: string | null = null;
  let selectedCategory = '';
  let searchQuery = '';

  const categoryIcons: Record<string, string> = {
    database: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4',
    cache: 'M13 10V3L4 14h7v7l9-11h-7z',
    storage: 'M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z',
    email: 'M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2zM22 6l-10 7L2 6',
    messaging: 'M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z',
    search: 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z',
  };

  const categoryColors: Record<string, string> = {
    database: '#3b82f6',
    cache: '#ef4444',
    storage: '#22c55e',
    email: '#f59e0b',
    messaging: '#8b5cf6',
    search: '#06b6d4',
  };

  onMount(async () => {
    try {
      const response = await getTemplates();
      templates = response.templates;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load templates';
    } finally {
      loading = false;
    }
  });

  $: categories = [...new Set(templates.map(t => t.category))].sort();

  $: filteredTemplates = templates.filter(t => {
    if (selectedCategory && t.category !== selectedCategory) return false;
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      return t.name.toLowerCase().includes(query) ||
             t.description.toLowerCase().includes(query) ||
             t.category.toLowerCase().includes(query);
    }
    return true;
  });

  function handleSelect(template: ServiceTemplate) {
    dispatch('select', template);
  }

  function getCategoryIcon(category: string): string {
    return categoryIcons[category] || categoryIcons.database;
  }

  function getCategoryColor(category: string): string {
    return categoryColors[category] || '#6b7280';
  }
</script>

<div class="template-gallery">
  <div class="gallery-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="3" y="3" width="7" height="7"/>
        <rect x="14" y="3" width="7" height="7"/>
        <rect x="14" y="14" width="7" height="7"/>
        <rect x="3" y="14" width="7" height="7"/>
      </svg>
      <h3>Service Templates</h3>
      <span class="template-count">{templates.length} available</span>
    </div>
    <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close template gallery">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  <div class="gallery-filters">
    <div class="search-box">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="11" cy="11" r="8"/>
        <line x1="21" y1="21" x2="16.65" y2="16.65"/>
      </svg>
      <input
        type="text"
        placeholder="Search templates..."
        aria-label="Search templates"
        bind:value={searchQuery}
        on:keydown={(e) => e.key === 'Escape' && (searchQuery = '')}
      />
      {#if searchQuery}
        <button class="clear-search-btn" on:click={() => searchQuery = ''} aria-label="Clear search">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M18 6L6 18M6 6l12 12"/>
          </svg>
        </button>
      {/if}
    </div>
    <div class="category-filters">
      <button
        class="category-btn"
        class:active={selectedCategory === ''}
        on:click={() => selectedCategory = ''}
      >
        All
      </button>
      {#each categories as category}
        <button
          class="category-btn"
          class:active={selectedCategory === category}
          on:click={() => selectedCategory = selectedCategory === category ? '' : category}
          style="--category-color: {getCategoryColor(category)}"
        >
          {category}
        </button>
      {/each}
    </div>
  </div>

  <div class="gallery-content">
    {#if loading}
      <div class="loading">
        <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
        </svg>
        <span>Loading templates...</span>
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
    {:else if filteredTemplates.length === 0}
      <div class="empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="8"/>
          <line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
        <span>No templates match your search</span>
      </div>
    {:else}
      <div class="template-grid">
        {#each filteredTemplates as template (template.id)}
          <button
            class="template-card"
            on:click={() => handleSelect(template)}
          >
            <div class="card-icon" style="background: {getCategoryColor(template.category)}20; color: {getCategoryColor(template.category)}">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d={getCategoryIcon(template.category)}/>
              </svg>
            </div>
            <div class="card-content">
              <h4>{template.name}</h4>
              <p>{template.description}</p>
              <div class="card-meta">
                <span class="category-tag" style="background: {getCategoryColor(template.category)}20; color: {getCategoryColor(template.category)}">
                  {template.category}
                </span>
                {#if template.ui}
                  <span class="ui-tag">+ UI</span>
                {/if}
                {#if template.ports && template.ports.length > 0}
                  <span class="ports-info">{template.ports.length} ports</span>
                {/if}
              </div>
            </div>
            <div class="card-arrow">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M5 12h14M12 5l7 7-7 7"/>
              </svg>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <div class="gallery-footer">
    <p>Templates provide pre-configured services. Run <code>devrouter add &lt;template-id&gt;</code> to add to your stack.</p>
  </div>
</div>

<style>
  .template-gallery {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    overflow: hidden;
    max-height: 500px;
    display: flex;
    flex-direction: column;
  }

  .gallery-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-surface);
    flex-shrink: 0;
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

  .template-count {
    font-size: 0.625rem;
    font-weight: 600;
    padding: 3px 8px;
    border-radius: 4px;
    background: rgba(0, 229, 255, 0.15);
    color: var(--accent-cyan);
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

  .gallery-filters {
    padding: 12px 18px;
    border-bottom: 1px solid var(--border-subtle);
    display: flex;
    gap: 12px;
    align-items: center;
    flex-wrap: wrap;
    flex-shrink: 0;
  }

  .search-box {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    flex: 1;
    min-width: 200px;
  }

  .search-box svg {
    width: 14px;
    height: 14px;
    color: var(--text-tertiary);
    flex-shrink: 0;
  }

  .search-box input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    font-size: 0.75rem;
    color: var(--text-primary);
    font-family: inherit;
  }

  .search-box input::placeholder {
    color: var(--text-tertiary);
  }

  .clear-search-btn {
    background: transparent;
    border: none;
    padding: 0;
    color: var(--text-tertiary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.15s ease;
  }

  .clear-search-btn:hover {
    color: var(--text-primary);
  }

  .clear-search-btn svg {
    width: 14px;
    height: 14px;
  }

  .category-filters {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  .category-btn {
    padding: 5px 10px;
    font-size: 0.688rem;
    font-weight: 500;
    font-family: inherit;
    text-transform: capitalize;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .category-btn:hover {
    border-color: var(--border-default);
  }

  .category-btn.active {
    background: rgba(0, 229, 255, 0.1);
    border-color: rgba(0, 229, 255, 0.3);
    color: var(--accent-cyan);
  }

  .gallery-content {
    flex: 1;
    overflow-y: auto;
    padding: 16px 18px;
  }

  .loading, .error, .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 40px 20px;
    color: var(--text-tertiary);
    font-size: 0.875rem;
  }

  .loading svg, .error svg, .empty svg {
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

  .template-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .template-card {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 14px 16px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.15s ease;
    text-align: left;
    width: 100%;
    font-family: inherit;
  }

  .template-card:hover {
    border-color: var(--border-default);
    background: var(--bg-hover);
  }

  .card-icon {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 8px;
    flex-shrink: 0;
  }

  .card-icon svg {
    width: 20px;
    height: 20px;
  }

  .card-content {
    flex: 1;
    min-width: 0;
  }

  .card-content h4 {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 4px 0;
  }

  .card-content p {
    font-size: 0.75rem;
    color: var(--text-secondary);
    margin: 0 0 8px 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .card-meta {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .category-tag {
    font-size: 0.625rem;
    font-weight: 600;
    text-transform: uppercase;
    padding: 2px 6px;
    border-radius: 3px;
  }

  .ui-tag {
    font-size: 0.625rem;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 3px;
    background: rgba(139, 92, 246, 0.15);
    color: #a78bfa;
  }

  .ports-info {
    font-size: 0.625rem;
    color: var(--text-tertiary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .card-arrow {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-tertiary);
    flex-shrink: 0;
    opacity: 0;
    transform: translateX(-4px);
    transition: all 0.15s ease;
  }

  .template-card:hover .card-arrow {
    opacity: 1;
    transform: translateX(0);
  }

  .card-arrow svg {
    width: 16px;
    height: 16px;
  }

  .gallery-footer {
    padding: 12px 18px;
    border-top: 1px solid var(--border-subtle);
    background: var(--bg-secondary);
    flex-shrink: 0;
  }

  .gallery-footer p {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    margin: 0;
  }

  .gallery-footer code {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.625rem;
    background: var(--bg-surface);
    padding: 2px 6px;
    border-radius: 3px;
    color: var(--accent-cyan);
  }
</style>
