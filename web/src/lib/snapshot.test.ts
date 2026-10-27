import { describe, expect, it } from 'vitest';
import type { ServiceDependency } from '../api/types';
import { buildSnapshot, type SnapshotBase } from './snapshot';

const envSecret = 'ENV_VALUE_SENTINEL_73c1';
const configSecret = 'CONFIG_ONLY_SENTINEL_a29e';
const dependencySecret = 'DEPENDENCY_ONLY_SENTINEL_f604';

function createBase(): SnapshotBase {
  const stackId = 'review-stack';
  const createdAt = '2026-10-28T00:00:00.000Z';
  return {
    meta: {
      createdAt,
      stackId,
      repoPath: '/projects/review',
      domain: 'localtest.me',
      source: 'compose',
    },
    stack: {
      id: stackId,
      name: 'review',
      repoPath: '/projects/review',
      source: 'compose',
      domain: 'localtest.me',
      composeFilePath: '/projects/review/compose.yaml',
      lastUpAt: createdAt,
      services: [{
        name: 'api',
        workspacePath: 'apps/api',
        host: 'review-api.localtest.me',
        url: 'http://review-api.localtest.me',
        port: 3000,
        composeService: 'review-api',
      }],
    },
    health: {
      stackId,
      services: [{
        name: 'api',
        status: 'healthy',
        containerState: 'running',
        url: 'http://review-api.localtest.me/health',
        latency: 5,
        checkedAt: createdAt,
      }],
      checkedAt: createdAt,
    },
    stats: {
      stackId,
      containers: [{
        name: 'review-api-1',
        service: 'api',
        cpuPercent: 2,
        memoryUsage: 1024,
        memoryLimit: 4096,
        memPercent: 25,
        netIO: '2kB / 1kB',
        blockIO: '0B / 0B',
        pids: 3,
      }],
      collectedAt: createdAt,
    },
    dependencies: {
      stackId,
      nodes: [
        { name: 'api', dependencies: ['db'], dependents: [] },
        { name: 'db', dependencies: [], dependents: ['api'] },
      ],
      edges: [{
        from: 'api',
        to: 'db',
        envVar: 'DATABASE_URL',
        url: `postgres://user:${dependencySecret}@db:5432/app`,
        type: 'url',
      }],
      serviceNames: ['api', 'db'],
    },
    env: {
      stackId,
      env: [{ service: 'api', name: 'API_TOKEN', value: envSecret }],
    },
    config: {
      path: '/projects/review/compose.yaml',
      content: `services:\n  api:\n    environment:\n      PASSWORD: ${configSecret}\n`,
      type: 'compose',
    },
    watch: {
      enabled: true,
      watching: true,
      repoPath: '/projects/review',
      watchedPaths: 3,
      recentEvents: [{
        path: 'apps/api/index.ts',
        action: 'modified',
        timestamp: createdAt,
        matchedRule: '*.ts',
        targetService: 'api',
      }],
      rules: [{ pattern: '*.ts', action: 'restart' }],
    },
  };
}

function freezeDeep<T>(value: T): T {
  if (value && typeof value === 'object') {
    Object.freeze(value);
    for (const child of Object.values(value)) freezeDeep(child);
  }
  return value;
}

