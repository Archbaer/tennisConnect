BEGIN;

-- A reaction is the viewer's one-way verdict. Rows are never updated: a pass is
-- permanent in V1, so a repeat reaction is rejected at the API boundary.
CREATE TABLE reactions (
    actor_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    target_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    action text NOT NULL CHECK (action IN ('like', 'pass')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (actor_id, target_id),
    CHECK (actor_id <> target_id)
);

-- No secondary index on reactions: the primary key (actor_id, target_id)
-- already serves both the discovery exclusion (actor_id = viewer) and the
-- reciprocal-like lookup (actor_id = target, target_id = actor).

-- A match is stored once per unordered pair. The canonical column order is
-- enforced so a concurrent reciprocal like cannot create a second row.
CREATE TABLE matches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_low uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    user_high uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    matched_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_low, user_high),
    CHECK (user_low < user_high)
);

CREATE INDEX matches_user_high_idx ON matches (user_high);

-- A block is one-way in storage but two-way in effect: every query checks both
-- directions. Blocks are never deleted in V1, so a match stays closed.
CREATE TABLE blocks (
    blocker_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    blocked_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);

CREATE INDEX blocks_blocked_idx ON blocks (blocked_id);

-- The Instagram handle is optional and disclosed only through an explicit
-- match-scoped read. Stored bare (no leading @) and lower case. The pattern
-- matches the Go validator exactly: no leading, trailing or doubled dot.
ALTER TABLE profiles
    ADD COLUMN instagram_handle text CHECK (
        instagram_handle ~ '^[a-z0-9_]([a-z0-9_.]*[a-z0-9_])?$'
        AND instagram_handle !~ '\.\.'
        AND char_length(instagram_handle) BETWEEN 1 AND 30
    );

COMMIT;
