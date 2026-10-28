CREATE TABLE login_tokens (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    email_encrypted bytea NOT NULL CHECK (octet_length(email_encrypted) > 0),
    email_lookup_hmac bytea NOT NULL UNIQUE CHECK (octet_length(email_lookup_hmac) = 32),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX login_tokens_expiry_idx ON login_tokens (expires_at);

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expiry_idx ON sessions (expires_at);

CREATE TABLE auth_rate_limits (
    key_hash bytea NOT NULL CHECK (octet_length(key_hash) = 32),
    window_start timestamptz NOT NULL,
    attempts integer NOT NULL CHECK (attempts > 0),
    PRIMARY KEY (key_hash, window_start)
);

CREATE INDEX auth_rate_limits_window_idx ON auth_rate_limits (window_start);
