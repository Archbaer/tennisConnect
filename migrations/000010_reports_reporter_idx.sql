BEGIN;

-- Discovery hides every target the viewer has reported, including reports
-- that a reviewer later dismissed. The existing unique index is partial on
-- status = 'open', so it cannot serve that reporter-scoped lookup.
CREATE INDEX reports_reporter_target_idx ON reports (reporter_id, target_id);

COMMIT;
