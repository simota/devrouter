import type {
  StackResponse,
  StackHealthResponse,
  StackStatsResponse,
  StackDependenciesResponse,
  EnvResponse,
  ConfigFileResponse,
  WatchStatus,
} from '../api/types';

export type SnapshotMeta = {
  createdAt: string;
  stackId: string;
  repoPath: string;
  domain: string;
  source: string;
};

export type SnapshotBase = {
  meta: SnapshotMeta;
  stack: StackResponse;
  health: StackHealthResponse;
  stats: StackStatsResponse;
  dependencies: StackDependenciesResponse;
  env: EnvResponse;
  config: ConfigFileResponse;
  watch: WatchStatus;
};

export type StackSnapshot = SnapshotBase & {
  envRedacted: boolean;
};

function redactEnv(env: EnvResponse): EnvResponse {
  return {
    ...env,
    // Go encodes an empty nil slice as null, despite the client array type.
    env: env.env?.map((item) => ({
      ...item,
      value: item.value ? '******' : '',
    })) ?? env.env,
  };
}

export function buildSnapshot(base: SnapshotBase, envRedacted: boolean): StackSnapshot {
  return {
    meta: base.meta,
    envRedacted,
    stack: base.stack,
    health: base.health,
    stats: base.stats,
    dependencies: envRedacted ? {
      ...base.dependencies,
      // The URL field contains a complete environment value, including for
      // hostname edges. Keep graph metadata, but withhold the copied value.
      edges: base.dependencies.edges?.map((edge) => (
        edge.url ? { ...edge, url: '******' } : edge
      )) ?? base.dependencies.edges,
    } : base.dependencies,
    env: envRedacted ? redactEnv(base.env) : base.env,
    // The environment endpoint can fail or already redact a secret. Its values
    // cannot safely identify every copy in the raw Compose configuration.
    config: envRedacted ? {
      ...base.config,
      content: base.config.content ? '******' : '',
    } : base.config,
    watch: base.watch,
  };
}
