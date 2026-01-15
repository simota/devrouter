<script lang="ts">
  type WatchRule = {
    pattern: string;
    action: string;
  };

  type Service = {
    name: string;
    workspacePath: string;
  };

  type WatchEvent = {
    id: number;
    path: string;
    rule: string;
    action: string;
    targetService: string;
    timestamp: string;
  };

  const mockServices: Service[] = [
    { name: 'web', workspacePath: 'apps/web' },
    { name: 'admin', workspacePath: 'apps/admin' },
    { name: 'api', workspacePath: 'services/api' },
    { name: 'worker', workspacePath: 'services/worker' },
    { name: 'shared', workspacePath: 'packages/shared' },
  ];

  const mockRules: WatchRule[] = [
    { pattern: '*.go', action: 'restart' },
    { pattern: '*.ts', action: 'restart' },
    { pattern: '*.tsx', action: 'restart' },
    { pattern: '*.js', action: 'restart' },
    { pattern: '*.jsx', action: 'restart' },
  ];

  const samplePaths = [
    'apps/web/src/pages/index.tsx',
    'apps/admin/src/app.ts',
    'services/api/src/handlers/user.go',
    'services/worker/src/jobs/cleanup.ts',
    'packages/shared/src/date.ts',
    'docs/README.md',
  ];

  let pathInput = samplePaths[0];
  let nextId = 1;

  function matchRule(path: string): WatchRule | null {
    const lower = path.toLowerCase();
    for (const rule of mockRules) {
      const suffix = rule.pattern.replace('*', '').toLowerCase();
      if (lower.endsWith(suffix)) {
        return rule;
      }
    }
    return null;
  }

  function matchService(path: string): Service | null {
    const normalized = path.replace(/^\.\//, '');
    let matched: Service | null = null;
    for (const service of mockServices) {
      if (normalized.startsWith(service.workspacePath)) {
        if (!matched || service.workspacePath.length > matched.workspacePath.length) {
          matched = service;
        }
      }
    }
    return matched;
  }

  function buildEvent(path: string): WatchEvent {
    const rule = matchRule(path);
    const service = matchService(path);
    return {
      id: nextId++,
      path,
      rule: rule ? rule.pattern : 'none',
      action: rule ? rule.action : 'ignore',
      targetService: service ? service.name : 'unmapped',
      timestamp: new Date().toISOString(),
    };
  }

  let events: WatchEvent[] = [
    buildEvent('apps/web/src/pages/index.tsx'),
    buildEvent('packages/shared/src/date.ts'),
    buildEvent('docs/README.md'),
  ];

  function addEvent(path: string) {
    const trimmed = path.trim();
    if (!trimmed) return;
    events = [buildEvent(trimmed), ...events].slice(0, 8);
  }

  function simulateChange() {
    addEvent(pathInput);
  }

  function addRandomExample() {
    const randomPath = samplePaths[Math.floor(Math.random() * samplePaths.length)];
    pathInput = randomPath;
    addEvent(randomPath);
  }

  function formatTime(iso: string): string {
    const date = new Date(iso);
    return date.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  $: previewRule = matchRule(pathInput);
  $: previewService = matchService(pathInput);
  $: unmappedCount = events.filter((event) => event.targetService === 'unmapped').length;
</script>

<div class="prototype-shell">
  <header class="prototype-header">
    <div>
      <p class="prototype-label">prototype</p>
      <h1>Service-aware watch restart</h1>
      <p class="prototype-subtitle">
        Mock UI to preview how file changes map to a target service.
      </p>
    </div>
    <div class="prototype-note">mock data only</div>
  </header>

  <section class="panel">
    <h2>Service map</h2>
    <div class="service-grid">
      {#each mockServices as service}
        <div class="service-card">
          <div class="service-name">{service.name}</div>
          <div class="service-path">{service.workspacePath}</div>
        </div>
      {/each}
    </div>
  </section>

  <section class="panel">
    <h2>Simulate change</h2>
    <div class="controls">
      <input
        class="path-input"
        type="text"
        bind:value={pathInput}
        placeholder="apps/web/src/pages/index.tsx"
      />
      <button class="primary" on:click={simulateChange}>Simulate</button>
      <button class="secondary" on:click={addRandomExample}>Random example</button>
    </div>
    <div class="preview">
      <div>
        <span class="preview-label">rule</span>
        <span class="preview-value">{previewRule ? previewRule.pattern : 'none'}</span>
      </div>
      <div>
        <span class="preview-label">target</span>
        <span class="preview-value">
          {previewService ? previewService.name : 'unmapped'}
        </span>
      </div>
    </div>
  </section>

  <section class="panel">
    <div class="panel-header">
      <h2>Recent events</h2>
      <div class="panel-stats">
        <span>total {events.length}</span>
        <span>unmapped {unmappedCount}</span>
      </div>
    </div>
    <div class="event-table">
      <div class="event-row event-header">
        <span>path</span>
        <span>rule</span>
        <span>target</span>
        <span>time</span>
      </div>
      {#each events as event (event.id)}
        <div class="event-row">
          <span class="event-path">{event.path}</span>
          <span class="event-rule">{event.rule}</span>
          <span class="event-target" class:unmapped={event.targetService === 'unmapped'}>
            {event.targetService}
          </span>
          <span class="event-time">{formatTime(event.timestamp)}</span>
        </div>
      {/each}
    </div>
  </section>
</div>

<style>
  .prototype-shell {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding: 28px 32px 40px;
    overflow-y: auto;
  }

  .prototype-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 24px;
  }

  .prototype-label {
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.2em;
    color: var(--text-tertiary);
    margin-bottom: 8px;
  }

  .prototype-header h1 {
    font-family: 'Space Grotesk', sans-serif;
    font-size: 1.6rem;
    margin: 0 0 6px 0;
  }

  .prototype-subtitle {
    color: var(--text-secondary);
    font-size: 0.85rem;
    max-width: 560px;
    line-height: 1.5;
  }

  .prototype-note {
    padding: 6px 10px;
    border-radius: 6px;
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-tertiary);
  }

  .panel {
    background: var(--bg-secondary);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    padding: 18px 20px;
  }

  .panel h2 {
    margin: 0 0 12px 0;
    font-size: 0.9rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-secondary);
  }

  .service-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 12px;
  }

  .service-card {
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    padding: 12px 14px;
    background: var(--bg-surface);
  }

  .service-name {
    font-weight: 600;
    margin-bottom: 6px;
  }

  .service-path {
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.75rem;
    color: var(--text-tertiary);
  }

  .controls {
    display: grid;
    grid-template-columns: 1fr auto auto;
    gap: 10px;
    align-items: center;
  }

  .path-input {
    width: 100%;
    padding: 10px 12px;
    border-radius: 8px;
    border: 1px solid var(--border-default);
    background: var(--bg-surface);
    color: var(--text-primary);
    font-family: 'IBM Plex Mono', monospace;
    font-size: 0.8rem;
  }

  button {
    padding: 10px 14px;
    border-radius: 8px;
    border: 1px solid transparent;
    font-size: 0.8rem;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  button.primary {
    background: rgba(0, 229, 255, 0.15);
    border-color: rgba(0, 229, 255, 0.35);
    color: var(--accent-cyan);
  }

  button.primary:hover {
    background: rgba(0, 229, 255, 0.25);
  }

  button.secondary {
    background: rgba(240, 136, 62, 0.15);
    border-color: rgba(240, 136, 62, 0.35);
    color: var(--accent-orange);
  }

  button.secondary:hover {
    background: rgba(240, 136, 62, 0.25);
  }

  .preview {
    display: flex;
    gap: 18px;
    margin-top: 12px;
    font-size: 0.8rem;
  }

  .preview-label {
    text-transform: uppercase;
    letter-spacing: 0.1em;
    font-size: 0.65rem;
    color: var(--text-tertiary);
    margin-right: 6px;
  }

  .preview-value {
    font-family: 'IBM Plex Mono', monospace;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }

  .panel-stats {
    display: flex;
    gap: 12px;
    font-size: 0.75rem;
    color: var(--text-tertiary);
  }

  .event-table {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .event-row {
    display: grid;
    grid-template-columns: 1.6fr 0.6fr 0.6fr 0.4fr;
    gap: 10px;
    align-items: center;
    padding: 10px 12px;
    background: var(--bg-surface);
    border-radius: 8px;
    font-size: 0.75rem;
  }

  .event-header {
    background: transparent;
    border-bottom: 1px solid var(--border-subtle);
    border-radius: 0;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-tertiary);
  }

  .event-path {
    font-family: 'IBM Plex Mono', monospace;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .event-rule,
  .event-time {
    color: var(--text-secondary);
  }

  .event-target {
    font-weight: 600;
    color: var(--accent-green);
  }

  .event-target.unmapped {
    color: var(--accent-red);
  }

  @media (max-width: 860px) {
    .controls {
      grid-template-columns: 1fr;
    }

    .event-row {
      grid-template-columns: 1fr;
    }
  }
</style>
