import { execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const seedPath = fileURLToPath(new URL('./seed.sql', import.meta.url));
const statePath = fileURLToPath(new URL('./.auth.json', import.meta.url));
const databaseURL =
  process.env.TEST_DATABASE_URL ??
  'postgres://tennisconnect:local-development-only@localhost:5432/tennisconnect_test?sslmode=disable';

function tokenPair(name: string) {
  const token = randomBytes(32).toString('base64url');
  return { name, token, hash: createHash('sha256').update(token).digest('hex') };
}

export default function seed(): void {
  const player = tokenPair('player');
  const banned = tokenPair('banned');
  const reviewer = tokenPair('reviewer');
  const sql = readFileSync(seedPath, 'utf8')
    .replaceAll('{{PLAYER_HASH}}', player.hash)
    .replaceAll('{{BANNED_HASH}}', banned.hash)
    .replaceAll('{{REVIEWER_HASH}}', reviewer.hash);

  writeFileSync(
    statePath,
    JSON.stringify({
      player: player.token,
      banned: banned.token,
      reviewer: reviewer.token
    }),
    { mode: 0o600 }
  );

  try {
    execFileSync('psql', [databaseURL, '-v', 'ON_ERROR_STOP=1', '-f', '-'], {
      input: sql,
      stdio: ['pipe', 'inherit', 'inherit']
    });
    return;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'ENOENT') {
      throw error;
    }
  }

  // Local development often has Docker but no host psql client.
  execFileSync(
    'docker',
    [
      'compose',
      'exec',
      '-T',
      'database',
      'psql',
      '--username=tennisconnect',
      '--dbname=tennisconnect_test',
      '-v',
      'ON_ERROR_STOP=1',
      '-f',
      '-'
    ],
    { input: sql, stdio: ['pipe', 'inherit', 'inherit'], cwd: '..' }
  );
}
