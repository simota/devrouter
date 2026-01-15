<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import type { ContainerStats, StackStatsResponse } from '../api/types';
  import { createStatsWebSocket } from '../api/client';
  import { notifyHighResourceUsage } from '../lib/notifications';

  export let stackId: string;
  export let stackName: string = '';

  const dispatch = createEventDispatcher<{ close: void }>();

  let stats: ContainerStats[] = [];
  let connected = false;
  let lastUpdate: Date | null = null;
  let ws: WebSocket | null = null;
  let history: Map<string, { cpu: number[]; mem: number[] }> = new Map();
  let alertedServices: Set<string> = new Set(); // Track services already alerted

  const MAX_HISTORY = 30; // 30 data points (1 minute at 2s intervals)
  const CPU_THRESHOLD = 90;
  const MEM_THRESHOLD = 90;

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatPercent(val: number): string {
    return val.toFixed(1) + '%';
  }

  function connect() {
    ws = createStatsWebSocket(stackId);

    ws.onopen = () => {
      connected = true;
    };

    ws.onmessage = (event) => {
      try {
        const data: StackStatsResponse = JSON.parse(event.data);
        stats = data.containers || [];
        lastUpdate = new Date();

        // Update history and check for high resource usage
        for (const container of stats) {
          let h = history.get(container.service);
          if (!h) {
            h = { cpu: [], mem: [] };
            history.set(container.service, h);
          }
          h.cpu.push(container.cpuPercent);
          h.mem.push(container.memPercent);
          if (h.cpu.length > MAX_HISTORY) h.cpu.shift();
          if (h.mem.length > MAX_HISTORY) h.mem.shift();

          // Check for high CPU usage
          const cpuKey = `cpu:${container.service}`;
          if (container.cpuPercent >= CPU_THRESHOLD && !alertedServices.has(cpuKey)) {
            notifyHighResourceUsage(stackName || stackId, container.service, 'cpu', container.cpuPercent);
            alertedServices.add(cpuKey);
          } else if (container.cpuPercent < CPU_THRESHOLD - 10) {
            alertedServices.delete(cpuKey);
          }

          // Check for high memory usage
          const memKey = `mem:${container.service}`;
          if (container.memPercent >= MEM_THRESHOLD && !alertedServices.has(memKey)) {
            notifyHighResourceUsage(stackName || stackId, container.service, 'memory', container.memPercent);
            alertedServices.add(memKey);
          } else if (container.memPercent < MEM_THRESHOLD - 10) {
            alertedServices.delete(memKey);
          }
        }
        history = history; // Trigger reactivity
      } catch (e) {
        console.error('Failed to parse stats:', e);
      }
    };

    ws.onclose = () => {
      connected = false;
    };

    ws.onerror = () => {
      connected = false;
    };
  }

  function disconnect() {
    if (ws) {
      ws.close();
      ws = null;
    }
  }

  function getSparklinePath(values: number[], width: number, height: number): string {
    if (values.length < 2) return '';
    const max = Math.max(...values, 1);
    const step = width / (MAX_HISTORY - 1);
    const points = values.map((v, i) => {
      const x = i * step;
      const y = height - (v / max) * height;
      return `${x},${y}`;
    });
    return `M${points.join(' L')}`;
  }

  function getCpuColor(percent: number): string {
    if (percent > 80) return 'var(--accent-red)';
    if (percent > 50) return 'var(--accent-orange)';
    return 'var(--accent-cyan)';
  }

  function getMemColor(percent: number): string {
    if (percent > 80) return 'var(--accent-red)';
    if (percent > 50) return 'var(--accent-orange)';
    return 'var(--accent-green)';
  }

  onMount(() => {
    connect();
  });

  onDestroy(() => {
    disconnect();
  });
</script>

