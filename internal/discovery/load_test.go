package discovery

import (
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tennisconnect/internal/testutil"
)

const loadSeedSQL = `email_encrypted = '\x73656564'`

func loadSeedEnabled(t *testing.T) (int, bool) {
	t.Helper()
	if os.Getenv("DISCOVERY_LOAD") != "1" {
		t.Log("DISCOVERY_LOAD=1 not set; skipping the bounded load gate")
		return 0, false
	}
	if testutil.DB == nil {
		t.Fatal("DISCOVERY_LOAD=1 requires TEST_DATABASE_URL")
	}
	seconds := 60
	if raw := os.Getenv("DISCOVERY_LOAD_SECONDS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 3600 {
			t.Fatalf("DISCOVERY_LOAD_SECONDS must be 1..3600, got %q", raw)
		}
		seconds = value
	}
	return seconds, true
}

func seedLoadData(t *testing.T) string {
	t.Helper()
	cleanupLoadData(t)
	t.Cleanup(func() { cleanupLoadData(t) })

	mustExec(t, `
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date, last_active_at)
		SELECT '\x73656564', decode(md5('seed-a-' || i) || md5('seed-b-' || i), 'hex'),
			date '1980-01-01' + (i % 9000), now() - ((i % 3) * interval '1 day')
		FROM generate_series(1, 100000) AS i`)

	mustExec(t, `
		INSERT INTO profiles (user_id, display_name, gender, skill_level, bio,
			gender_preference, min_age, max_age, skill_tolerance)
		SELECT u.id, 'Load ' || left(u.id::text, 8),
			CASE WHEN row_number() OVER (ORDER BY u.id) % 2 = 0 THEN 'woman' ELSE 'man' END,
			CASE row_number() OVER (ORDER BY u.id) % 4
				WHEN 0 THEN 'beginner' WHEN 1 THEN 'improver' WHEN 2 THEN 'intermediate' ELSE 'advanced'
			END,
			'Load bio', 'all', 18, 120, 1
		FROM users u WHERE u.`+loadSeedSQL)

	mustExec(t, `
		INSERT INTO availability (profile_id, weekday, start_time, end_time)
		SELECT p.user_id, 2, '18:00', '21:00'
		FROM profiles p JOIN users u ON u.id = p.user_id
		WHERE u.`+loadSeedSQL)

	mustExec(t, `
		INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
		SELECT u.id, 'photos/load/' || u.id, 1, 'approved'
		FROM users u WHERE u.`+loadSeedSQL)

	mustExec(t, `
		INSERT INTO locations (user_id, home, home_area_label, travel, travel_area_label, travel_expires_at)
		WITH seeded AS (
			SELECT id, row_number() OVER (ORDER BY id) AS rn
			FROM users WHERE `+loadSeedSQL+`
		)
		SELECT seeded.id,
			CASE WHEN seeded.rn % 50 = 0
				THEN ST_SetSRID(ST_MakePoint(-0.1278 + (random() - 0.5) * 0.35, 51.5074 + (random() - 0.5) * 0.35), 4326)::geography
				ELSE ST_SetSRID(ST_MakePoint(-6.0 + random() * 9.0, 50.0 + random() * 8.0), 4326)::geography
			END,
			'Load area',
			CASE WHEN seeded.rn % 20 = 0
				THEN CASE WHEN seeded.rn % 50 = 0
					THEN ST_SetSRID(ST_MakePoint(-0.1278 + (random() - 0.5) * 0.35, 51.5074 + (random() - 0.5) * 0.35), 4326)::geography
					ELSE ST_SetSRID(ST_MakePoint(-6.0 + random() * 9.0, 50.0 + random() * 8.0), 4326)::geography
				END
			END,
			CASE WHEN seeded.rn % 20 = 0 THEN 'Load travel' END,
			CASE WHEN seeded.rn % 20 = 0 THEN now() + interval '7 days' END
		FROM seeded`)

	for _, table := range []string{"users", "profiles", "availability", "photos", "locations"} {
		mustExec(t, "ANALYZE "+table)
	}

	var viewerID string
	if err := testutil.DB.QueryRow(`
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date, last_active_at)
		VALUES ('\x01', decode(md5('load-viewer-a') || md5('load-viewer-b'), 'hex'), '1990-01-01', now())
		RETURNING id`).Scan(&viewerID); err != nil {
		t.Fatalf("insert load viewer: %v", err)
	}
	mustExec(t, `
		INSERT INTO profiles (user_id, display_name, gender, skill_level, bio,
			gender_preference, min_age, max_age, skill_tolerance)
		VALUES ($1, 'Load viewer', 'woman', 'intermediate', 'Load bio', 'all', 18, 120, 1)`, viewerID)
	mustExec(t, `
		INSERT INTO availability (profile_id, weekday, start_time, end_time)
		VALUES ($1, 2, '18:00', '21:00')`, viewerID)
	mustExec(t, `
		INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
		VALUES ($1, 'photos/load-viewer', 1, 'approved')`, viewerID)
	mustExec(t, `
		INSERT INTO locations (user_id, home, home_area_label)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, 'Load area')`,
		viewerID, londonLongitude, londonLatitude)
	return viewerID
}

