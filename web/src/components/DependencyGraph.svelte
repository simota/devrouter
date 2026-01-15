<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import type { ServiceDependency, DependencyNode } from '../api/types';
  import { loadDependencyImpact } from '../lib/dependencyImpactService';
  import {
    buildNodeIndex,
    computeImpacted,
    computeRootCandidates,
    getHealthStatus,
    summarizeHealth,
    type HealthStatus,
  } from '../lib/dependencyImpact';

  export let stackId: string;

  const dispatch = createEventDispatcher<{ close: void; selectService: string }>();

  let loading = true;
  let error: string | null = null;
  let nodes: DependencyNode[] = [];
  let edges: ServiceDependency[] = [];
  let selectedNode: string | null = null;
  let selectedDetails: DependencyNode | null = null;
  let warnings: string[] = [];
  let healthMap = new Map<string, HealthStatus>();
  let impactMode = true;
  let selectedRoot: string | null = null;
  let rootCandidates: string[] = [];
  let impactedSet = new Set<string>();
  let impactedList: string[] = [];
  let healthSummary = { healthy: 0, unhealthy: 0, unknown: 0 };
  let hasUserSelectedRoot = false;
  let nodeIndex = new Map<string, DependencyNode>();

  // Layout settings
  const NODE_WIDTH = 120;
  const NODE_HEIGHT = 40;
  const HORIZONTAL_SPACING = 180;
  const VERTICAL_SPACING = 80;
  const GRAPH_PADDING = 60; // Extra padding for scrollable area
  const EDGE_STROKE_BASE = 1.5;
  const EDGE_STROKE_HIGHLIGHT = 2.2;

  interface PositionedNode extends DependencyNode {
    x: number;
    y: number;
    level: number;
  }

  let positionedNodes: PositionedNode[] = [];
  let svgWidth = 600;
  let svgHeight = 400;

  onMount(() => {
    let active = true;

    const load = async () => {
      loading = true;
      error = null;
      warnings = [];
      const result = await loadDependencyImpact(stackId);
      if (!active) return;

      if (!result.ok) {
        error = result.error.message;
        loading = false;
        return;
      }

      nodes = result.value.nodes;
      edges = result.value.edges;
      healthMap = result.value.healthMap;
      warnings = result.value.warnings;
      selectedNode = null;
      selectedRoot = null;
      hasUserSelectedRoot = false;
      loading = false;
    };

    void load();

    return () => {
      active = false;
    };
  });

  function calculateLayout() {
    if (nodes.length === 0) return;

    // Calculate levels using topological sort
    const levels = new Map<string, number>();
    const visited = new Set<string>();

    function calculateLevel(nodeName: string): number {
      if (levels.has(nodeName)) return levels.get(nodeName)!;
      if (visited.has(nodeName)) return 0; // Circular dependency

      visited.add(nodeName);
      const node = nodes.find(n => n.name === nodeName);
      if (!node || node.dependencies.length === 0) {
        levels.set(nodeName, 0);
        return 0;
      }

      let maxDepLevel = 0;
      for (const dep of node.dependencies) {
        maxDepLevel = Math.max(maxDepLevel, calculateLevel(dep) + 1);
      }
      levels.set(nodeName, maxDepLevel);
      return maxDepLevel;
    }

    // Calculate level for each node
    for (const node of nodes) {
      calculateLevel(node.name);
    }

    // Group nodes by level
    const levelGroups = new Map<number, string[]>();
    for (const [name, level] of levels) {
      if (!levelGroups.has(level)) {
        levelGroups.set(level, []);
      }
      levelGroups.get(level)!.push(name);
    }

    // Position nodes
    const maxLevel = Math.max(...levels.values());
    positionedNodes = [];

    // First pass: calculate required width
    let maxNodesAtAnyLevel = 0;
    for (let level = 0; level <= maxLevel; level++) {
      const nodesAtLevel = levelGroups.get(level) || [];
      maxNodesAtAnyLevel = Math.max(maxNodesAtAnyLevel, nodesAtLevel.length);
    }
    const requiredWidth = maxNodesAtAnyLevel * HORIZONTAL_SPACING + GRAPH_PADDING * 2;
    const baseWidth = Math.max(600, requiredWidth);

    for (let level = 0; level <= maxLevel; level++) {
      const nodesAtLevel = levelGroups.get(level) || [];
      const levelWidth = nodesAtLevel.length * HORIZONTAL_SPACING;
      const startX = Math.max(GRAPH_PADDING + NODE_WIDTH / 2, (baseWidth - levelWidth) / 2 + HORIZONTAL_SPACING / 2);

      nodesAtLevel.forEach((name, idx) => {
        const node = nodes.find(n => n.name === name)!;
        positionedNodes.push({
          ...node,
          x: startX + idx * HORIZONTAL_SPACING,
          y: GRAPH_PADDING + level * VERTICAL_SPACING,
          level,
        });
      });
    }

    // Adjust SVG dimensions with padding
    if (positionedNodes.length > 0) {
      const maxX = Math.max(...positionedNodes.map(n => n.x)) + NODE_WIDTH / 2 + GRAPH_PADDING;
      const maxY = Math.max(...positionedNodes.map(n => n.y)) + NODE_HEIGHT + GRAPH_PADDING;
      svgWidth = Math.max(600, maxX);
      svgHeight = Math.max(300, maxY);
    }
  }

  $: if (nodes.length > 0) {
    calculateLayout();
  } else {
    positionedNodes = [];
    svgWidth = 600;
    svgHeight = 400;
  }

  $: nodeIndex = buildNodeIndex(nodes);
  $: rootCandidates = computeRootCandidates(nodes, healthMap);
  $: healthSummary = summarizeHealth(nodes, healthMap);
  $: if (!impactMode) {
    selectedRoot = null;
    hasUserSelectedRoot = false;
  }
  $: if (impactMode && !selectedRoot && rootCandidates.length > 0 && !hasUserSelectedRoot) {
    selectedRoot = rootCandidates[0];
    selectedNode = rootCandidates[0];
  }
  $: impactedSet = impactMode ? computeImpacted(selectedRoot, nodeIndex) : new Set<string>();
  $: impactedList = Array.from(impactedSet).filter((name) => name !== selectedRoot);
  $: selectedDetails = selectedNode ? nodes.find((node) => node.name === selectedNode) ?? null : null;

  function getNodePosition(name: string): { x: number; y: number } | null {
    const node = positionedNodes.find(n => n.name === name);
    return node ? { x: node.x, y: node.y } : null;
  }

  function getEdgePath(from: string, to: string): string {
    const fromPos = getNodePosition(from);
    const toPos = getNodePosition(to);
    if (!fromPos || !toPos) return '';

    const startX = fromPos.x;
    const startY = fromPos.y + NODE_HEIGHT;
    const endX = toPos.x;
    const endY = toPos.y;

    // Curved path
    const midY = (startY + endY) / 2;
    return `M ${startX} ${startY} C ${startX} ${midY}, ${endX} ${midY}, ${endX} ${endY}`;
  }

  function getEdgeColor(type: ServiceDependency['type']): string {
    switch (type) {
      case 'depends_on': return 'var(--accent-cyan)';
      case 'link': return 'var(--accent-green)';
      case 'url': return 'var(--accent-orange)';
      case 'hostname': return 'var(--accent-orange)';
      default: return 'var(--text-tertiary)';
    }
  }

  function handleNodeClick(name: string) {
    selectedNode = selectedNode === name ? null : name;
    if (impactMode) {
      selectedRoot = name;
      hasUserSelectedRoot = true;
    }
  }

  function handleRootSelect(name: string) {
    impactMode = true;
    selectedRoot = name;
    selectedNode = name;
    hasUserSelectedRoot = true;
  }

  function clearSelection() {
    selectedNode = null;
    selectedRoot = null;
    hasUserSelectedRoot = true;
  }

  function getNodeStatus(name: string): HealthStatus {
    return getHealthStatus(healthMap, name);
  }

  function getNodeClass(node: PositionedNode): string {
    const classes = ['node', `status-${getNodeStatus(node.name)}`];
    if (selectedNode === node.name) classes.push('selected');
    if (impactMode && impactedSet.has(node.name)) classes.push('impacted');
    if (impactMode && rootCandidates.includes(node.name)) classes.push('root-cause');
    if (selectedNode && node.dependencies.includes(selectedNode)) classes.push('dependency');
    if (selectedNode && node.dependents.includes(selectedNode)) classes.push('dependent');
    return classes.join(' ');
  }

  function isEdgeHighlighted(edge: ServiceDependency): boolean {
    if (!selectedNode) return false;
    return edge.from === selectedNode || edge.to === selectedNode;
  }

  function isEdgeImpacted(edge: ServiceDependency): boolean {
    if (!impactMode || impactedSet.size === 0) return false;
    return impactedSet.has(edge.from) || impactedSet.has(edge.to);
  }
