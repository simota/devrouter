import { describe, it, expect } from 'vitest';
import type { DependencyNode } from '../api/types';
import { computeRootCandidates, type HealthStatus } from './dependencyImpact';

describe('computeRootCandidates', () => {
  it('GIVEN unknown dependencies WHEN service is unhealthy THEN it is a root candidate', () => {
    const nodes: DependencyNode[] = [
      { name: 'api', dependencies: ['db'], dependents: [] },
      { name: 'db', dependencies: [], dependents: ['api'] },
    ];
    const healthMap = new Map<string, HealthStatus>([
      ['api', 'unhealthy'],
      ['db', 'unknown'],
    ]);

    expect(computeRootCandidates(nodes, healthMap)).toEqual(['api']);
  });
});
