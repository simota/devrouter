import type { LogMessage } from './types';

interface LogConnection {
  socket: WebSocket;
  onStatus?: (connected: boolean) => void;
}

let connection: LogConnection | null = null;

export function connectLogs(
  stackId: string,
  serviceName?: string,
  onMessage?: (msg: LogMessage) => void,
  onStatus?: (connected: boolean) => void
): void {
  disconnectLogs();

  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  let url = `${protocol}//${location.host}/api/ws/logs/${encodeURIComponent(stackId)}`;
  if (serviceName) {
    url += `/${encodeURIComponent(serviceName)}`;
  }

  const current: LogConnection = { socket: new WebSocket(url), onStatus };
  const socket = current.socket;
  connection = current;

  socket.onopen = () => {
    if (connection !== current) return;
    onStatus?.(true);
  };

  socket.onmessage = (event) => {
    if (connection !== current) return;
    try {
      const msg: LogMessage = JSON.parse(event.data);
      onMessage?.(msg);
    } catch {
      onMessage?.({
        type: 'log',
        timestamp: new Date().toISOString(),
        content: event.data,
      });
    }
  };

  socket.onerror = () => {
    if (connection !== current) return;
    onStatus?.(false);
  };

  socket.onclose = () => {
    if (connection !== current) return;
    connection = null;
    onStatus?.(false);
  };

  onStatus?.(false);
}

export function disconnectLogs(): void {
  const current = connection;
  connection = null;
  if (!current) return;

  // Detach the old callbacks before close, which completes asynchronously.
  current.socket.onopen = null;
  current.socket.onmessage = null;
  current.socket.onerror = null;
  current.socket.onclose = null;
  current.socket.close();
  current.onStatus?.(false);
}

export function isConnected(): boolean {
  return connection !== null && connection.socket.readyState === WebSocket.OPEN;
}
