BEGIN;

CREATE TABLE locations (
    user_id uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    home geography(Point, 4326) NOT NULL,
    home_area_label text NOT NULL CHECK (char_length(home_area_label) BETWEEN 1 AND 80),
    travel geography(Point, 4326),
    travel_area_label text CHECK (travel_area_label IS NULL OR char_length(travel_area_label) BETWEEN 1 AND 80),
    travel_expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (travel IS NULL AND travel_area_label IS NULL AND travel_expires_at IS NULL)
        OR (travel IS NOT NULL AND travel_area_label IS NOT NULL AND travel_expires_at IS NOT NULL)
    )
);

CREATE INDEX locations_home_gist ON locations USING GIST (home);
CREATE INDEX locations_travel_gist ON locations USING GIST (travel);

COMMIT;
