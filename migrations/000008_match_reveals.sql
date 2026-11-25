BEGIN;

-- One row per viewer per match. Reading a match never writes; the reveal route
-- inserts idempotently on first view. The primary key also serves the match-list
-- left join, so no secondary index is needed.
CREATE TABLE match_reveals (
    match_id uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    revealed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (match_id, user_id)
);

COMMIT;
