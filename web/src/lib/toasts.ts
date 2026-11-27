import { writable } from 'svelte/store';

export const TOAST_DURATION_MS = 4000;
export const MAX_TOASTS = 3;

export type Toast = {
  id: number;
  message: string;
  announce: boolean;
};

const { subscribe, update } = writable<Toast[]>([]);

export const toasts = { subscribe };

let nextId = 1;

export function showToast(message: string, options: { announce?: boolean } = {}): number {
  const id = nextId++;
  const toast: Toast = { id, message, announce: options.announce ?? true };
  update((list) => [...list, toast].slice(-MAX_TOASTS));
  return id;
}

export function dismissToast(id: number): void {
  update((list) => list.filter((toast) => toast.id !== id));
}

export function clearToasts(): void {
  update(() => []);
}
