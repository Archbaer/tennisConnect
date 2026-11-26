#!/usr/bin/env bash
set -euo pipefail

# Local-only development fixture. It never changes server auth behaviour and
# refuses to touch anything except the local Docker database named tennisconnect.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! docker compose ps database >/dev/null 2>&1; then
  echo "Start the local database first: docker compose up -d database" >&2
  exit 1
fi

psql_local() {
  docker compose exec -T database psql --username=tennisconnect --dbname=tennisconnect "$@"
}

if ! psql_local -Atc "SELECT to_regclass('public.recovery_sessions') IS NOT NULL" | grep -q t; then
  cat >&2 <<'EOF'
The local database schema is stale. Apply the latest migrations first:

  docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 \
    -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000009_recovery_sessions.sql
EOF
  exit 1
fi

TOKEN="$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')"
HASH="$(printf '%s' "$TOKEN" | openssl dgst -sha256 -binary | xxd -p -c 32)"

psql_local --set=ON_ERROR_STOP=1 -v token_hash="$HASH" <<'SQL'
BEGIN;

DELETE FROM users WHERE id IN (
  '00000000-0000-4000-8000-000000000301',
  '00000000-0000-4000-8000-000000000302'
);

INSERT INTO users (id, email_encrypted, email_lookup_hmac, birth_date, email_verified_at, last_active_at)
VALUES
  ('00000000-0000-4000-8000-000000000301', decode('01', 'hex'), decode(repeat('d1', 32), 'hex'), '1990-01-01', now(), now()),
  ('00000000-0000-4000-8000-000000000302', decode('02', 'hex'), decode(repeat('d2', 32), 'hex'), '1991-02-02', now(), now());

INSERT INTO profiles (user_id, display_name, gender, skill_level, bio, gender_preference, min_age, max_age, skill_tolerance)
VALUES
  ('00000000-0000-4000-8000-000000000301', 'Dev Player', 'woman', 'intermediate', 'Local development player.', 'all', 18, 120, 1),
  ('00000000-0000-4000-8000-000000000302', 'Navya', 'woman', 'intermediate', 'Local development candidate.', 'all', 18, 120, 1);

INSERT INTO availability (profile_id, weekday, start_time, end_time)
VALUES
  ('00000000-0000-4000-8000-000000000301', 2, '18:00', '21:00'),
  ('00000000-0000-4000-8000-000000000301', 4, '18:00', '21:00'),
  ('00000000-0000-4000-8000-000000000302', 2, '18:00', '21:00'),
  ('00000000-0000-4000-8000-000000000302', 4, '18:00', '21:00');

INSERT INTO locations (user_id, home, home_area_label)
VALUES
  ('00000000-0000-4000-8000-000000000301', ST_SetSRID(ST_MakePoint(-0.16, 51.46), 4326)::geography, 'Battersea'),
  ('00000000-0000-4000-8000-000000000302', ST_SetSRID(ST_MakePoint(-0.15, 51.47), 4326)::geography, 'Battersea');

INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
VALUES
  ('00000000-0000-4000-8000-000000000301', 'dev/player-1', 1, 'approved'),
  ('00000000-0000-4000-8000-000000000302', 'dev/candidate-1', 1, 'approved');

INSERT INTO sessions (token_hash, user_id, expires_at)
VALUES (decode(:'token_hash', 'hex'), '00000000-0000-4000-8000-000000000301', now() + interval '7 days');

COMMIT;
SQL

cat <<EOF
Local Dev Player session created.

Cookie:
document.cookie = 'tennisconnect_session=$TOKEN; path=/; SameSite=Lax';

Or curl:
curl -H 'Cookie: tennisconnect_session=$TOKEN' http://localhost:8080/api/v1/profile

This is local-only, expires in 7 days, and seeds no real e-mail address.
EOF
