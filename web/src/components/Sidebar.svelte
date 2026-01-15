<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type { StackSummary } from '../api/types';

  export let stacks: StackSummary[] = [];
  export let selectedStackId: string | null = null;

  const dispatch = createEventDispatcher<{ select: string }>();

  // Calculate which item should be focusable (roving tabindex)
  $: selectedIndex = stacks.findIndex(s => s.id === selectedStackId);
  $: focusIndex = selectedIndex !== -1 ? selectedIndex : 0;

  function handleSelect(stackId: string) {
    dispatch('select', stackId);
  }

  function handleListKeydown(event: KeyboardEvent) {
    const target = event.target as HTMLElement;
    const button = target.closest('.stack-item');
    if (!button) return;

    if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
      event.preventDefault();
      const currentId = button.getAttribute('data-stack-id');
      const currentIndex = stacks.findIndex(s => s.id === currentId);
      if (currentIndex === -1) return;

      let newIndex;
      if (event.key === 'ArrowDown') {
        newIndex = (currentIndex + 1) % stacks.length;
      } else {
        newIndex = (currentIndex - 1 + stacks.length) % stacks.length;
      }

      const newStack = stacks[newIndex];
      handleSelect(newStack.id);

      // Focus the new button after a tick to allow selection state to update
      setTimeout(() => {
        const nextButton = document.getElementById(`stack-item-${newStack.id}`);
        nextButton?.focus();
      }, 0);
    }
  }

  function formatTime(isoString: string): string {
    const date = new Date(isoString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  }
</script>

<aside class="sidebar">
  <div class="sidebar-header">
    <h2>Stacks</h2>
    <span class="count">{stacks.length}</span>
  </div>

  <div class="stack-list" on:keydown={handleListKeydown} role="listbox" tabindex="-1">
    {#if stacks.length === 0}
      <div class="empty" role="status">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <rect x="3" y="3" width="18" height="18" rx="2"/>
          <path d="M3 9h18M9 21V9"/>
        </svg>
        <span>No stacks running</span>
      </div>
    {:else}
      {#each stacks as stack, i (stack.id)}
        <button
          id="stack-item-{stack.id}"
          data-stack-id={stack.id}
          class="stack-item"
          class:selected={selectedStackId === stack.id}
          on:click={() => handleSelect(stack.id)}
          role="option"
          aria-selected={selectedStackId === stack.id}
          tabindex={i === focusIndex ? 0 : -1}
        >
          <div class="stack-indicator"></div>
          <div class="stack-info">
            <span class="stack-name">{stack.name}</span>
            <span class="stack-meta">
              {stack.serviceCount} service{stack.serviceCount !== 1 ? 's' : ''}
              <span class="separator">·</span>
              {formatTime(stack.lastUpAt)}
            </span>
          </div>
          <div class="stack-badge">{stack.source}</div>
        </button>
      {/each}
    {/if}
  </div>

  <div class="sidebar-footer">
    <span class="hint">
      <kbd>↑</kbd><kbd>↓</kbd> Navigate
    </span>
  </div>
</aside>

<style>
  .sidebar {
    width: var(--sidebar-width);
    background: var(--bg-secondary);
    border-right: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .sidebar-header {
    padding: 20px 20px 16px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid var(--border-subtle);
  }

  .sidebar-header h2 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-secondary);
  }

  .count {
    font-size: 0.625rem;
    color: var(--accent-cyan);
    background: rgba(0, 229, 255, 0.1);
    padding: 2px 8px;
    border-radius: 10px;
    font-weight: 600;
  }

  .stack-list {
    flex: 1;
    overflow-y: auto;
    padding: 12px;
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 160px;
    color: var(--text-tertiary);
    gap: 12px;
  }

  .empty svg {
    width: 32px;
    height: 32px;
    opacity: 0.5;
  }

  .empty span {
    font-size: 0.813rem;
  }

  .stack-item {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 14px;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.15s ease;
    text-align: left;
    margin-bottom: 4px;
    position: relative;
    outline: none;
  }

  .stack-item:focus-visible {
    box-shadow: 0 0 0 2px var(--accent-cyan);
    border-color: var(--accent-cyan);
  }

  .stack-item:hover {
    background: var(--bg-hover);
    border-color: var(--border-subtle);
  }

  .stack-item.selected {
    background: var(--bg-elevated);
    border-color: var(--accent-cyan);
    box-shadow: inset 0 0 0 1px rgba(0, 229, 255, 0.1);
  }

  .stack-item.selected .stack-indicator {
    background: var(--accent-cyan);
    box-shadow: var(--glow-cyan);
  }

  .stack-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--accent-green);
    flex-shrink: 0;
    box-shadow: var(--glow-green);
  }

  .stack-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .stack-name {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .stack-meta {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .separator {
    opacity: 0.5;
  }

  .stack-badge {
    font-size: 0.563rem;
    color: var(--text-tertiary);
    background: var(--bg-surface);
    padding: 2px 6px;
    border-radius: 4px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    flex-shrink: 0;
  }

  .sidebar-footer {
    padding: 12px 20px;
    border-top: 1px solid var(--border-subtle);
  }

  .hint {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 0.688rem;
    color: var(--text-tertiary);
  }

  kbd {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 18px;
    height: 18px;
    padding: 0 4px;
    font-size: 0.625rem;
    font-family: inherit;
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: 3px;
    box-shadow: 0 1px 0 var(--border-default);
  }
</style>
