import type { DependencyNode, ServiceDependency } from '../api/types';

export const HEALTH_STATUSES = ['healthy', 'unhealthy', 'unknown'] as const;
export type HealthStatus = typeof HEALTH_STATUSES[number];

export const EDGE_TYPES = ['depends_on', 'link', 'url', 'hostname'] as const;
export type EdgeType = typeof EDGE_TYPES[number];

export interface ServiceHealthSnapshot {
  name: string;
  status: HealthStatus;
}

export function isHealthStatus(value: unknown): value is HealthStatus {
  return typeof value === 'string' && (HEALTH_STATUSES as readonly string[]).includes(value);
}

export function isEdgeType(value: unknown): value is EdgeType {
  return typeof value === 'string' && (EDGE_TYPES as readonly string[]).includes(value);
}

export function deriveServiceNamesFromEdges(edges: ServiceDependency[]): string[] {
  const names = new Set<string>();
  for (const edge of edges) {
    names.add(edge.from);
    names.add(edge.to);
  }
  return Array.from(names).sort();
}

export function buildDependencyGraph(serviceNames: string[], edges: ServiceDependency[]): DependencyNode[] {
  const nodeMap = new Map<string, DependencyNode>();
  const names = serviceNames.length > 0 ? serviceNames : deriveServiceNamesFromEdges(edges);

  for (const name of names) {
    nodeMap.set(name, { name, dependencies: [], dependents: [] });
  }

  for (const edge of edges) {
    const fromNode = getOrCreateNode(nodeMap, edge.from);
    const toNode = getOrCreateNode(nodeMap, edge.to);

    pushUnique(fromNode.dependencies, edge.to);
    pushUnique(toNode.dependents, edge.from);
  }

  return Array.from(nodeMap.values()).sort((a, b) => a.name.localeCompare(b.name));
}

export function buildNodeIndex(nodes: DependencyNode[]): Map<string, DependencyNode> {
  return new Map(nodes.map((node) => [node.name, node]));
}

export function buildHealthMap(services: ServiceHealthSnapshot[]): Map<string, HealthStatus> {
  const map = new Map<string, HealthStatus>();
  for (const service of services) {
    map.set(service.name, service.status);
  }
  return map;
}

export function getHealthStatus(map: Map<string, HealthStatus>, name: string): HealthStatus {
  return map.get(name) ?? 'unknown';
}

export function computeRootCandidates(nodes: DependencyNode[], healthMap: Map<string, HealthStatus>): string[] {
  const candidates: string[] = [];

  for (const node of nodes) {
    if (getHealthStatus(healthMap, node.name) !== 'unhealthy') continue;

    const depsHealthy = node.dependencies.every((dep) => {
      const status = getHealthStatus(healthMap, dep);
      return status === 'healthy' || status === 'unknown';
    });

    if (depsHealthy) {
      candidates.push(node.name);
    }
  }

  return candidates.sort();
}

export function computeImpacted(root: string | null, nodeIndex: Map<string, DependencyNode>): Set<string> {
  const impacted = new Set<string>();
  if (!root) return impacted;

  const queue: string[] = [root];
  while (queue.length > 0) {
    const current = queue.shift();
    if (!current || impacted.has(current)) continue;

    impacted.add(current);
    const node = nodeIndex.get(current);
    if (!node) continue;

    for (const dependent of node.dependents) {
      if (!impacted.has(dependent)) {
        queue.push(dependent);
      }
    }
  }

  return impacted;
}

export function summarizeHealth(nodes: DependencyNode[], healthMap: Map<string, HealthStatus>): {
  healthy: number;
  unhealthy: number;
  unknown: number;
} {
  let healthy = 0;
  let unhealthy = 0;
  let unknown = 0;

  for (const node of nodes) {
    const status = getHealthStatus(healthMap, node.name);
    if (status === 'healthy') healthy += 1;
    else if (status === 'unhealthy') unhealthy += 1;
    else unknown += 1;
  }

  return { healthy, unhealthy, unknown };
}

function getOrCreateNode(map: Map<string, DependencyNode>, name: string): DependencyNode {
  const existing = map.get(name);
  if (existing) return existing;
  const node = { name, dependencies: [], dependents: [] };
  map.set(name, node);
  return node;
}

function pushUnique(list: string[], value: string): void {
  if (!list.includes(value)) {
    list.push(value);
  }
}