</script>

<div class="dependency-graph">
  <div class="graph-header">
    <div class="header-left">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="5" r="3"/>
        <line x1="12" y1="8" x2="12" y2="14"/>
        <circle cx="6" cy="19" r="3"/>
        <circle cx="18" cy="19" r="3"/>
        <line x1="12" y1="14" x2="6" y2="16"/>
        <line x1="12" y1="14" x2="18" y2="16"/>
      </svg>
      <h3>Service Dependencies</h3>
      {#if !loading && edges.length > 0}
        <span class="edge-count">{edges.length} connections</span>
      {/if}
    </div>
    <div class="header-right">
      <label class="impact-toggle">
        <input type="checkbox" bind:checked={impactMode} />
        <span>Impact mode</span>
      </label>
    <button class="close-btn" on:click={() => dispatch('close')} aria-label="Close dependency graph">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 6L6 18M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </div>

  <div class="graph-content">
    {#if loading}
      <div class="loading">
        <svg class="spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" stroke-dasharray="60" stroke-dashoffset="20"/>
        </svg>
        <span>Analyzing dependencies...</span>
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
    {:else if nodes.length === 0}
      <div class="empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <line x1="8" y1="12" x2="16" y2="12"/>
        </svg>
        <span>No services found</span>
      </div>
    {:else}
      <div class="graph-layout">
        <div class="graph-main">
          {#if edges.length === 0}
            <div class="no-deps-banner">
              <span>No dependencies detected between services</span>
              <p class="hint">Dependencies are detected from depends_on, links, and environment variables</p>
            </div>
          {/if}

          <div class="legend">
            <div class="legend-item">
              <span class="legend-line depends"></span>
              <span>depends_on</span>
            </div>
            <div class="legend-item">
              <span class="legend-line link"></span>
              <span>link</span>
            </div>
            <div class="legend-item">
              <span class="legend-line url"></span>
              <span>env URL</span>
            </div>
          </div>

          <div class="svg-container">
            <svg width={svgWidth} height={svgHeight}>
              <defs>
                <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                  <polygon points="0 0, 10 3.5, 0 7" fill="var(--text-tertiary)"/>
                </marker>
                <marker id="arrowhead-cyan" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                  <polygon points="0 0, 10 3.5, 0 7" fill="var(--accent-cyan)"/>
                </marker>
                <marker id="arrowhead-green" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                  <polygon points="0 0, 10 3.5, 0 7" fill="var(--accent-green)"/>
                </marker>
                <marker id="arrowhead-orange" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                  <polygon points="0 0, 10 3.5, 0 7" fill="var(--accent-orange)"/>
                </marker>
              </defs>

              <!-- Edges -->
              {#each edges as edge (edge.from + '-' + edge.to + '-' + edge.type)}
                <path
                  class="edge"
                  class:highlighted={isEdgeHighlighted(edge)}
                  class:impacted={isEdgeImpacted(edge)}
                  d={getEdgePath(edge.from, edge.to)}
                  stroke={getEdgeColor(edge.type)}
                  fill="none"
                  stroke-width={isEdgeHighlighted(edge) || isEdgeImpacted(edge) ? EDGE_STROKE_HIGHLIGHT : EDGE_STROKE_BASE}
                  marker-end={`url(#arrowhead-${edge.type === 'depends_on' ? 'cyan' : edge.type === 'link' ? 'green' : 'orange'})`}
                />
              {/each}

              <!-- Nodes -->
              {#each positionedNodes as node (node.name)}
                <g
                  class={getNodeClass(node)}
                  transform={`translate(${node.x - NODE_WIDTH/2}, ${node.y})`}
                  on:click={() => handleNodeClick(node.name)}
                  on:keydown={(e) => e.key === 'Enter' && handleNodeClick(node.name)}
                  role="button"
                  tabindex="0"
                  aria-label={`${node.name} (${getNodeStatus(node.name)})`}
                >
                  <rect
                    width={NODE_WIDTH}
                    height={NODE_HEIGHT}
                    rx="6"
                  />
                  <text
                    x={NODE_WIDTH / 2}
                    y={NODE_HEIGHT / 2 + 4}
                    text-anchor="middle"
                  >
                    {node.name.length > 14 ? node.name.slice(0, 12) + '...' : node.name}
                  </text>
                  <circle
                    class={`status-dot ${getNodeStatus(node.name)}`}
                    cx={NODE_WIDTH - 10}
                    cy={10}
                    r="4"
                  />
                </g>
              {/each}
            </svg>
          </div>
        </div>

        <aside class="graph-side">
          {#if warnings.length > 0}
            <div class="warning-banner">
              {#each warnings as warning}
                <span>{warning}</span>
              {/each}
            </div>
          {/if}
          <div class="side-section">
            <div class="side-header">
              <h4>Status summary</h4>
              <button class="clear-btn" on:click={clearSelection}>Clear</button>
            </div>
            <div class="status-summary">
              <span class="summary-pill healthy">Healthy {healthSummary.healthy}</span>
              <span class="summary-pill unhealthy">Unhealthy {healthSummary.unhealthy}</span>
              <span class="summary-pill unknown">Unknown {healthSummary.unknown}</span>
            </div>
          </div>

          <div class="side-section">
            <h4>Root cause candidates</h4>
            {#if rootCandidates.length === 0}
              <div class="side-empty">No candidates detected.</div>
            {:else}
              <div class="root-list">
                {#each rootCandidates as name}
                  <button
                    class="root-item"
                    class:active={name === selectedRoot}
                    on:click={() => handleRootSelect(name)}
                  >
                    <span class="root-name">{name}</span>
                    <span class={`status-badge ${getNodeStatus(name)}`}>{getNodeStatus(name)}</span>
                  </button>
                {/each}
              </div>
            {/if}
          </div>

          <div class="side-section">
            <h4>Impacted services</h4>
            {#if !impactMode}
              <div class="side-empty">Enable impact mode to see downstream impact.</div>
            {:else if !selectedRoot}
              <div class="side-empty">Select a service to compute impact.</div>
            {:else if impactedList.length === 0}
              <div class="side-empty">No dependents detected.</div>
            {:else}
              <ul class="impact-list">
                {#each impactedList as name}
                  <li class="impact-item">
                    <span>{name}</span>
                    <span class={`status-badge ${getNodeStatus(name)}`}>{getNodeStatus(name)}</span>
                  </li>
                {/each}
              </ul>
            {/if}
          </div>

          <div class="side-section">
            <h4>Selected service</h4>
            {#if !selectedDetails}
              <div class="side-empty">Select a node to see details.</div>
            {:else}
              <div class="detail-card">
                <div class="detail-header">
                  <span class="detail-name">{selectedDetails.name}</span>
                  <span class={`status-badge ${getNodeStatus(selectedDetails.name)}`}>
                    {getNodeStatus(selectedDetails.name)}
                  </span>
                </div>
                {#if selectedDetails.dependencies.length > 0}
                  <div class="detail-section">
                    <span class="detail-label">Depends on:</span>
                    <span class="detail-value">{selectedDetails.dependencies.join(', ')}</span>
                  </div>
                {/if}
                {#if selectedDetails.dependents.length > 0}
                  <div class="detail-section">
                    <span class="detail-label">Required by:</span>
                    <span class="detail-value">{selectedDetails.dependents.join(', ')}</span>
                  </div>
                {/if}
                {#if selectedDetails.dependencies.length === 0 && selectedDetails.dependents.length === 0}
                  <div class="detail-section">
                    <span class="detail-label">No direct dependencies</span>
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        </aside>
      </div>
    {/if}
  </div>
</div>

<style>
  .dependency-graph {
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 10px;
    overflow: visible;
  }

  .graph-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border-subtle);
    background: var(--bg-surface);
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 12px;
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

  .edge-count {
    font-size: 0.625rem;
    font-weight: 600;
    padding: 3px 8px;
    border-radius: 4px;
    background: rgba(0, 229, 255, 0.15);
    color: var(--accent-cyan);
  }

  .impact-toggle {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border-radius: 999px;
    border: 1px solid var(--border-subtle);
    background: var(--bg-secondary);
    color: var(--text-tertiary);
    font-size: 0.688rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .impact-toggle input {
    accent-color: var(--accent-cyan);
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

  .graph-content {
    padding: 16px;
    min-height: 200px;
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

  .hint {
    font-size: 0.75rem;
    color: var(--text-tertiary);
    margin: 0;
  }

  .graph-layout {
    display: grid;
    grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
    gap: 16px;
    align-items: start;
  }

  .graph-main {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .graph-side {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 14px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
  }

  .warning-banner {
    padding: 10px 12px;
    border-radius: 8px;
    border: 1px solid rgba(240, 136, 62, 0.4);
    background: rgba(240, 136, 62, 0.12);
    color: var(--accent-orange);
    font-size: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .no-deps-banner {
    padding: 12px 14px;
    border-radius: 8px;
    background: rgba(240, 136, 62, 0.1);
    border: 1px dashed rgba(240, 136, 62, 0.4);
    color: var(--text-secondary);
    font-size: 0.75rem;
  }

  .legend {
    display: flex;
    gap: 16px;
    margin-bottom: 16px;
    padding: 8px 12px;
    background: var(--bg-surface);
    border-radius: 6px;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.688rem;
    color: var(--text-secondary);
  }

  .legend-line {
    width: 20px;
    height: 2px;
    border-radius: 1px;
  }

  .legend-line.depends { background: var(--accent-cyan); }
  .legend-line.link { background: var(--accent-green); }
  .legend-line.url { background: var(--accent-orange); }

  .svg-container {
    overflow: auto;
    background: var(--bg-surface);
    border-radius: 8px;
    border: 1px solid var(--border-subtle);
  }

  .edge {
    opacity: 0.6;
    transition: opacity 0.2s ease;
  }

  .edge.highlighted,
  .edge.impacted {
    opacity: 1;
  }

  .node rect {
    fill: var(--bg-elevated);
    stroke: var(--border-default);
    stroke-width: 1.5;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .node:hover rect {
    fill: var(--bg-hover);
    stroke: var(--accent-cyan);
  }

  .node.status-healthy rect {
    stroke: var(--accent-green);
  }

  .node.status-unhealthy rect {
    stroke: var(--accent-red);
  }

  .node.status-unknown rect {
    stroke: var(--accent-orange);
  }

  .node.impacted rect {
    fill: rgba(0, 229, 255, 0.08);
  }

  .node.root-cause rect {
    stroke-width: 2.5;
    filter: drop-shadow(0 0 6px rgba(248, 81, 73, 0.35));
  }

  .node.selected rect {
    fill: rgba(0, 229, 255, 0.1);
    stroke: var(--accent-cyan);
    stroke-width: 2;
  }

  .node.dependency rect {
    fill: rgba(210, 153, 34, 0.1);
    stroke: var(--accent-orange);
  }

  .node.dependent rect {
    fill: rgba(63, 185, 80, 0.1);
    stroke: var(--accent-green);
  }

  .node text {
    fill: var(--text-primary);
    font-size: 11px;
    font-family: 'IBM Plex Mono', monospace;
    pointer-events: none;
  }

  .status-dot {
    fill: var(--text-tertiary);
  }

  .status-dot.healthy {
    fill: var(--accent-green);
  }

  .status-dot.unhealthy {
    fill: var(--accent-red);
  }

  .status-dot.unknown {
    fill: var(--accent-orange);
  }

  .side-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .side-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .side-section h4 {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--text-tertiary);
    margin: 0;
  }

  .clear-btn {
    background: transparent;
    border: 1px solid var(--border-default);
    color: var(--text-secondary);
    font-size: 0.65rem;
    padding: 4px 8px;
    border-radius: 6px;
    cursor: pointer;
  }

  .status-summary {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .summary-pill {
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    padding: 4px 8px;
    border-radius: 999px;
    border: 1px solid transparent;
  }

  .summary-pill.healthy {
    color: var(--accent-green);
    border-color: rgba(57, 211, 83, 0.4);
    background: rgba(57, 211, 83, 0.12);
  }

  .summary-pill.unhealthy {
    color: var(--accent-red);
    border-color: rgba(248, 81, 73, 0.4);
    background: rgba(248, 81, 73, 0.12);
  }

  .summary-pill.unknown {
    color: var(--accent-orange);
    border-color: rgba(240, 136, 62, 0.4);
    background: rgba(240, 136, 62, 0.12);
  }

  .root-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .root-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 10px;
    border-radius: 8px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    color: var(--text-primary);
    cursor: pointer;
    transition: transform 0.2s ease-out, border-color 0.2s ease-out, background-color 0.2s ease-out;
    transform: translateZ(0);
  }

  .root-item:hover {
    transform: translateY(-2px);
  }

  .root-item:active {
    transform: translateY(0);
  }

  .root-item.active {
    border-color: var(--accent-cyan);
    box-shadow: 0 0 0 1px rgba(0, 229, 255, 0.2);
  }

  .root-name {
    font-size: 0.75rem;
    font-family: 'IBM Plex Mono', monospace;
  }

  .status-badge {
    font-size: 0.6rem;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    padding: 3px 6px;
    border-radius: 999px;
    border: 1px solid transparent;
  }

  .status-badge.healthy {
    color: var(--accent-green);
    border-color: rgba(57, 211, 83, 0.4);
    background: rgba(57, 211, 83, 0.1);
  }

  .status-badge.unhealthy {
    color: var(--accent-red);
    border-color: rgba(248, 81, 73, 0.4);
    background: rgba(248, 81, 73, 0.12);
  }

  .status-badge.unknown {
    color: var(--accent-orange);
    border-color: rgba(240, 136, 62, 0.4);
    background: rgba(240, 136, 62, 0.12);
  }

  .impact-list {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 0;
    margin: 0;
  }

  .impact-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 10px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    font-size: 0.75rem;
  }

  .detail-card {
    padding: 10px 12px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .detail-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .detail-name {
    font-size: 0.85rem;
    font-family: 'IBM Plex Mono', monospace;
    color: var(--text-primary);
  }

  .side-empty {
    padding: 10px 12px;
    border-radius: 8px;
    border: 1px dashed var(--border-default);
    color: var(--text-tertiary);
    font-size: 0.75rem;
    background: var(--bg-secondary);
  }

  .detail-section {
    display: flex;
    gap: 8px;
    font-size: 0.75rem;
    margin-bottom: 6px;
  }

  .detail-section:last-child {
    margin-bottom: 0;
  }

  .detail-label {
    color: var(--text-tertiary);
  }

  .detail-value {
    color: var(--text-secondary);
    font-family: 'IBM Plex Mono', monospace;
  }

  @media (max-width: 900px) {
    .graph-layout {
      grid-template-columns: 1fr;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .root-item {
      transition: none;
      transform: none;
    }
  }
</style>
