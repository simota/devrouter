import { getStackEnv } from '../api/client';
import type { EnvVarItem } from '../api/types';

type Result<T> = { ok: true; value: T } | { ok: false; error: EnvLoadError };

export type EnvLoadError = {
  kind: 'network' | 'validation';
  message: string;
};

export async function loadStackEnv(stackId: string): Promise<Result<EnvVarItem[]>> {
  try {
    const raw = await getStackEnv(stackId);
    const parsed = parseEnvResponse(raw);
    if (!parsed.ok) return parsed;
    return { ok: true, value: parsed.value };
  } catch (error) {
    return { ok: false, error: { kind: 'network', message: toErrorMessage(error, 'Failed to load environment variables') } };
  }
}

function parseEnvResponse(raw: unknown): Result<EnvVarItem[]> {
  if (!isRecord(raw)) {
    return validationError('Environment response is not an object');
  }

  const envValue = raw.env;
  if (!Array.isArray(envValue)) {
    return validationError('Environment response is missing env');
  }

  const items: EnvVarItem[] = [];
  for (const entry of envValue) {
    if (!isRecord(entry)) {
      return validationError('Environment entry is not an object');
    }

    const service = entry.service;
    const name = entry.name;
    const value = entry.value;

    if (!isNonEmptyString(service) || !isNonEmptyString(name) || typeof value !== 'string') {
      return validationError('Environment entry has invalid fields');
    }

    items.push({ service, name, value });
  }

  return { ok: true, value: items };
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
