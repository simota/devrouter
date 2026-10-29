import { describe, expect, it } from 'vitest';
import ServiceRow from './ServiceRow.svelte';
import type { ServiceHealthStatus, ServiceResponse } from '../api/types';

// Svelte 4 compiles components for SSR under vitest's node environment,
// which exposes a static render() that returns the markup.
const render = (props: { service: ServiceResponse; health: ServiceHealthStatus }) =>
  (ServiceRow as unknown as { render: (p: typeof props) => { html: string } }).render(props).html;

const health = (overrides: Partial<ServiceHealthStatus>): ServiceHealthStatus => ({
  name: 'svc',
  status: 'healthy',
  containerState: 'running',
  url: '',
  latency: 0,
  checkedAt: '2026-10-29T00:00:00Z',
  ...overrides,
});

describe('ServiceRow', () => {
  it('renders a worker without links, open or copy, and with a worker badge', () => {
    const html = render({
      service: { name: 'bff-worker', workspacePath: 'bff-worker', host: '', url: '', port: 0, composeService: 'bff-worker', kind: 'worker' },
      health: health({ name: 'bff-worker' }),
    });

    expect(html).toContain('kind-badge');
    expect(html).toMatch(/health-badge healthy[^>]*>\s*healthy/);
    expect(html).not.toContain('<a');
    expect(html).not.toContain('open-btn');
    expect(html).not.toContain('copy-btn');
    expect(html).not.toContain('service-url');
  });

  it('shows the container state of a stopped worker', () => {
    const html = render({
      service: { name: 'bff-worker', workspacePath: 'bff-worker', host: '', url: '', port: 0, composeService: 'bff-worker', kind: 'worker' },
      health: health({ name: 'bff-worker', status: 'unknown', containerState: 'exited' }),
    });

    expect(html).toMatch(/container-badge stopped[^>]*>\s*Exited/);
  });

  it('keeps the URL, open and copy controls for an http service', () => {
    const html = render({
      service: { name: 'api', workspacePath: 'api', host: 'api.localtest.me', url: 'https://api.localtest.me', port: 8080, composeService: 'api', healthCheck: { endpoint: '/health' } },
      health: health({ name: 'api', url: 'https://api.localtest.me/health' }),
    });

    expect(html).toContain('href="https://api.localtest.me/health"');
    expect(html).toContain('api.localtest.me');
    expect(html).toContain('open-btn');
    expect(html).toContain('copy-btn');
    expect(html).not.toContain('kind-badge');
  });
});
