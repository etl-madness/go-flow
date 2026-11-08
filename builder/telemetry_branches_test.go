package builder

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestTelemetryFilterDialects(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := `
CREATE TABLE pipeline_runs (
 run_id TEXT, file_path TEXT, config_path TEXT, status TEXT, started_at TEXT, user_name TEXT,
 hostname TEXT, application_name TEXT, error_class TEXT, error_message TEXT
);
INSERT INTO pipeline_runs VALUES ('run-1','pipeline.xml','','succeeded','2026-01-02T03:04:05Z','user','host','worker','','');
INSERT INTO pipeline_runs VALUES ('run-2','other.xml','','failed','2026-01-01T03:04:05Z','user','host','worker','error','failure');
CREATE TABLE pipeline_events (run_id TEXT, sequence_num INTEGER, event_type TEXT, spid TEXT);
INSERT INTO pipeline_events VALUES ('run-1', 1, 'node.finished', '12->13');`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			runs, err := QueryPipelineRuns(ctx, db, driver, "pipeline_runs", "pipeline", "succeeded", 10)
			if err != nil || len(runs) != 1 || runs[0].RunID != "run-1" || runs[0].StartedAt == nil {
				t.Fatalf("runs = %+v, %v", runs, err)
			}
			events, err := QueryPipelineEvents(ctx, db, driver, "pipeline_events", "run-1")
			if err != nil || len(events) != 1 || events[0].SPID != "12->13" {
				t.Fatalf("events = %+v, %v", events, err)
			}
		})
	}
	if _, err := QueryPipelineRuns(ctx, db, "oracle", "pipeline_runs", "pipeline", "succeeded", 10); err == nil {
		t.Fatal("SQLite should reject Oracle named parameter binding")
	}
}

func TestTelemetryValueConversions(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, value := range []any{now, &now, "2026-01-02T03:04:05Z", []byte("2026-01-02 03:04:05")} {
		got := parseFlexibleTime(value)
		if got == nil || !got.Equal(now) {
			t.Errorf("parseFlexibleTime(%v) = %v", value, got)
		}
	}
	for _, value := range []any{nil, "", "invalid", 1} {
		if got := parseFlexibleTime(value); got != nil {
			t.Errorf("parseFlexibleTime(%v) = %v, want nil", value, got)
		}
	}
	for _, tc := range []struct {
		value any
		want  any
	}{
		{nil, nil}, {int64(0), nil}, {int(12), int64(12)}, {int32(13), int64(13)},
		{[]byte("14"), int64(14)}, {[]byte(" "), nil}, {"15", int64(15)}, {"x", "x"}, {float64(16), int64(16)},
	} {
		if got := parseSPIDValue(tc.value); got != tc.want {
			t.Errorf("parseSPIDValue(%v) = %v, want %v", tc.value, got, tc.want)
		}
	}
}
