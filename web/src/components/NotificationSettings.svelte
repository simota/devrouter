<script lang="ts">
  import { onMount } from 'svelte';
  import { notifications } from '../lib/notifications';

  let supported = false;
  let permission: NotificationPermission = 'default';
  let enabled = true;

  onMount(() => {
    supported = notifications.isSupported();
    permission = notifications.getPermission();
    enabled = notifications.isEnabled();
  });

  async function requestPermission() {
    permission = await notifications.requestPermission();
  }

  function toggleEnabled() {
    enabled = !enabled;
    notifications.setEnabled(enabled);
  }

  async function testNotification() {
    if (permission !== 'granted') {
      await requestPermission();
    }
    notifications.info('Test Notification', 'Notifications are working!');
  }
</script>

<div class="notification-settings">
  <div class="setting-row">
    <div class="setting-info">
      <span class="setting-label">Desktop Notifications</span>
      <span class="setting-description">
        {#if !supported}
          Not supported in this browser
        {:else if permission === 'denied'}
          Blocked by browser settings
        {:else if permission === 'default'}
          Click to enable notifications
        {:else}
          Get alerts when services go down
        {/if}
      </span>
    </div>

    {#if supported}
      <div class="setting-controls">
        {#if permission === 'granted'}
          <button class="toggle-btn" class:active={enabled} on:click={toggleEnabled}>
            {enabled ? 'ON' : 'OFF'}
          </button>
          <button class="test-btn" on:click={testNotification} title="Test notification">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/>
              <path d="M13.73 21a2 2 0 01-3.46 0"/>
            </svg>
          </button>
        {:else if permission === 'default'}
          <button class="enable-btn" on:click={requestPermission}>
            Enable
          </button>
        {:else}
          <span class="blocked-badge">Blocked</span>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .notification-settings {
    padding: 12px 16px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
  }

  .setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .setting-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .setting-label {
    font-size: 0.813rem;
    font-weight: 500;
    color: var(--text-primary);
  }

  .setting-description {
    font-size: 0.688rem;
    color: var(--text-tertiary);
  }

  .setting-controls {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .toggle-btn {
    padding: 6px 14px;
    font-size: 0.688rem;
    font-weight: 600;
    font-family: 'IBM Plex Mono', monospace;
    background: rgba(139, 148, 158, 0.15);
    border: 1px solid rgba(139, 148, 158, 0.3);
    border-radius: 20px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .toggle-btn.active {
    background: rgba(63, 185, 80, 0.15);
    border-color: rgba(63, 185, 80, 0.3);
    color: var(--accent-green);
  }

  .toggle-btn:hover {
    border-color: var(--border-default);
  }

  .test-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .test-btn:hover {
    background: var(--bg-hover);
    border-color: var(--border-default);
    color: var(--text-primary);
  }

  .test-btn svg {
    width: 14px;
    height: 14px;
  }

  .enable-btn {
    padding: 6px 14px;
    font-size: 0.75rem;
    font-weight: 500;
    background: rgba(0, 229, 255, 0.1);
    border: 1px solid rgba(0, 229, 255, 0.2);
    border-radius: 6px;
    color: var(--accent-cyan);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .enable-btn:hover {
    background: rgba(0, 229, 255, 0.2);
    border-color: rgba(0, 229, 255, 0.3);
  }

  .blocked-badge {
    font-size: 0.625rem;
    font-weight: 600;
    text-transform: uppercase;
    padding: 4px 10px;
    border-radius: 4px;
    background: rgba(248, 81, 73, 0.15);
    color: var(--accent-red);
  }
</style>
