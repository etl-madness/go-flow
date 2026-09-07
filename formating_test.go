package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/etl-madness/flow"
	_ "modernc.org/sqlite"
)

func TestGenerateRunID(t *testing.T) {
	id1 := generateRunID()
	id2 := generateRunID()

	if len(id1) != 32 {
		t.Fatalf("expected run ID length 32, got %d (%s)", len(id1), id1)
	}
	if len(id2) != 32 {
		t.Fatalf("expected run ID length 32, got %d (%s)", len(id2), id2)
	}
	if id1 == id2 {
		t.Fatalf("expected unique run IDs, got identical values: %s", id1)
	}
}

func TestDatabaseSinkRunDetails(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE pipeline_events (
			run_id VARCHAR(64) NOT NULL,
			execution_id VARCHAR(64) NOT NULL,
			sequence_num INT NOT NULL,
			occurred_at DATETIME NOT NULL,
			event_type VARCHAR(64) NOT NULL,
			node_kind VARCHAR(64) NOT NULL,
			node_id VARCHAR(128) NOT NULL,
			status VARCHAR(32) NOT NULL,
			error_message TEXT,
			rows_read BIGINT,
			rows_written BIGINT,
			rows_affected BIGINT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create pipeline_events table: %v", err)
	}

	sink := NewDatabaseSink(db, "sqlite")

	if sink.RunID() != "" {
		t.Fatalf("expected initial runID to be empty, got: %s", sink.RunID())
	}

	ctx := context.Background()

	// Emit run.started event
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-abc-123",
		Type:       flow.EventRunStarted,
		OccurredAt: time.Now(),
		Status:     flow.RunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("failed to emit event: %v", err)
	}

	if sink.RunID() != "run-abc-123" {
		t.Fatalf("expected runID 'run-abc-123', got: %s", sink.RunID())
	}

	// Emit run.finished event
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:        "run-abc-123",
		Type:         flow.EventRunFinished,
		OccurredAt:   time.Now(),
		Status:       flow.RunStatusFailed,
		ErrorClass:   flow.ErrorClassDatabase,
		ErrorMessage: "connection timed out",
	})
	if err != nil {
		t.Fatalf("failed to emit finish event: %v", err)
	}

	runID, status, errClass, errMsg := sink.RunDetails()
	if runID != "run-abc-123" {
		t.Errorf("expected runID 'run-abc-123', got: %s", runID)
	}
	if status != "failed" {
		t.Errorf("expected status 'failed', got: %s", status)
	}
	if errClass != "database" {
		t.Errorf("expected errorClass 'database', got: %s", errClass)
	}
	if errMsg != "connection timed out" {
		t.Errorf("expected errorMessage 'connection timed out', got: %s", errMsg)
	}
}

func TestLogRunSummaryToDB_PrimaryKeyConstraint(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer db.Close()

	// Create pipeline_runs table with PRIMARY KEY on run_id
	_, err = db.Exec(`
		CREATE TABLE pipeline_runs (
			run_id VARCHAR(64) PRIMARY KEY,
			file_path VARCHAR(4000) NOT NULL,
			config_path VARCHAR(4000),
			status VARCHAR(32) NOT NULL,
			started_at DATETIME NOT NULL,
			finished_at DATETIME NOT NULL,
			duration_ms BIGINT NOT NULL,
			task_count INT NOT NULL,
			error_class VARCHAR(128),
			error_message TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create pipeline_runs table: %v", err)
	}

	ctx := context.Background()
	now := time.Now()

	// 1. Insert first record with empty RunID - should auto-generate unique RunID
	summary1 := RunSummaryRecord{
		RunID:      "",
		FilePath:   "scripts.xml",
		Status:     "succeeded",
		StartedAt:  now,
		FinishedAt: now.Add(time.Second),
		Duration:   time.Second,
		TaskCount:  5,
	}

	if err := LogRunSummaryToDB(ctx, db, "sqlite", summary1); err != nil {
		t.Fatalf("first LogRunSummaryToDB failed: %v", err)
	}

	// 2. Insert second record also with empty RunID - should auto-generate another unique RunID and NOT collide!
	summary2 := RunSummaryRecord{
		RunID:      "",
		FilePath:   "scripts.xml",
		Status:     "succeeded",
		StartedAt:  now,
		FinishedAt: now.Add(time.Second),
		Duration:   time.Second,
		TaskCount:  5,
	}

	if err := LogRunSummaryToDB(ctx, db, "sqlite", summary2); err != nil {
		t.Fatalf("second LogRunSummaryToDB failed (possible duplicate key): %v", err)
	}

	// Verify both records were inserted with non-empty, distinct run IDs
	rows, err := db.Query("SELECT run_id FROM pipeline_runs")
	if err != nil {
		t.Fatalf("failed to query pipeline_runs: %v", err)
	}
	defer rows.Close()

	var runIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("failed to scan run_id: %v", err)
		}
		if id == "" {
			t.Fatalf("inserted run_id was empty")
		}
		runIDs = append(runIDs, id)
	}

	if len(runIDs) != 2 {
		t.Fatalf("expected 2 rows in pipeline_runs, found: %d", len(runIDs))
	}
	if runIDs[0] == runIDs[1] {
		t.Fatalf("expected distinct run IDs, got duplicates: %s and %s", runIDs[0], runIDs[1])
	}
}
