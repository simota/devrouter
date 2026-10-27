import { writable } from 'svelte/store';
import type { StackSummary, StackResponse } from '../api/types';
import { getStacks, getStack } from '../api/client';

export const stacks = writable<StackSummary[]>([]);
export const selectedStackId = writable<string | null>(null);
export const selectedStack = writable<StackResponse | null>(null);
// The list and the selected stack load independently, so their states are kept apart:
// a list poll must not clear an error from loading the selected stack.
export const stacksLoaded = writable<boolean>(false);
export const stacksError = writable<string | null>(null);
export const stackLoading = writable<boolean>(false);
export const stackError = writable<string | null>(null);

export async function fetchStacks(): Promise<void> {
  try {
    const res = await getStacks();
    stacks.set(res.stacks);
    stacksLoaded.set(true);
    stacksError.set(null);
  } catch (e) {
    stacksError.set(e instanceof Error ? e.message : 'Unknown error');
  }
}

export async function selectStack(stackId: string): Promise<void> {
  selectedStackId.set(stackId);
  stackLoading.set(true);
  stackError.set(null);
  try {
    const stack = await getStack(stackId);
    selectedStack.set(stack);
  } catch (e) {
    stackError.set(e instanceof Error ? e.message : 'Unknown error');
    selectedStack.set(null);
  } finally {
    stackLoading.set(false);
  }
}

export function clearSelection(): void {
  selectedStackId.set(null);
  selectedStack.set(null);
  stackError.set(null);
}
