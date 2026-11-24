BEGIN;

TRUNCATE login_tokens, sessions, auth_rate_limits, reviewers, reviewer_login_tokens,
    reviewer_sessions, reviewer_rate_limits, safety_rate_limits, reports, report_events,
    appeals, users CASCADE;

-- Fixed test identities. Tokens are only valid in the local/CI test database.
INSERT INTO users (id, email_encrypted, email_lookup_hmac, birth_date, email_verified_at, last_active_at)
VALUES
    ('00000000-0000-4000-8000-000000000101', decode('01', 'hex'), decode(repeat('a1', 32), 'hex'), '1990-01-01', now(), now()),
    ('00000000-0000-4000-8000-000000000102', decode('02', 'hex'), decode(repeat('a2', 32), 'hex'), '1991-02-02', now(), now()),
    ('00000000-0000-4000-8000-000000000103', decode('03', 'hex'), decode(repeat('a3', 32), 'hex'), '1992-03-03', now(), now());

UPDATE users SET banned_at = now() - interval '1 day', ban_category = 'harassment'
WHERE id = '00000000-0000-4000-8000-000000000102';

INSERT INTO profiles (user_id, display_name, gender, skill_level, bio, gender_preference, min_age, max_age, skill_tolerance)
VALUES
    ('00000000-0000-4000-8000-000000000101', 'E2E Player', 'man', 'intermediate', 'Test player.', 'all', 18, 120, 1),
    ('00000000-0000-4000-8000-000000000103', 'Navya', 'woman', 'intermediate', 'Test candidate.', 'all', 18, 120, 1);

INSERT INTO availability (profile_id, weekday, start_time, end_time)
VALUES
    ('00000000-0000-4000-8000-000000000101', 2, '18:00', '21:00'),
    ('00000000-0000-4000-8000-000000000103', 2, '18:00', '21:00');

INSERT INTO locations (user_id, home, home_area_label)
VALUES
    ('00000000-0000-4000-8000-000000000101', ST_SetSRID(ST_MakePoint(-0.16, 51.46), 4326)::geography, 'Battersea'),
    ('00000000-0000-4000-8000-000000000103', ST_SetSRID(ST_MakePoint(-0.15, 51.47), 4326)::geography, 'Battersea');

INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
VALUES
    ('00000000-0000-4000-8000-000000000101', 'e2e/player-1', 1, 'approved'),
    ('00000000-0000-4000-8000-000000000103', 'e2e/candidate-1', 1, 'approved');

INSERT INTO sessions (token_hash, user_id, expires_at)
VALUES
    (decode('{{PLAYER_HASH}}', 'hex'),
     '00000000-0000-4000-8000-000000000101', now() + interval '1 day'),
    (decode('{{BANNED_HASH}}', 'hex'),
     '00000000-0000-4000-8000-000000000102', now() + interval '1 day');

INSERT INTO reviewers (id, email_encrypted, email_lookup_hmac)
VALUES ('00000000-0000-4000-8000-000000000201', decode('04', 'hex'), decode(repeat('b1', 32), 'hex'));

INSERT INTO reviewer_sessions (token_hash, reviewer_id, expires_at, mfa_at)
VALUES (decode('{{REVIEWER_HASH}}', 'hex'),
        '00000000-0000-4000-8000-000000000201', now() + interval '1 hour', NULL);

COMMIT;
