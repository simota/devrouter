import type { EnvVarItem } from '../api/types';

export const ENV_ROW_STATUSES = ['consistent', 'inconsistent', 'missing'] as const;
export type EnvRowStatus = typeof ENV_ROW_STATUSES[number];

export interface EnvConsistencyRow {
  name: string;
  valuesByService: Record<string, string | null>;
  distinctValues: string[];
  hasMissing: boolean;
  hasInconsistent: boolean;
}

export function deriveEnvServices(envItems: EnvVarItem[]): string[] {
  return Array.from(new Set(envItems.map((item) => item.service))).sort();
}

export function buildEnvConsistencyRows(envItems: EnvVarItem[], services: string[]): EnvConsistencyRow[] {
  const serviceList = services.length > 0 ? services : deriveEnvServices(envItems);
  const variableMap = new Map<string, Map<string, string[]>>();

  for (const item of envItems) {
    const serviceMap = getOrCreateServiceMap(variableMap, item.name);
    const values = serviceMap.get(item.service) ?? [];
    values.push(item.value);
    serviceMap.set(item.service, values);
  }

  const rows: EnvConsistencyRow[] = [];

  for (const [name, serviceMap] of variableMap) {
    const valuesByService: Record<string, string | null> = {};
    const distinctValues = new Set<string>();
    let hasMissing = false;
    let hasInconsistent = false;

    for (const service of serviceList) {
      const values = serviceMap.get(service);
      if (!values || values.length === 0) {
        valuesByService[service] = null;
        hasMissing = true;
        continue;
      }

      const uniqueValues = Array.from(new Set(values));
      const value = uniqueValues[uniqueValues.length - 1];
      valuesByService[service] = value;
      distinctValues.add(value);

      if (uniqueValues.length > 1) {
        hasInconsistent = true;
      }
    }

    if (distinctValues.size > 1) {
      hasInconsistent = true;
    }

    rows.push({
      name,
      valuesByService,
      distinctValues: Array.from(distinctValues),
      hasMissing,
      hasInconsistent,
    });
  }

  return rows.sort((a, b) => a.name.localeCompare(b.name));
}

export function getEnvRowStatus(row: EnvConsistencyRow): EnvRowStatus {
  if (row.hasInconsistent) return 'inconsistent';
  if (row.hasMissing) return 'missing';
  return 'consistent';
}

export function formatEnvValue(value: string | null): string {
  if (value === null) return '(missing)';
  if (value === '') return '(empty)';
  return value;
}

function getOrCreateServiceMap(
  variableMap: Map<string, Map<string, string[]>>,
  name: string,
): Map<string, string[]> {
  const existing = variableMap.get(name);
  if (existing) return existing;
  const next = new Map<string, string[]>();
  variableMap.set(name, next);
  return next;
}
