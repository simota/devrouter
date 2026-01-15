<script lang="ts">
  import { onMount } from 'svelte';
  import Header from './components/Header.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import StackCard from './components/StackCard.svelte';
  import LogViewer from './components/LogViewer.svelte';
  import WatchRestartPrototype from './prototypes/WatchRestartPrototype.svelte';
  import { stacks, selectedStack, fetchStacks, selectStack, clearSelection } from './stores/stacks';
  import { logEntries, clearLogs, appendLog, setLogConnected, logConnected } from './stores/logs';
  import { connectLogs, disconnectLogs } from './api/websocket';
  import type { LogMessage } from './api/types';

  let logPanelExpanded = false;
  let prototypeMode = false;

  const getPrototypeMode = () => {
    if (typeof window === 'undefined') return false;
    return new URLSearchParams(window.location.search).get('prototype') === 'watch-restart';
  };

  onMount(() => {
    prototypeMode = getPrototypeMode();
    if (prototypeMode) return;
    fetchStacks();
    const interval = setInterval(fetchStacks, 10000);
    return () => clearInterval(interval);
  });

  function handleStackSelect(event: CustomEvent<string>) {
    const stackId = event.detail;
    selectStack(stackId);

    disconnectLogs();
    clearLogs();

    connectLogs(
      stackId,
      undefined,
      (msg: LogMessage) => appendLog(msg),
      (connected: boolean) => setLogConnected(connected)
    );

    logPanelExpanded = true;
  }

  function handleLogsToggle() {
    logPanelExpanded = !logPanelExpanded;
  }

  function handleClearSelection() {
    clearSelection();
    disconnectLogs();
    clearLogs();
  }
</script>

<div class="app">
  {#if prototypeMode}
    <WatchRestartPrototype />
  {:else}
    <Header />

    <div class="main-layout">
      <Sidebar
        stacks={$stacks}
        selectedStackId={$selectedStack?.id || null}
        on:select={handleStackSelect}
      />

      <main class="content">
        {#if $selectedStack}
          <StackCard
            stack={$selectedStack}
            on:close={handleClearSelection}
          />
        {:else}
          <div class="empty-state">
            <div class="empty-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M5 12h14M12 5l7 7-7 7"/>
              </svg>
            </div>
            <h2>Select a Stack</h2>
            <p>Choose a stack from the sidebar to view its services and logs</p>
          </div>
        {/if}
      </main>
    </div>

    <LogViewer
      logs={$logEntries}
      expanded={logPanelExpanded}
      connected={$logConnected}
      on:toggle={handleLogsToggle}
      on:clear={() => clearLogs()}
    />
  {/if}
</div>

<style>
  @import url('https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600&family=Space+Grotesk:wght@400;500;600;700&display=swap');

  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(body) {
    font-family: 'IBM Plex Mono', monospace;
    background: var(--bg-primary);
    color: var(--text-primary);
    overflow: hidden;
  }

  :global(:focus-visible) {
    outline: 2px solid var(--accent-cyan);
    outline-offset: 2px;
    box-shadow: var(--glow-cyan);
  }

  :global(:root) {
    --bg-primary: #0a0e14;
    --bg-secondary: #0f1419;
    --bg-surface: #151b23;
    --bg-elevated: #1c242e;
    --bg-hover: #242d3a;

    --border-subtle: #21262d;
    --border-default: #30363d;
    --border-emphasis: #484f58;

    --text-primary: #e6edf3;
    --text-secondary: #8b949e;
    --text-tertiary: #6e7681;

    --accent-cyan: #00e5ff;
    --accent-green: #39d353;
    --accent-orange: #f0883e;
    --accent-red: #f85149;
    --accent-purple: #a371f7;

    --glow-cyan: 0 0 20px rgba(0, 229, 255, 0.3);
    --glow-green: 0 0 15px rgba(57, 211, 83, 0.3);

    --sidebar-width: 280px;
    --header-height: 56px;
    --log-collapsed-height: 48px;
    --log-expanded-height: 280px;
  }

  .app {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background:
      radial-gradient(ellipse at 0% 0%, rgba(0, 229, 255, 0.03) 0%, transparent 50%),
      radial-gradient(ellipse at 100% 100%, rgba(57, 211, 83, 0.02) 0%, transparent 50%),
      var(--bg-primary);
  }

  .main-layout {
    display: flex;
    flex: 1;
    overflow: hidden;
  }

  .content {
    flex: 1;
    padding: 24px 32px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    color: var(--text-secondary);
    animation: fadeIn 0.5s ease-out;
  }

  .empty-icon {
    width: 64px;
    height: 64px;
    margin-bottom: 24px;
    color: var(--text-tertiary);
    opacity: 0.5;
  }

  .empty-icon svg {
    width: 100%;
    height: 100%;
  }

  .empty-state h2 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 1.5rem;
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 8px;
    letter-spacing: -0.02em;
  }

  .empty-state p {
    font-size: 0.875rem;
    max-width: 280px;
    line-height: 1.5;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
