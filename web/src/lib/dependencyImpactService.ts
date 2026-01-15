import { getStackDependencies, getStackHealth } from '../api/client';
import type { DependencyNode, ServiceDependency } from '../api/types';
import {
  buildDependencyGraph,
  buildHealthMap,
  isEdgeType,
  isHealthStatus,
  type HealthStatus,
  type ServiceHealthSnapshot,
} from './dependencyImpact';

type Result<T> = { ok: true; value: T } | { ok: false; error: DependencyImpactError };

export type DependencyImpactError = {
  kind: 'network' | 'validation';
  message: string;
};

export interface DependencyImpactData {
  nodes: DependencyNode[];
  edges: ServiceDependency[];
  healthMap: Map<string, HealthStatus>;
  warnings: string[];
}

export async function loadDependencyImpact(stackId: string): Promise<Result<DependencyImpactData>> {
  try {
    const [depsResult, healthResult] = await Promise.allSettled([
      getStackDependencies(stackId),
      getStackHealth(stackId),
    ]);

    if (depsResult.status === 'rejected') {
      return {
        ok: false,
        error: { kind: 'network', message: toErrorMessage(depsResult.reason, 'Failed to load dependencies') },
      };
    }

    const depsParse = parseDependenciesResponse(depsResult.value);
    if (!depsParse.ok) return depsParse;

    const warnings: string[] = [];
    let healthSnapshots: ServiceHealthSnapshot[] = [];

    if (healthResult.status === 'rejected') {
      warnings.push('Health data unavailable; showing unknown status.');
    } else {
      const healthParse = parseHealthResponse(healthResult.value);
      if (!healthParse.ok) {
        warnings.push('Health data invalid; showing unknown status.');
      } else {
        healthSnapshots = healthParse.value;
      }
    }

    const nodes = buildDependencyGraph(depsParse.value.serviceNames, depsParse.value.edges);
    const healthMap = buildHealthMap(healthSnapshots);

    return { ok: true, value: { nodes, edges: depsParse.value.edges, healthMap, warnings } };
  } catch (error) {
    return { ok: false, error: { kind: 'network', message: toErrorMessage(error, 'Failed to load dependency data') } };
  }
}

function parseDependenciesResponse(raw: unknown): Result<{ edges: ServiceDependency[]; serviceNames: string[] }> {
  if (!isRecord(raw)) {
    return validationError('Dependency response is not an object');
  }

  const edgesValue = raw.edges;
  if (!Array.isArray(edgesValue)) {
    return validationError('Dependency response is missing edges');
  }

  const edges: ServiceDependency[] = [];
  for (const entry of edgesValue) {
    if (!isRecord(entry)) {
      return validationError('Dependency edge is not an object');
    }

    const from = entry.from;
    const to = entry.to;
    const type = entry.type;

    if (!isNonEmptyString(from) || !isNonEmptyString(to) || !isEdgeType(type)) {
      return validationError('Dependency edge has invalid fields');
    }

    const parsed: ServiceDependency = { from, to, type };

    if (entry.envVar !== undefined) {
      if (!isNonEmptyString(entry.envVar)) {
        return validationError('Dependency edge envVar must be a string');
      }
      parsed.envVar = entry.envVar;
    }

    if (entry.url !== undefined) {
      if (!isNonEmptyString(entry.url)) {
        return validationError('Dependency edge url must be a string');
      }
      parsed.url = entry.url;
    }

    edges.push(parsed);
  }

  const serviceNames = parseStringArray(raw.serviceNames);
  return { ok: true, value: { edges, serviceNames } };
}

function parseHealthResponse(raw: unknown): Result<ServiceHealthSnapshot[]> {
  if (!isRecord(raw)) {
    return validationError('Health response is not an object');
  }

  const servicesValue = raw.services;
  if (!Array.isArray(servicesValue)) {
    return validationError('Health response is missing services');
  }

  const services: ServiceHealthSnapshot[] = [];
  for (const entry of servicesValue) {
    if (!isRecord(entry)) {
      return validationError('Health service entry is not an object');
    }

    const name = entry.name;
    const statusValue = entry.status;

    if (!isNonEmptyString(name) || typeof statusValue !== 'string') {
      return validationError('Health service entry has invalid fields');
    }

    const status: HealthStatus = isHealthStatus(statusValue) ? statusValue : 'unknown';
    services.push({ name, status });
  }

  return { ok: true, value: services };
}

function parseStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  const result: string[] = [];
  for (const item of value) {
    if (isNonEmptyString(item)) {
      result.push(item);
    }
  }
  return result;
}

function validationError(message: string): Result<never> {
  return { ok: false, error: { kind: 'validation', message } };
}

function toErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (typeof error === 'string') return error;
  return fallback;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0;
}
