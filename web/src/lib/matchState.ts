import { writable } from 'svelte/store';
import type { Match } from '#lib/api';

export const unrevealedMatches = writable(0);

export function setMatches(matches: Match[]): void {
  unrevealedMatches.set(matches.filter((match) => !match.revealed).length);
}

export function noteMatchCreated(): void {
  unrevealedMatches.update((count) => count + 1);
}

export function noteMatchRevealed(): void {
  unrevealedMatches.update((count) => Math.max(0, count - 1));
}
