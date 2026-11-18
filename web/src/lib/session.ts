import { writable } from 'svelte/store';

export type SessionState = 'unknown' | 'signed-in' | 'suspended' | 'signed-out';

export const session = writable<SessionState>('unknown');

export async function loadSession(): Promise<SessionState> {
  try {
    const response = await fetch('/api/v1/auth/session', { credentials: 'include' });
    if (response.status === 204) {
      session.set('signed-in');
      return 'signed-in';
    }
    if (response.status === 403) {
      session.set('suspended');
      return 'suspended';
    }
    session.set('signed-out');
    return 'signed-out';
  } catch {
    session.set('signed-out');
    return 'signed-out';
  }
}
