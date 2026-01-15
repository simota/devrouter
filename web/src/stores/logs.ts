import { writable } from 'svelte/store';
import type { LogMessage } from '../api/types';

const MAX_LOG_ENTRIES = 1000;

export const logEntries = writable<LogMessage[]>([]);
export const logConnected = writable<boolean>(false);
export const logFollow = writable<boolean>(true);

export function appendLog(entry: LogMessage): void {
  logEntries.update((logs) => {
    const updated = [...logs, entry];
    if (updated.length > MAX_LOG_ENTRIES) {
      return updated.slice(-MAX_LOG_ENTRIES);
    }
    return updated;
  });
}

export function clearLogs(): void {
  logEntries.set([]);
}

export function setLogConnected(connected: boolean): void {
  logConnected.set(connected);
}
