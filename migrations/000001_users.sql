BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email_encrypted bytea NOT NULL CHECK (octet_length(email_encrypted) > 0),
    email_lookup_hmac bytea NOT NULL UNIQUE CHECK (octet_length(email_lookup_hmac) = 32),
    email_verified_at timestamptz,
    birth_date date NOT NULL,
    last_active_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

COMMIT;