<div class="resource-monitor">
  <div class="monitor-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="2" y="3" width="20" height="14" rx="2"/>
        <line x1="8" y1="21" x2="16" y2="21"/>
        <line x1="12" y1="17" x2="12" y2="21"/>
      </svg>
      <h3>Resource Monitor</h3>
      <span class="connection-status" class:connected>
        {connected ? 'Live' : 'Disconnected'}
      </span>
    </div>
    <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close resource monitor">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  {#if stats.length === 0}
    <div class="no-data">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="8" x2="12" y2="12"/>
        <line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
      <span>No running containers</span>
    </div>
  {:else}
    <div class="stats-grid">
      {#each stats as container (container.name)}
        {@const h = history.get(container.service) || { cpu: [], mem: [] }}
        <div class="container-stats">
          <div class="container-header">
            <span class="container-name">{container.service}</span>
            <span class="pids">{container.pids} PIDs</span>
          </div>

          <div class="metrics-row">
            <div class="metric">
              <div class="metric-header">
                <span class="metric-label">CPU</span>
                <span class="metric-value" style="color: {getCpuColor(container.cpuPercent)}">
                  {formatPercent(container.cpuPercent)}
                </span>
              </div>
              <div class="sparkline">
                <svg viewBox="0 0 100 24" preserveAspectRatio="none">
                  <path
                    d={getSparklinePath(h.cpu, 100, 24)}
                    fill="none"
                    stroke={getCpuColor(container.cpuPercent)}
                    stroke-width="1.5"
                  />
                </svg>
              </div>
              <div class="progress-bar">
                <div
                  class="progress-fill cpu"
                  style="width: {Math.min(container.cpuPercent, 100)}%; background: {getCpuColor(container.cpuPercent)}"
                ></div>
              </div>
            </div>

            <div class="metric">
              <div class="metric-header">
                <span class="metric-label">Memory</span>
                <span class="metric-value" style="color: {getMemColor(container.memPercent)}">
                  {formatBytes(container.memoryUsage)} / {formatBytes(container.memoryLimit)}
                </span>
              </div>
              <div class="sparkline">
                <svg viewBox="0 0 100 24" preserveAspectRatio="none">
                  <path
                    d={getSparklinePath(h.mem, 100, 24)}
                    fill="none"
                    stroke={getMemColor(container.memPercent)}
                    stroke-width="1.5"
                  />
                </svg>
              </div>
              <div class="progress-bar">
                <div
                  class="progress-fill mem"
                  style="width: {Math.min(container.memPercent, 100)}%; background: {getMemColor(container.memPercent)}"
                ></div>
              </div>
            </div>
          </div>

          <div class="io-row">
            <div class="io-item">
              <span class="io-label">NET I/O</span>
              <span class="io-value">{container.netIO}</span>
            </div>
            <div class="io-item">
              <span class="io-label">BLOCK I/O</span>
              <span class="io-value">{container.blockIO}</span>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .resource-monitor {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    overflow: visible;
  }

  .monitor-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-surface);
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

  .connection-status {
    font-size: 0.625rem;
    font-weight: 600;
    text-transform: uppercase;
    padding: 3px 8px;
    border-radius: 4px;
    background: rgba(139, 148, 158, 0.15);
    color: var(--text-tertiary);
  }

  .connection-status.connected {
    background: rgba(63, 185, 80, 0.15);
    color: var(--accent-green);
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

  .no-data {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 40px 20px;
    color: var(--text-tertiary);
    font-size: 0.875rem;
  }

  .no-data svg {
    width: 32px;
    height: 32px;
    opacity: 0.5;
  }

  .stats-grid {
    display: flex;
    flex-direction: column;
    gap: 1px;
    background: var(--border-subtle);
  }

  .container-stats {
    padding: 16px 18px;
    background: var(--bg-elevated);
  }

  .container-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 14px;
  }

  .container-name {
    font-size: 0.813rem;
    font-weight: 600;
    color: var(--text-primary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .pids {
    font-size: 0.688rem;
    color: var(--text-tertiary);
    font-family: 'IBM Plex Mono', monospace;
  }

  .metrics-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
    margin-bottom: 12px;
  }

  .metric {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .metric-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .metric-label {
    font-size: 0.625rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-tertiary);
  }

  .metric-value {
    font-size: 0.75rem;
    font-weight: 600;
    font-family: 'IBM Plex Mono', monospace;
  }

  .sparkline {
    height: 24px;
    width: 100%;
    background: var(--bg-surface);
    border-radius: 4px;
    overflow: hidden;
  }

  .sparkline svg {
    width: 100%;
    height: 100%;
  }

  .progress-bar {
    height: 4px;
    background: var(--bg-surface);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    transition: width 0.3s ease;
    border-radius: 2px;
  }

  .io-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
  }

  .io-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .io-label {
    font-size: 0.563rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-tertiary);
  }

  .io-value {
    font-size: 0.688rem;
    color: var(--text-secondary);
    font-family: 'IBM Plex Mono', monospace;
  }
</style>
