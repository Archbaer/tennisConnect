BEGIN;

CREATE TABLE photos (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    object_key text NOT NULL UNIQUE CHECK (char_length(object_key) > 0),
    display_order smallint NOT NULL CHECK (display_order BETWEEN 1 AND 3),
    moderation_status text NOT NULL DEFAULT 'pending' CHECK (moderation_status IN ('pending', 'approved', 'rejected')),
    provider_reference text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_id, display_order)
);

COMMIT;
