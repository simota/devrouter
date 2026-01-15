import type { LogMessage } from './types';

let ws: WebSocket | null = null;
let onMessageCallback: ((msg: LogMessage) => void) | null = null;
let onStatusCallback: ((connected: boolean) => void) | null = null;

export function connectLogs(
  stackId: string,
  serviceName?: string,
  onMessage?: (msg: LogMessage) => void,
  onStatus?: (connected: boolean) => void
): void {
  disconnectLogs();

  onMessageCallback = onMessage || null;
  onStatusCallback = onStatus || null;

  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  let url = `${protocol}//${location.host}/api/ws/logs/${encodeURIComponent(stackId)}`;
  if (serviceName) {
    url += `/${encodeURIComponent(serviceName)}`;
  }

  ws = new WebSocket(url);

  ws.onopen = () => {
    onStatusCallback?.(true);
  };

  ws.onmessage = (event) => {
    try {
      const msg: LogMessage = JSON.parse(event.data);
      onMessageCallback?.(msg);
    } catch {
      onMessageCallback?.({
        type: 'log',
        timestamp: new Date().toISOString(),
        content: event.data,
      });
    }
  };

  ws.onerror = () => {
    onStatusCallback?.(false);
  };

  ws.onclose = () => {
    onStatusCallback?.(false);
    ws = null;
  };
}

export function disconnectLogs(): void {
  if (ws) {
    ws.close();
    ws = null;
  }
  onMessageCallback = null;
  onStatusCallback = null;
}

export function isConnected(): boolean {
  return ws !== null && ws.readyState === WebSocket.OPEN;
}
