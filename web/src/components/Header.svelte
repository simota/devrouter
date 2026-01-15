<script lang="ts">
  import { onMount } from 'svelte';
  import { getHealth } from '../api/client';
  import { notifications } from '../lib/notifications';

  let dockerStatus: 'ok' | 'degraded' | 'unknown' = 'unknown';
  let checking = true;
  let notificationsEnabled = false;
  let notificationPermission: NotificationPermission = 'default';
  let showNotificationMenu = false;

  onMount(() => {
    checkHealth();
    const interval = setInterval(checkHealth, 15000);
    notificationsEnabled = notifications.isEnabled();
    notificationPermission = notifications.getPermission();
    return () => clearInterval(interval);
  });

  async function checkHealth() {
    try {
      const health = await getHealth();
      dockerStatus = health.docker ? 'ok' : 'degraded';
    } catch {
      dockerStatus = 'unknown';
    } finally {
      checking = false;
    }
  }

  async function toggleNotifications() {
    if (notificationPermission === 'default') {
      notificationPermission = await notifications.requestPermission();
    }
    if (notificationPermission === 'granted') {
      notificationsEnabled = !notificationsEnabled;
      notifications.setEnabled(notificationsEnabled);
    }
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.notification-control')) {
      showNotificationMenu = false;
    }
  }
</script>

<svelte:window on:click={handleClickOutside} />

<header class="header">
  <div class="logo">
    <div class="logo-icon">
      <svg viewBox="0 0 24 24" fill="none">
        <path
          d="M4 6h16M4 12h16M4 18h8"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
        />
        <circle cx="18" cy="18" r="3" fill="currentColor" class="pulse" />
      </svg>
    </div>
    <span class="logo-text">DevRouter</span>
    <span class="version">v0.1</span>
  </div>

  <div class="status">
    <div class="notification-control">
      <button
        class="notification-btn"
        class:enabled={notificationsEnabled && notificationPermission === 'granted'}
        class:blocked={notificationPermission === 'denied'}
        on:click|stopPropagation={toggleNotifications}
        title={notificationPermission === 'denied' ? 'Notifications blocked' : notificationsEnabled ? 'Notifications enabled' : 'Enable notifications'}
        aria-label={notificationPermission === 'denied' ? 'Notifications blocked' : notificationsEnabled ? 'Disable notifications' : 'Enable notifications'}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/>
          <path d="M13.73 21a2 2 0 01-3.46 0"/>
        </svg>
        {#if notificationsEnabled && notificationPermission === 'granted'}
          <span class="notification-badge"></span>
        {/if}
      </button>
    </div>

    <div class="status-item">
      <span class="status-label">Docker</span>
      <span class="status-indicator" class:ok={dockerStatus === 'ok'} class:degraded={dockerStatus === 'degraded'}>
        {#if checking}
          <span class="status-dot checking"></span>
          <span>Checking...</span>
        {:else if dockerStatus === 'ok'}
          <span class="status-dot ok"></span>
          <span>Running</span>
        {:else if dockerStatus === 'degraded'}
          <span class="status-dot degraded"></span>
          <span>Degraded</span>
        {:else}
          <span class="status-dot unknown"></span>
          <span>Unknown</span>
        {/if}
      </span>
    </div>
  </div>
</header>

<style>
  .header {
    height: var(--header-height);
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border-subtle);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 24px;
    position: relative;
    z-index: 100;
  }

  .header::after {
    content: '';
    position: absolute;
    bottom: -1px;
    left: 0;
    right: 0;
    height: 1px;
    background: linear-gradient(
      90deg,
      transparent,
      var(--accent-cyan) 20%,
      var(--accent-cyan) 80%,
      transparent
    );
    opacity: 0.3;
  }

  .logo {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .logo-icon {
    width: 28px;
    height: 28px;
    color: var(--accent-cyan);
    filter: drop-shadow(var(--glow-cyan));
  }

  .logo-icon svg {
    width: 100%;
    height: 100%;
  }

  .logo-icon .pulse {
    animation: pulse 2s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .logo-text {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 1.25rem;
    font-weight: 700;
    letter-spacing: -0.02em;
    background: linear-gradient(135deg, var(--text-primary), var(--accent-cyan));
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .version {
    font-size: 0.625rem;
    color: var(--text-tertiary);
    background: var(--bg-surface);
    padding: 2px 6px;
    border-radius: 4px;
    border: 1px solid var(--border-subtle);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .status {
    display: flex;
    align-items: center;
    gap: 24px;
  }

  .status-item {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .status-label {
    font-size: 0.75rem;
    color: var(--text-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .status-indicator {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.75rem;
    color: var(--text-secondary);
    padding: 4px 10px;
    background: var(--bg-surface);
    border-radius: 12px;
    border: 1px solid var(--border-subtle);
  }

  .status-indicator.ok {
    color: var(--accent-green);
    border-color: rgba(57, 211, 83, 0.2);
  }

  .status-indicator.degraded {
    color: var(--accent-orange);
    border-color: rgba(240, 136, 62, 0.2);
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--text-tertiary);
  }

  .status-dot.ok {
    background: var(--accent-green);
    box-shadow: var(--glow-green);
  }

  .status-dot.degraded {
    background: var(--accent-orange);
    animation: blink 1s ease-in-out infinite;
  }

  .status-dot.checking {
    background: var(--text-tertiary);
    animation: blink 0.5s ease-in-out infinite;
  }

  .status-dot.unknown {
    background: var(--text-tertiary);
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.3; }
  }

  .notification-control {
    position: relative;
  }

  .notification-btn {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all 0.15s ease;
    position: relative;
  }

  .notification-btn:hover {
    border-color: var(--border-default);
    color: var(--text-secondary);
  }

  .notification-btn.enabled {
    color: var(--accent-cyan);
    border-color: rgba(0, 229, 255, 0.3);
    background: rgba(0, 229, 255, 0.1);
  }

  .notification-btn.blocked {
    color: var(--text-tertiary);
    opacity: 0.5;
    cursor: not-allowed;
  }

  .notification-btn svg {
    width: 18px;
    height: 18px;
  }

  .notification-badge {
    position: absolute;
    top: 6px;
    right: 6px;
    width: 8px;
    height: 8px;
    background: var(--accent-green);
    border-radius: 50%;
    box-shadow: var(--glow-green);
  }
</style>
