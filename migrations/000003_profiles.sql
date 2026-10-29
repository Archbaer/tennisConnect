CREATE TABLE profiles (
    user_id uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 50),
    gender text NOT NULL CHECK (gender IN ('woman', 'man')),
    skill_level text NOT NULL CHECK (skill_level IN ('beginner', 'improver', 'intermediate', 'advanced')),
    ranking text CHECK (char_length(ranking) <= 50),
    bio text NOT NULL DEFAULT '' CHECK (char_length(bio) <= 200),
    gender_preference text NOT NULL CHECK (gender_preference IN ('women', 'men', 'all')),
    min_age smallint NOT NULL CHECK (min_age >= 18),
    max_age smallint NOT NULL CHECK (max_age <= 120),
    skill_tolerance smallint NOT NULL DEFAULT 1 CHECK (skill_tolerance IN (0, 1)),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (min_age <= max_age)
);

CREATE TABLE availability (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    profile_id uuid NOT NULL REFERENCES profiles (user_id) ON DELETE CASCADE,
    time_zone text NOT NULL DEFAULT 'Europe/London' CHECK (time_zone = 'Europe/London'),
    weekday smallint NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    start_time time without time zone NOT NULL,
    end_time time without time zone NOT NULL,
    CHECK (end_time > start_time),
    UNIQUE (profile_id, weekday, start_time, end_time)
);
