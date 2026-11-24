import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

export type E2ETokens = {
  player: string;
  banned: string;
  reviewer: string;
};

// Written by global-setup.ts after the test database is seeded.
export function tokens(): E2ETokens {
  const path = fileURLToPath(new URL('./.auth.json', import.meta.url));
  return JSON.parse(readFileSync(path, 'utf8')) as E2ETokens;
}