describe('snapshot export redaction', () => {
  it('removes independent environment secrets from the entire serialized export', () => {
    const snapshot = buildSnapshot(createBase(), true);
    const serialized = JSON.stringify(snapshot, null, 2);

    for (const secret of [envSecret, configSecret, dependencySecret]) {
      expect(serialized).not.toContain(secret);
    }
    expect(snapshot.envRedacted).toBe(true);
    expect(snapshot.env.env[0].value).toBe('******');
    expect(snapshot.config.content).toBe('******');
    expect(snapshot.dependencies.edges[0].url).toBe('******');
  });

  it('protects configuration and dependency values when the environment request failed', () => {
    const base = createBase();
    // SnapshotPanel uses this fallback when the environment API rejects.
    base.env = { stackId: base.stack.id, env: [] };

    const snapshot = buildSnapshot(base, true);
    const serialized = JSON.stringify(snapshot);

    expect(serialized).not.toContain(configSecret);
    expect(serialized).not.toContain(dependencySecret);
    expect(snapshot.env).toEqual(base.env);
    expect(snapshot.config.content).toBe('******');
    expect(snapshot.dependencies.edges[0].url).toBe('******');
  });

  it.each(['compose', 'devrouter'] as const)(
    'withholds the full %s configuration without relying on a known environment-value list',
    (type) => {
      const base = createBase();
      base.env.env = [];
      base.dependencies.edges = [];
      base.config.type = type;
      base.config.content = 'services:\n  api:\n    command: [node, server.js, --password, CONFIG_COMMAND_SENTINEL]\n';

      const snapshot = buildSnapshot(base, true);

      expect(JSON.stringify(snapshot)).not.toContain('CONFIG_COMMAND_SENTINEL');
      expect(snapshot.config).toEqual({ ...base.config, content: '******' });
    },
  );

  it.each([
    { name: 'quotes and backslashes', value: 'VALUE_SENTINEL_"quoted"\\path\'single\'' },
    { name: 'multiple lines', value: 'VALUE_SENTINEL_line1\nline2\r\nline3' },
    { name: 'one character', value: 'x' },
    { name: 'whitespace only', value: ' \n\t' },
  ])('redacts $name without parsing or matching the value', ({ value }) => {
    const base = createBase();
    base.env.env[0].value = value;
    base.config.content = value;
    base.dependencies.edges[0].url = value;

    const snapshot = JSON.parse(JSON.stringify(buildSnapshot(base, true)));

    expect(snapshot.env.env[0].value).toBe('******');
    expect(snapshot.config.content).toBe('******');
    expect(snapshot.dependencies.edges[0].url).toBe('******');
  });

  it.each(['url', 'hostname', 'depends_on', 'link'] as const)(
    'redacts an environment-derived value on a %s edge while preserving diagnostic metadata',
    (type: ServiceDependency['type']) => {
      const base = createBase();
      base.env.env = [];
      const edge = {
        from: 'api', to: 'db', envVar: 'DB_HOST', url: dependencySecret, type,
      };
      base.dependencies.edges = [edge];

      const snapshot = buildSnapshot(base, true);

      expect(JSON.stringify(snapshot)).not.toContain(dependencySecret);
      expect(snapshot.dependencies.edges).toEqual([{ ...edge, url: '******' }]);
      expect(snapshot.dependencies.nodes).toEqual(base.dependencies.nodes);
      expect(snapshot.dependencies.serviceNames).toEqual(base.dependencies.serviceNames);
    },
  );

  it('preserves empty values and dependency edges with no URL', () => {
    const base = createBase();
    base.env.env = [{ service: 'api', name: 'OPTIONAL_VALUE', value: '' }];
    base.config.content = '';
    base.dependencies.edges = [
      { from: 'api', to: 'db', type: 'url', envVar: 'EMPTY_URL', url: '' },
      { from: 'api', to: 'db', type: 'depends_on' },
      { from: 'api', to: 'db', type: 'link', url: undefined },
    ];

    const snapshot = buildSnapshot(base, true);

    expect(snapshot.env).toEqual(base.env);
    expect(snapshot.config).toEqual(base.config);
    expect(snapshot.dependencies).toEqual(base.dependencies);
    expect(JSON.stringify(snapshot.dependencies)).toBe(JSON.stringify(base.dependencies));
  });

  it.each([
    { name: 'null environment list', nullEnv: true, nullEdges: false },
    { name: 'null dependency edges', nullEnv: false, nullEdges: true },
    { name: 'both lists null', nullEnv: true, nullEdges: true },
  ])('accepts a successful API response with $name', ({ nullEnv, nullEdges }) => {
    const base = createBase();
    // Go encodes nil slices as null. The current client types only advertise
    // arrays, so construct the actual wire shape without changing that contract.
    const apiBase = JSON.parse(JSON.stringify({
      ...base,
      env: { ...base.env, env: nullEnv ? null : base.env.env },
      dependencies: {
        ...base.dependencies,
        edges: nullEdges ? null : base.dependencies.edges,
      },
    })) as SnapshotBase;

    const snapshot = buildSnapshot(apiBase, true);
    const serialized = JSON.stringify(snapshot);

    for (const secret of [envSecret, configSecret, dependencySecret]) {
      expect(serialized).not.toContain(secret);
    }
    expect(snapshot.env.env).toEqual(nullEnv ? null : [
      { service: 'api', name: 'API_TOKEN', value: '******' },
    ]);
    expect(snapshot.dependencies.edges).toEqual(nullEdges ? null : [
      { ...base.dependencies.edges[0], url: '******' },
    ]);
    expect(snapshot.config.content).toBe('******');
    expect(buildSnapshot(apiBase, false)).toEqual({ ...apiBase, envRedacted: false });
  });

  it('retains useful diagnostics independently of withheld values', () => {
    const base = createBase();
    const snapshot = buildSnapshot(base, true);

    expect(snapshot.meta).toEqual(base.meta);
    expect(snapshot.stack).toEqual(base.stack);
    expect(snapshot.health).toEqual(base.health);
    expect(snapshot.stats).toEqual(base.stats);
    expect(snapshot.watch).toEqual(base.watch);
    expect(snapshot.env.stackId).toBe(base.env.stackId);
    expect(snapshot.env.env[0]).toEqual({ service: 'api', name: 'API_TOKEN', value: '******' });
    expect(snapshot.config.path).toBe(base.config.path);
    expect(snapshot.config.type).toBe(base.config.type);
    expect(snapshot.dependencies.stackId).toBe(base.dependencies.stackId);
    expect(snapshot.dependencies.nodes).toEqual(base.dependencies.nodes);
    expect(snapshot.dependencies.serviceNames).toEqual(base.dependencies.serviceNames);
  });

  it('keeps the original API data in an explicitly unredacted export', () => {
    const base = createBase();
    // A server-redacted value cannot and should not be reconstructed by export.
    base.env.env.push({ service: 'api', name: 'SERVER_REDACTED', value: '[REDACTED]' });

    const snapshot = buildSnapshot(base, false);

    expect(snapshot).toEqual({ ...base, envRedacted: false });
    const serialized = JSON.stringify(snapshot);
    for (const secret of [envSecret, configSecret, dependencySecret]) {
      expect(serialized).toContain(secret);
    }
    expect(snapshot.env.env[1].value).toBe('[REDACTED]');

    const redacted = JSON.stringify(buildSnapshot(base, true));
    for (const secret of [envSecret, configSecret, dependencySecret]) {
      expect(redacted).not.toContain(secret);
    }
  });

  it('can toggle redacted to raw to redacted without changing the fetched snapshot', () => {
    const base = freezeDeep(createBase());
    const before = JSON.stringify(base);

    const first = JSON.stringify(buildSnapshot(base, true));
    const raw = buildSnapshot(base, false);
    const last = JSON.stringify(buildSnapshot(base, true));

    expect(first).toBe(last);
    expect(raw).toEqual({ ...base, envRedacted: false });
    expect(JSON.stringify(base)).toBe(before);
    for (const secret of [envSecret, configSecret, dependencySecret]) {
      expect(last).not.toContain(secret);
    }
  });

  it('exports partial-result fallbacks without replacing unavailable content with a mask', () => {
    const base = createBase();
    const stackId = base.stack.id;
    const createdAt = base.meta.createdAt;
    base.health = { stackId, services: [], checkedAt: createdAt };
    base.stats = { stackId, containers: [], collectedAt: createdAt };
    base.dependencies = { stackId, nodes: [], edges: [], serviceNames: ['api'] };
    base.env = { stackId, env: [] };
    base.config = { path: base.stack.composeFilePath, content: '', type: 'compose' };
    base.watch = {
      enabled: false, watching: false, repoPath: base.stack.repoPath,
      watchedPaths: 0, recentEvents: [], rules: [],
    };

    expect(buildSnapshot(base, true)).toEqual({ ...base, envRedacted: true });
    expect(buildSnapshot(base, false)).toEqual({ ...base, envRedacted: false });
  });
});