func cleanupLoadData(t *testing.T) {
	t.Helper()
	if testutil.DB == nil {
		return
	}
	if _, err := testutil.DB.Exec("DELETE FROM users WHERE " + loadSeedSQL); err != nil {
		t.Errorf("clean load seed users: %v", err)
	}
	if _, err := testutil.DB.Exec("DELETE FROM users WHERE email_lookup_hmac = decode(md5('load-viewer-a') || md5('load-viewer-b'), 'hex')"); err != nil {
		t.Errorf("clean load viewer: %v", err)
	}
}

func explainCandidateQuery(t *testing.T, viewerID string) string {
	t.Helper()
	rows, err := testutil.DB.Query("EXPLAIN (ANALYZE, BUFFERS) "+candidateQuery,
		viewerID, float64(10)*metresPerMile, nil, nil, 21)
	if err != nil {
		t.Fatalf("explain candidate query: %v", err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("explain rows: %v", err)
	}
	return plan.String()
}

func soakDiscovery(t *testing.T, service *Service, viewerID string, seconds int) {
	t.Helper()
	total := seconds * 50
	latencies := make([]time.Duration, total)
	var failures atomic.Int64
	var wait sync.WaitGroup
	slots := make(chan struct{}, 50)
	start := time.Now()
	for index := range total {
		time.Sleep(time.Until(start.Add(time.Duration(index) * 20 * time.Millisecond)))
		wait.Add(1)
		slots <- struct{}{}
		go func(index int) {
			defer wait.Done()
			defer func() { <-slots }()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/discovery?radius=10&limit=20", nil)
			response := httptest.NewRecorder()
			began := time.Now()
			service.Get(response, request, viewerID)
			latencies[index] = time.Since(began)
			if response.Code != http.StatusOK {
				failures.Add(1)
			}
		}(index)
	}
	wait.Wait()

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95Index := int(math.Ceil(0.95*float64(total))) - 1
	p95 := latencies[p95Index]
	errorCount := int(failures.Load())
	t.Logf("discovery load: %d requests over %ds, p95=%v, errors=%d (%.3f%%)", total, seconds, p95, errorCount, 100*float64(errorCount)/float64(total))
	if p95 >= 500*time.Millisecond {
		t.Fatalf("p95 = %v, want < 500ms", p95)
	}
	if errorCount*1000 >= total {
		t.Fatalf("error rate = %d/%d, want < 0.1%%", errorCount, total)
	}
}

func TestDiscoveryLoadGate(t *testing.T) {
	seconds, enabled := loadSeedEnabled(t)
	if !enabled {
		t.Skip("set DISCOVERY_LOAD=1 to run the bounded discovery load gate")
	}
	viewerID := seedLoadData(t)
	t.Cleanup(func() { cleanupLoadData(t) })

	plan := explainCandidateQuery(t, viewerID)
	t.Logf("candidate plan:\n%s", plan)
	if !strings.Contains(plan, "locations_home_gist") {
		t.Fatalf("home branch did not use its GiST index:\n%s", plan)
	}
	if !strings.Contains(plan, "locations_travel_gist") {
		t.Fatalf("travel branch did not use its GiST index:\n%s", plan)
	}

	service := &Service{DB: testutil.DB, Presigner: newFakePresigner()}
	soakDiscovery(t, service, viewerID, seconds)
}
