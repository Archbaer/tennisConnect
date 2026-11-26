BEGIN;

-- A short-lived, recovery-only credential. It is not a player session: the
-- account stays deleted until the owner explicitly recovers or continues.
CREATE TABLE recovery_sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);

CREATE INDEX recovery_sessions_user_idx ON recovery_sessions (user_id);
CREATE INDEX recovery_sessions_expiry_idx ON recovery_sessions (expires_at);

COMMIT;
