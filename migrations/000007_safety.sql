BEGIN;

-- Reviewers are a separate identity. They never hold a player profile and are
-- created only by the cmd/reviewer operator tool. The TOTP secret stays
-- encrypted; the e-mail is encrypted and HMACed like a player account.
CREATE TABLE reviewers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email_encrypted bytea NOT NULL CHECK (octet_length(email_encrypted) > 0),
    email_lookup_hmac bytea NOT NULL UNIQUE CHECK (octet_length(email_lookup_hmac) = 32),
    totp_secret_encrypted bytea CHECK (octet_length(totp_secret_encrypted) > 0),
    last_totp_step bigint NOT NULL DEFAULT 0 CHECK (last_totp_step >= 0),
    mfa_enrolled_at timestamptz,
    last_active_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reviewer_login_tokens (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    reviewer_id uuid NOT NULL REFERENCES reviewers (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX reviewer_login_tokens_expiry_idx ON reviewer_login_tokens (expires_at);

CREATE TABLE reviewer_sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    reviewer_id uuid NOT NULL REFERENCES reviewers (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    mfa_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX reviewer_sessions_reviewer_idx ON reviewer_sessions (reviewer_id);
CREATE INDEX reviewer_sessions_expiry_idx ON reviewer_sessions (expires_at);

CREATE TABLE reviewer_rate_limits (
    key_hash bytea NOT NULL CHECK (octet_length(key_hash) = 32),
    window_start timestamptz NOT NULL,
    attempts integer NOT NULL CHECK (attempts > 0),
    PRIMARY KEY (key_hash, window_start)
);

CREATE INDEX reviewer_rate_limits_window_idx ON reviewer_rate_limits (window_start);

-- A report is the only way a player asks a human to look at another account.
-- Reporter identity is never returned to the target or exposed in player APIs.
CREATE TABLE reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    target_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category text NOT NULL CHECK (category IN (
        'fake_profile',
        'inappropriate_photos',
        'harassment',
        'contact_details',
        'underage',
        'other'
    )),
    details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'dismissed', 'banned')),
    review_due_at timestamptz NOT NULL DEFAULT (now() + interval '72 hours'),
    reviewer_id uuid REFERENCES reviewers (id) ON DELETE SET NULL,
    reviewed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (reporter_id <> target_id)
);

-- One open report per pair keeps the queue free of duplicate submissions while
-- still allowing a fresh report after a dismissal.
CREATE UNIQUE INDEX reports_one_open_per_pair_idx
    ON reports (reporter_id, target_id) WHERE status = 'open';
CREATE INDEX reports_open_queue_idx ON reports (review_due_at) WHERE status = 'open';
CREATE INDEX reports_target_idx ON reports (target_id);

-- Append-only reviewer audit trail. No API updates or deletes these rows.
CREATE TABLE report_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_id uuid NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
    actor_kind text NOT NULL CHECK (actor_kind IN ('user', 'reviewer', 'system')),
    actor_id uuid,
    action text NOT NULL CHECK (action IN ('submitted', 'dismissed', 'banned', 'appeal_submitted')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX report_events_report_idx ON report_events (report_id, created_at);

-- A ban is permanent in V1. banned_at is the durable marker that blocks
-- same-email re-registration even after a deletion purge.
ALTER TABLE users
    ADD COLUMN banned_at timestamptz,
    ADD COLUMN ban_category text CHECK (ban_category IS NULL OR ban_category IN (
        'fake_profile',
        'inappropriate_photos',
        'harassment',
        'contact_details',
        'underage',
        'other'
    ));

-- One appeal per banned account, submitted inside the 14-day window enforced
-- in the handler. No automated unban path exists in V1.
CREATE TABLE appeals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    report_id uuid REFERENCES reports (id) ON DELETE SET NULL,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    status text NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'upheld', 'reinstated')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX appeals_status_idx ON appeals (status, created_at);

ALTER TABLE users
    ADD COLUMN ban_report_id uuid REFERENCES reports (id) ON DELETE SET NULL;

CREATE TABLE safety_rate_limits (
    key_hash bytea NOT NULL CHECK (octet_length(key_hash) = 32),
    window_start timestamptz NOT NULL,
    attempts integer NOT NULL CHECK (attempts > 0),
    PRIMARY KEY (key_hash, window_start)
);

CREATE INDEX safety_rate_limits_window_idx ON safety_rate_limits (window_start);

COMMIT;
