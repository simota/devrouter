import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { connectLogs, disconnectLogs, isConnected } from './websocket';

class FakeWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: FakeWebSocket[] = [];

  readyState = FakeWebSocket.CONNECTING;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;
  close = vi.fn(() => {
    this.readyState = FakeWebSocket.CLOSING;
  });

  constructor(readonly url: string) {
    FakeWebSocket.instances.push(this);
  }

  open() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
  }
}

describe('log connection lifetime', () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.stubGlobal('WebSocket', FakeWebSocket);
    vi.stubGlobal('location', { protocol: 'http:', host: 'localhost:19847' });
  });

  afterEach(() => {
    disconnectLogs();
    vi.unstubAllGlobals();
  });

  it('ignores queued events from a connection replaced by another stack', () => {
    const oldMessage = vi.fn();
    const oldStatus = vi.fn();
    connectLogs('first', undefined, oldMessage, oldStatus);
    const first = FakeWebSocket.instances[0];
    first.open();

    // Keep the callbacks to model events already queued before replacement.
    const lateOpen = first.onopen!;
    const lateMessage = first.onmessage!;
    const lateError = first.onerror!;
    const lateClose = first.onclose!;

    const newMessage = vi.fn();
    const newStatus = vi.fn();
    connectLogs('second', undefined, newMessage, newStatus);
    const second = FakeWebSocket.instances[1];
    second.open();
    const statusBeforeOldEvents = newStatus.mock.calls.slice();

    lateOpen();
    lateMessage({ data: JSON.stringify({ type: 'log', content: 'first stack' }) });
    lateError();
    lateClose();

    expect(first.close).toHaveBeenCalledOnce();
    expect(oldMessage).not.toHaveBeenCalled();
    expect(newMessage).not.toHaveBeenCalled();
    expect(newStatus.mock.calls).toEqual(statusBeforeOldEvents);
    expect(isConnected()).toBe(true);

    const currentMessage = { type: 'log', content: 'second stack' };
    second.onmessage?.({ data: JSON.stringify(currentMessage) });
    expect(newMessage).toHaveBeenCalledExactlyOnceWith(currentMessage);

    disconnectLogs();
    expect(second.close).toHaveBeenCalledOnce();
  });

  it('reports disconnected immediately and ignores events after explicit cleanup', () => {
    const onMessage = vi.fn();
    const onStatus = vi.fn();
    connectLogs('stack', undefined, onMessage, onStatus);
    const socket = FakeWebSocket.instances[0];
    socket.open();
    expect(onStatus).toHaveBeenLastCalledWith(true);
    const lateMessage = socket.onmessage!;
    const lateClose = socket.onclose!;

    disconnectLogs();
    expect(onStatus).toHaveBeenLastCalledWith(false);
    expect(isConnected()).toBe(false);
    const callsAfterDisconnect = onStatus.mock.calls.length;

    lateMessage({ data: 'late log' });
    lateClose();
    disconnectLogs();
    expect(onMessage).not.toHaveBeenCalled();
    expect(onStatus).toHaveBeenCalledTimes(callsAfterDisconnect);
    expect(socket.close).toHaveBeenCalledOnce();
  });

  it('reports connecting, open, error and peer close for the active connection', () => {
    const onStatus = vi.fn();
    connectLogs('stack', undefined, undefined, onStatus);
    const socket = FakeWebSocket.instances[0];
    expect(onStatus).toHaveBeenLastCalledWith(false);
    expect(isConnected()).toBe(false);
    socket.open();
    expect(onStatus).toHaveBeenLastCalledWith(true);
    expect(isConnected()).toBe(true);
    socket.onerror?.();
    expect(onStatus).toHaveBeenLastCalledWith(false);
    socket.readyState = FakeWebSocket.CLOSED;
    socket.onclose?.();
    expect(onStatus).toHaveBeenLastCalledWith(false);
    expect(isConnected()).toBe(false);
    disconnectLogs();
    expect(socket.close).not.toHaveBeenCalled();
  });

  it('preserves service URL encoding, TLS selection and plain text fallback', () => {
    vi.stubGlobal('location', { protocol: 'https:', host: 'devrouter.local:19847' });
    const onMessage = vi.fn();
    connectLogs('stack/name', 'api worker', onMessage);
    const socket = FakeWebSocket.instances[0];
    expect(socket.url).toBe('wss://devrouter.local:19847/api/ws/logs/stack%2Fname/api%20worker');
    socket.open();
    socket.onmessage?.({ data: 'plain text' });
    expect(onMessage).toHaveBeenCalledExactlyOnceWith({
      type: 'log',
      timestamp: expect.any(String),
      content: 'plain text',
    });
  });
});
