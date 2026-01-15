import type { StackListResponse, StackResponse, HealthResponse, StackHealthResponse, ConfigFileResponse, InitConfigRequest, InitConfigResponse, EnvResponse, StackStatsResponse, StackDependenciesResponse, TemplateListResponse, ServiceTemplate, HistoryEventsResponse, HistoryStatsResponse, StackInsights } from './types';

const API_BASE = '/api';

export async function getStacks(): Promise<StackListResponse> {
  const res = await fetch(`${API_BASE}/stacks`);
  if (!res.ok) throw new Error('Failed to fetch stacks');
  return res.json();
}

export async function getStack(id: string): Promise<StackResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error('Failed to fetch stack');
  return res.json();
}

export async function stopStack(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to stop stack');
}

export async function getHealth(): Promise<HealthResponse> {
  const res = await fetch(`${API_BASE}/health`);
  if (!res.ok) throw new Error('Failed to fetch health');
  return res.json();
}

export async function getStackHealth(stackId: string): Promise<StackHealthResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/health`);
  if (!res.ok) throw new Error('Failed to fetch stack health');
  return res.json();
}

export async function getStackConfig(stackId: string): Promise<ConfigFileResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/config`);
  if (!res.ok) throw new Error('Failed to fetch stack config');
  return res.json();
}

export async function previewInitConfig(stackId: string, payload: InitConfigRequest): Promise<InitConfigResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/init/preview`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const errorBody = await res.json().catch(() => null) as { message?: string } | null;
    throw new Error(errorBody?.message || 'Failed to load init preview');
  }
  return res.json();
}

export async function applyInitConfig(stackId: string, payload: InitConfigRequest): Promise<InitConfigResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/init/apply`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const errorBody = await res.json().catch(() => null) as { message?: string } | null;
    throw new Error(errorBody?.message || 'Failed to apply init config');
  }
  return res.json();
}

export function getServiceUrl(stackId: string, serviceName: string): string {
  return `${API_BASE}/stacks/${encodeURIComponent(stackId)}/services/${encodeURIComponent(serviceName)}/url`;
}

export async function getStackEnv(stackId: string): Promise<EnvResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/env`);
  if (!res.ok) throw new Error('Failed to fetch stack env');
  return res.json();
}

export async function restartStack(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(id)}/restart`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to restart stack');
}

export async function restartService(stackId: string, serviceName: string): Promise<void> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/services/${encodeURIComponent(serviceName)}/restart`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to restart service');
}

export async function buildRestartStack(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(id)}/build-restart`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to build and restart stack');
}

export async function buildRestartService(stackId: string, serviceName: string): Promise<void> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/services/${encodeURIComponent(serviceName)}/build-restart`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to build and restart service');
}

export async function getStackStats(stackId: string): Promise<StackStatsResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/stats`);
  if (!res.ok) throw new Error('Failed to fetch stack stats');
  return res.json();
}

export function createStatsWebSocket(stackId: string): WebSocket {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return new WebSocket(`${protocol}//${window.location.host}${API_BASE}/ws/stats/${encodeURIComponent(stackId)}`);
}

export async function getStackDependencies(stackId: string): Promise<StackDependenciesResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/dependencies`);
  if (!res.ok) throw new Error('Failed to fetch stack dependencies');
  return res.json();
}

export async function getTemplates(): Promise<TemplateListResponse> {
  const res = await fetch(`${API_BASE}/templates`);
  if (!res.ok) throw new Error('Failed to fetch templates');
  return res.json();
}

export async function getTemplate(id: string): Promise<ServiceTemplate> {
  const res = await fetch(`${API_BASE}/templates/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error('Failed to fetch template');
  return res.json();
}

export async function getHistoryEvents(stackId: string, since?: string): Promise<HistoryEventsResponse> {
  const params = since ? `?since=${encodeURIComponent(since)}` : '';
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/history/events${params}`);
  if (!res.ok) throw new Error('Failed to fetch history events');
  return res.json();
}

export async function getHistoryStats(stackId: string, range: '1h' | '6h' | '24h' = '1h'): Promise<HistoryStatsResponse> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/history/stats?range=${range}`);
  if (!res.ok) throw new Error('Failed to fetch history stats');
  return res.json();
}

export async function getStackInsights(stackId: string): Promise<StackInsights> {
  const res = await fetch(`${API_BASE}/stacks/${encodeURIComponent(stackId)}/insights`);
  if (!res.ok) throw new Error('Failed to fetch stack insights');
  return res.json();
}
