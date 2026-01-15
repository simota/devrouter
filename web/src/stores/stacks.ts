import { writable, derived } from 'svelte/store';
import type { StackSummary, StackResponse } from '../api/types';
import { getStacks, getStack } from '../api/client';

export const stacks = writable<StackSummary[]>([]);
export const selectedStackId = writable<string | null>(null);
export const selectedStack = writable<StackResponse | null>(null);
export const loading = writable<boolean>(false);
export const error = writable<string | null>(null);

export async function fetchStacks(): Promise<void> {
  loading.set(true);
  error.set(null);
  try {
    const res = await getStacks();
    stacks.set(res.stacks);
  } catch (e) {
    error.set(e instanceof Error ? e.message : 'Unknown error');
  } finally {
    loading.set(false);
  }
}

export async function selectStack(stackId: string): Promise<void> {
  selectedStackId.set(stackId);
  loading.set(true);
  error.set(null);
  try {
    const stack = await getStack(stackId);
    selectedStack.set(stack);
  } catch (e) {
    error.set(e instanceof Error ? e.message : 'Unknown error');
    selectedStack.set(null);
  } finally {
    loading.set(false);
  }
}

export function clearSelection(): void {
  selectedStackId.set(null);
  selectedStack.set(null);
}
