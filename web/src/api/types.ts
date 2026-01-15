export interface StackListResponse {
  stacks: StackSummary[];
}

export interface StackSummary {
  id: string;
  name: string;
  source: string;
  domain: string;
  serviceCount: number;
  lastUpAt: string;
}

export interface StackResponse {
  id: string;
  name: string;
  repoPath: string;
  source: string;
  domain: string;
  composeFilePath: string;
  lastUpAt: string;
  lastDownAt?: string;
  services: ServiceResponse[];
}

export interface HealthCheckConfig {
  endpoint?: string;
  method?: string;
  timeout?: number;
  status?: number[];
}

export interface ServiceResponse {
  name: string;
  workspacePath: string;
  host: string;
  url: string;
  port: number;
  composeService: string;
  healthCheck?: HealthCheckConfig;
}

export interface HealthResponse {
  status: string;
  docker: boolean;
  timestamp: string;
}

export interface LogMessage {
  type: 'log' | 'error' | 'connected' | 'disconnected' | 'warning';
  service?: string;
  timestamp: string;
  content: string;
}

export interface ServiceHealthStatus {
  name: string;
  status: 'healthy' | 'unhealthy' | 'unknown';
  containerState: 'running' | 'stopped' | 'exited' | 'not_found';
  url: string;
  latency: number;
  checkedAt: string;
}

export interface StackHealthResponse {
  stackId: string;
  services: ServiceHealthStatus[];
  checkedAt: string;
}

export interface ConfigFileResponse {
  path: string;
  content: string;
  type: 'compose' | 'devrouter';
}

export interface InitConfigRequest {
  stack?: string;
  domain?: string;
  force?: boolean;
}

export interface InitConfigResponse {
  outputPath: string;
  content: string;
  warnings: string[];
  applyAllowed: boolean;
  requiresForce: boolean;
  source: string;
  applied?: boolean;
}

export interface EnvVarItem {
  service: string;
  name: string;
  value: string;
}

export interface EnvResponse {
  stackId: string;
  env: EnvVarItem[];
}

export interface ContainerStats {
  name: string;
  service: string;
  cpuPercent: number;
  memoryUsage: number;
  memoryLimit: number;
  memPercent: number;
  netIO: string;
  blockIO: string;
  pids: number;
}

export interface StackStatsResponse {
  stackId: string;
  containers: ContainerStats[];
  collectedAt: string;
}

export interface ServiceDependency {
  from: string;
  to: string;
  envVar?: string;
  url?: string;
  type: 'depends_on' | 'link' | 'url' | 'hostname';
}

export interface DependencyNode {
  name: string;
  dependencies: string[];
  dependents: string[];
}

export interface StackDependenciesResponse {
  stackId: string;
  nodes: DependencyNode[];
  edges: ServiceDependency[];
  serviceNames: string[];
}

export interface TemplateHealth {
  test?: string[];
  interval?: string;
  timeout?: string;
  retries?: number;
}

export interface ServiceUI {
  name: string;
  image: string;
  ports?: string[];
  environment?: Record<string, string>;
}

export interface ServiceTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  image: string;
  ports?: string[];
  environment?: Record<string, string>;
  volumes?: string[];
  healthCheck?: TemplateHealth;
  ui?: ServiceUI;
}

export interface TemplateListResponse {
  templates: ServiceTemplate[];
}

export interface WatchRule {
  pattern: string;
  action: string;
}

export interface WatchEvent {
  path: string;
  action: 'created' | 'modified' | 'deleted';
  timestamp: string;
  matchedRule?: string;
  targetService?: string;
}

export interface WatchStatus {
  enabled: boolean;
  watching: boolean;
  repoPath: string;
  watchedPaths: number;
  recentEvents: WatchEvent[];
  rules: WatchRule[];
}

// History types
export interface HistoryEvent {
  ts: string;
  type: 'up' | 'down' | 'restart';
  stack: string;
  stackId?: string;
  services?: string[];
  service?: string;
  trigger?: string;
  duration?: string;
}

export interface HistoryEventsResponse {
  stackId: string;
  events: HistoryEvent[];
}

export interface StatsEntry {
  ts: string;
  service: string;
  cpu: number;
  mem: number;
  memPct: number;
  netIO?: string;
  blockIO?: string;
}

export interface HistoryStatsResponse {
  stackId: string;
  stats: StatsEntry[];
  range: string;
}

export interface StackInsights {
  stackId: string;
  uptime: string;
  uptimeSeconds: number;
  restartCount: number;
  restartsByService: Record<string, number>;
  peakMemory: number;
  peakMemoryAt?: string;
  peakMemorySvc?: string;
  avgCpu: number;
  computedAt: string;
}
