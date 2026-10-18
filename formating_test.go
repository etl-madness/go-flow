package main

import (
	"context"
	"database/sql"
	"strings"
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

func TestConfigureExecutorOptionsPath(t *testing.T) {
	executor := flow.NewExecutor(flow.NewRegistry())
	dsn := "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name = 'OPTIONS_GLOBAL'"

	configureExecutorOptionsPath(executor, dsn)

	result, err := executor.ExecuteRun(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected successful empty execution, got error: %v", err)
	}
	if result.OptionsPath != dsn {
		t.Fatalf("expected options path %q, got %q", dsn, result.OptionsPath)
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
			user_name VARCHAR(256),
			hostname VARCHAR(256),
			options_path VARCHAR(2048),
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
		RunID:       "run-abc-123",
		Type:        flow.EventRunStarted,
		OccurredAt:  time.Now().UTC(),
		Status:      flow.RunStatusSucceeded,
		UserName:    "alice",
		Hostname:    "etl-worker.example.com",
		OptionsPath: "/tmp/options.xml",
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
		OccurredAt:   time.Now().UTC(),
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

	var userName, hostname, optionsPath string
	if err := db.QueryRow("SELECT user_name, hostname, options_path FROM pipeline_events WHERE run_id = ? ORDER BY sequence_num DESC LIMIT 1", "run-abc-123").Scan(&userName, &hostname, &optionsPath); err != nil {
		t.Fatalf("failed to read event identity columns: %v", err)
	}
	if userName != "alice" {
		t.Fatalf("expected user_name 'alice', got: %s", userName)
	}
	if hostname != "etl-worker.example.com" {
		t.Fatalf("expected hostname 'etl-worker.example.com', got: %s", hostname)
	}
	if optionsPath != "/tmp/options.xml" {
		t.Fatalf("expected options_path '/tmp/options.xml', got: %s", optionsPath)
	}
}

func TestLogRunSummaryToDB_MasksSensitiveSourceInNonDebugMode(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer db.Close()

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
			user_name VARCHAR(256),
			hostname VARCHAR(256),
			options_path VARCHAR(2048),
			error_class VARCHAR(128),
			error_message TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create pipeline_runs table: %v", err)
	}

	rec := RunSummaryRecord{
		RunID:       "run-mask-1",
		FilePath:    "sql://postgres@postgres://app_user:Password123!@prod-db:5432/appdb?sslmode=require#SELECT xml FROM t",
		ConfigPath:  "sql://mysql@mysql://ops:Secret!@db.example.com:3306/etl#SELECT xml FROM cfg",
		Status:      "succeeded",
		StartedAt:   time.Now().UTC(),
		FinishedAt:  time.Now().UTC().Add(time.Second),
		Duration:    time.Second,
		TaskCount:   3,
		UserName:    "alice",
		Hostname:    "etl-host",
		OptionsPath: "https://user:supersecret@example.com/options.xml",
	}

	if err := LogRunSummaryToDB(context.Background(), db, "sqlite", rec, false); err != nil {
		t.Fatalf("LogRunSummaryToDB failed: %v", err)
	}

	var filePath, configPath, optionsPath string
	if err := db.QueryRow("SELECT file_path, config_path, options_path FROM pipeline_runs WHERE run_id = ?", rec.RunID).Scan(&filePath, &configPath, &optionsPath); err != nil {
		t.Fatalf("failed to read masked summary values: %v", err)
	}
	if strings.Contains(filePath, "Password123!") || strings.Contains(filePath, "app_user:Password123!") {
		t.Fatalf("expected password to be masked in file_path, got %q", filePath)
	}
	if strings.Contains(configPath, "Secret!") || strings.Contains(configPath, "ops:Secret!") {
		t.Fatalf("expected password to be masked in config_path, got %q", configPath)
	}
	if strings.Contains(optionsPath, "supersecret") {
		t.Fatalf("expected password to be masked in options_path, got %q", optionsPath)
	}
	if !strings.Contains(filePath, "******") || !strings.Contains(configPath, "******") || !strings.Contains(optionsPath, "******") {
		t.Fatalf("expected masked password placeholder in all recorded sources, got file=%q config=%q options=%q", filePath, configPath, optionsPath)
	}

	debugRec := rec
	debugRec.RunID = "run-mask-debug"
	if err := LogRunSummaryToDB(context.Background(), db, "sqlite", debugRec, true); err != nil {
		t.Fatalf("LogRunSummaryToDB failed in debug mode: %v", err)
	}

	var debugFilePath string
	if err := db.QueryRow("SELECT file_path FROM pipeline_runs WHERE run_id = ?", debugRec.RunID).Scan(&debugFilePath); err != nil {
		t.Fatalf("failed to read debug-mode row: %v", err)
	}
	if !strings.Contains(debugFilePath, "Password123!") {
		t.Fatalf("expected debug-mode value to remain unmasked, got %q", debugFilePath)
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
			user_name VARCHAR(256),
			hostname VARCHAR(256),
			options_path VARCHAR(2048),
			error_class VARCHAR(128),
			error_message TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create pipeline_runs table: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Insert first record with empty RunID - should auto-generate unique RunID
	summary1 := RunSummaryRecord{
		RunID:       "",
		FilePath:    "scripts.xml",
		Status:      "succeeded",
		StartedAt:   now,
		FinishedAt:  now.Add(time.Second),
		Duration:    time.Second,
		TaskCount:   5,
		UserName:    "alice",
		Hostname:    "etl-worker.example.com",
		OptionsPath: "/tmp/options.xml",
	}

	if err := LogRunSummaryToDB(ctx, db, "sqlite", summary1, false); err != nil {
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

	if err := LogRunSummaryToDB(ctx, db, "sqlite", summary2, false); err != nil {
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

	var userName, hostname, optionsPath string
	if err := db.QueryRow("SELECT user_name, hostname, options_path FROM pipeline_runs ORDER BY started_at DESC LIMIT 1").Scan(&userName, &hostname, &optionsPath); err != nil {
		t.Fatalf("failed to read run identity columns: %v", err)
	}
	if userName != "alice" {
		t.Fatalf("expected user_name 'alice', got: %s", userName)
	}
	if hostname != "etl-worker.example.com" {
		t.Fatalf("expected hostname 'etl-worker.example.com', got: %s", hostname)
	}
	if optionsPath != "/tmp/options.xml" {
		t.Fatalf("expected options_path '/tmp/options.xml', got: %s", optionsPath)
	}
}

func TestMaskSensitiveSourceForDB(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTP URL with basic auth",
			input:    "https://alice:supersecret@example.com/pipeline.xml",
			expected: "https://alice:******@example.com/pipeline.xml",
		},
		{
			name:     "PostgreSQL DB URI with query",
			input:    "sql://postgres@postgres://appuser:SecretPass123@db.internal:5432/app#SELECT * FROM scripts",
			expected: "sql://postgres@postgres://appuser:******@db.internal:5432/app#SELECT * FROM scripts",
		},
		{
			name:     "PostgreSQL DB URI without fragment",
			input:    "sql://postgres@postgres://appuser:SecretPass123@db.internal:5432/app",
			expected: "sql://postgres@postgres://appuser:******@db.internal:5432/app",
		},
		{
			name:     "SQL Server Key-Value DSN with password keyword",
			input:    "sql://sqlserver@server=localhost;user id=sa;password=SuperSecretPassword!;database=master#SELECT xml FROM pipelines",
			expected: "sql://sqlserver@server=localhost;user id=sa;password=******;database=master#SELECT xml FROM pipelines",
		},
		{
			name:     "SQL Server Key-Value DSN with pwd keyword",
			input:    "server=localhost;uid=sa;pwd=SuperSecret123;database=test",
			expected: "server=localhost;uid=sa;pwd=******;database=test",
		},
		{
			name:     "Plain file path without credentials",
			input:    "/var/data/pipeline.xml",
			expected: "/var/data/pipeline.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSensitiveSourceForDB(tt.input, false)
			if got != tt.expected {
				t.Errorf("maskSensitiveSourceForDB(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFetchDatabaseSessionIdentity_SQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	spid, dbUser, appName, err := FetchDatabaseSessionIdentity(ctx, db, "sqlite")
	if err != nil {
		t.Fatalf("unexpected error fetching sqlite session identity: %v", err)
	}
	if spid <= 0 {
		t.Errorf("expected positive process ID for sqlite spid, got %d", spid)
	}
	if dbUser == "" {
		t.Errorf("expected non-empty database user, got empty string")
	}
	if appName == "" {
		t.Errorf("expected non-empty application name, got empty string")
	}
}

func TestDatabaseSink_EmitsSPIDAndDBUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
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
			user_name VARCHAR(256),
			os_user_name VARCHAR(256),
			db_user_name VARCHAR(256),
			spid BIGINT,
			sspid BIGINT,
			application_name VARCHAR(256),
			hostname VARCHAR(256),
			options_path VARCHAR(2048),
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
	sink.SetSessionIdentity(12345, "etl_db_user", "flow_test_app")

	ctx := context.Background()
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:       "run-spid-1",
		ExecutionID: "exec-spid-1",
		Sequence:    1,
		Type:        flow.EventNodeStarted,
		NodeKind:    "query",
		NodeID:      "node-1",
		Status:      flow.RunStatusSucceeded,
		UserName:    "os_alice",
		Hostname:    "worker-node",
	})
	if err != nil {
		t.Fatalf("failed to emit event: %v", err)
	}

	var (
		userName   string
		osUserName string
		dbUserName string
		spid       int64
		sspid      int64
		appName    string
	)
	row := db.QueryRow("SELECT user_name, os_user_name, db_user_name, spid, sspid, application_name FROM pipeline_events WHERE run_id = 'run-spid-1'")
	if err := row.Scan(&userName, &osUserName, &dbUserName, &spid, &sspid, &appName); err != nil {
		t.Fatalf("failed scanning pipeline_events row: %v", err)
	}

	if userName != "os_alice" {
		t.Errorf("expected user_name 'os_alice', got %q", userName)
	}
	if osUserName != "os_alice" {
		t.Errorf("expected os_user_name 'os_alice', got %q", osUserName)
	}
	if dbUserName != "etl_db_user" {
		t.Errorf("expected db_user_name 'etl_db_user', got %q", dbUserName)
	}
	if spid != 12345 {
		t.Errorf("expected spid 12345, got %d", spid)
	}
	if sspid != 12345 {
		t.Errorf("expected sspid 12345, got %d", sspid)
	}
	if appName != "flow_test_app" {
		t.Errorf("expected application_name 'flow_test_app', got %q", appName)
	}
}

func TestLogRunSummaryToDB_IncludesSPIDAndDBUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE pipeline_runs (
			run_id VARCHAR(64) PRIMARY KEY,
			file_path VARCHAR(255) NOT NULL,
			config_path VARCHAR(255),
			status VARCHAR(32) NOT NULL,
			started_at DATETIME NOT NULL,
			finished_at DATETIME NOT NULL,
			duration_ms BIGINT NOT NULL,
			task_count INT NOT NULL,
			user_name VARCHAR(256),
			os_user_name VARCHAR(256),
			db_user_name VARCHAR(256),
			spid BIGINT,
			sspid BIGINT,
			application_name VARCHAR(256),
			hostname VARCHAR(256),
			options_path VARCHAR(2048),
			error_class VARCHAR(128),
			error_message TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create pipeline_runs table: %v", err)
	}

	now := time.Now().UTC()
	rec := RunSummaryRecord{
		RunID:            "run-summary-test",
		FilePath:         "pipeline.xml",
		Status:           "succeeded",
		StartedAt:        now,
		FinishedAt:       now.Add(time.Second),
		Duration:         time.Second,
		TaskCount:        3,
		UserName:         "alice_os",
		OSUserName:       "alice_os",
		DatabaseUserName: "app_db_user",
		DBUserName:       "app_db_user",
		SPID:             98765,
		ApplicationName:  "flow_worker_app",
		Hostname:         "srv01",
	}

	ctx := context.Background()
	if err := LogRunSummaryToDB(ctx, db, "sqlite", rec, false); err != nil {
		t.Fatalf("failed LogRunSummaryToDB: %v", err)
	}

	var (
		userName   string
		osUserName string
		dbUserName string
		spid       int64
		sspid      int64
		appName    string
	)
	row := db.QueryRow("SELECT user_name, os_user_name, db_user_name, spid, sspid, application_name FROM pipeline_runs WHERE run_id = 'run-summary-test'")
	if err := row.Scan(&userName, &osUserName, &dbUserName, &spid, &sspid, &appName); err != nil {
		t.Fatalf("failed scanning pipeline_runs: %v", err)
	}

	if osUserName != "alice_os" {
		t.Errorf("expected os_user_name 'alice_os', got %q", osUserName)
	}
	if dbUserName != "app_db_user" {
		t.Errorf("expected db_user_name 'app_db_user', got %q", dbUserName)
	}
	if spid != 98765 {
		t.Errorf("expected spid 98765, got %d", spid)
	}
	if sspid != 98765 {
		t.Errorf("expected sspid 98765, got %d", sspid)
	}
	if appName != "flow_worker_app" {
		t.Errorf("expected application_name 'flow_worker_app', got %q", appName)
	}
}

func TestDatabaseSink_BackwardCompatibilityWithoutSPID(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	defer db.Close()

	// Legacy table with ONLY the original 15 columns
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
			user_name VARCHAR(256),
			hostname VARCHAR(256),
			options_path VARCHAR(2048),
			error_message TEXT,
			rows_read BIGINT,
			rows_written BIGINT,
			rows_affected BIGINT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create legacy pipeline_events table: %v", err)
	}

	sink := NewDatabaseSink(db, "sqlite")
	sink.SetSessionIdentity(54321, "db_admin")

	ctx := context.Background()
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:       "run-legacy-1",
		ExecutionID: "exec-legacy-1",
		Sequence:    1,
		Type:        flow.EventNodeStarted,
		UserName:    "os_bob",
	})
	if err != nil {
		t.Fatalf("expected legacy insert to succeed without error, got: %v", err)
	}

	var userName string
	err = db.QueryRow("SELECT user_name FROM pipeline_events WHERE run_id = 'run-legacy-1'").Scan(&userName)
	if err != nil {
		t.Fatalf("failed querying legacy row: %v", err)
	}
	if userName != "os_bob" {
		t.Errorf("expected user_name 'os_bob', got %q", userName)
	}
}

func TestDatabaseSink_IndividualConnectionLogging(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	_, err = db.ExecContext(ctx, `
		CREATE TABLE pipeline_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			execution_id TEXT NOT NULL,
			sequence_num INTEGER NOT NULL,
			occurred_at TEXT NOT NULL,
			event_type TEXT NOT NULL,
			node_kind TEXT NOT NULL,
			node_id TEXT NOT NULL,
			status TEXT NOT NULL,
			user_name TEXT,
			os_user_name TEXT,
			db_user_name TEXT,
			spid TEXT,
			application_name TEXT,
			hostname TEXT,
			options_path TEXT,
			error_message TEXT,
			rows_read INTEGER DEFAULT 0,
			rows_written INTEGER DEFAULT 0,
			rows_affected INTEGER DEFAULT 0
		);
		CREATE TABLE pipeline_runs (
			run_id TEXT PRIMARY KEY,
			file_path TEXT NOT NULL,
			config_path TEXT,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL,
			duration_ms INTEGER NOT NULL,
			task_count INTEGER NOT NULL,
			user_name TEXT,
			os_user_name TEXT,
			db_user_name TEXT,
			spid INTEGER,
			sspid INTEGER,
			application_name TEXT,
			hostname TEXT,
			options_path TEXT,
			error_class TEXT,
			error_message TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	sink := NewDatabaseSink(db, "sqlite")
	// Set default log_db identity
	sink.SetSessionIdentity(1000, "log_user", "log_app")

	// Set known session identities for individual databases
	sink.SetDBSessionIdentity("source_db", 2000, "src_user", "src_app")
	sink.SetDBSessionIdentity("target_db", 3000, "tgt_user", "tgt_app")

	// Register AST nodes
	nodes := []flow.PipelineNode{
		{
			Kind: flow.NodeSQL,
			SQL: &flow.SQLElement{
				ID:     "fetch_step",
				DBName: "source_db",
			},
		},
		{
			Kind: flow.NodeSQL,
			SQL: &flow.SQLElement{
				ID:     "insert_step",
				DBName: "target_db",
			},
		},
		{
			Kind: flow.NodeScript,
			Script: &flow.ScriptItem{
				ID: "transform_script",
			},
		},
	}
	sink.RegisterNodes(nodes)

	now := time.Now().UTC()

	// 1. Emit run.started lifecycle event
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-indiv-1",
		Sequence:   1,
		OccurredAt: now,
		Type:       flow.EventRunStarted,
	})
	if err != nil {
		t.Fatalf("failed emitting run.started: %v", err)
	}

	// 2. Emit source_step event
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-indiv-1",
		Sequence:   2,
		OccurredAt: now,
		Type:       flow.EventNodeFinished,
		NodeKind:   "sql",
		NodeID:     "fetch_step",
		NodePath:   "sql[0]",
		Status:     flow.RunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("failed emitting fetch_step: %v", err)
	}

	// 3. Emit insert_step event
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-indiv-1",
		Sequence:   3,
		OccurredAt: now,
		Type:       flow.EventNodeFinished,
		NodeKind:   "sql",
		NodeID:     "insert_step",
		NodePath:   "sql[1]",
		Status:     flow.RunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("failed emitting insert_step: %v", err)
	}

	// 4. Emit non-db script step event
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-indiv-1",
		Sequence:   4,
		OccurredAt: now,
		Type:       flow.EventNodeFinished,
		NodeKind:   "script",
		NodeID:     "transform_script",
		NodePath:   "script[2]",
		Status:     flow.RunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("failed emitting transform_script: %v", err)
	}

	// 5. Emit run.finished lifecycle event
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-indiv-1",
		Sequence:   5,
		OccurredAt: now,
		Type:       flow.EventRunFinished,
		Status:     flow.RunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("failed emitting run.finished: %v", err)
	}

	// Verify events
	type evtRow struct {
		seq     int
		nodeID  string
		dbUser  string
		spid    string
		appName string
	}
	rows, err := db.QueryContext(ctx, "SELECT sequence_num, node_id, db_user_name, spid, application_name FROM pipeline_events ORDER BY sequence_num ASC")
	if err != nil {
		t.Fatalf("failed querying events: %v", err)
	}
	defer rows.Close()

	var recorded []evtRow
	for rows.Next() {
		var r evtRow
		if err := rows.Scan(&r.seq, &r.nodeID, &r.dbUser, &r.spid, &r.appName); err != nil {
			t.Fatalf("failed scanning row: %v", err)
		}
		recorded = append(recorded, r)
	}

	if len(recorded) != 5 {
		t.Fatalf("expected 5 events, got %d", len(recorded))
	}

	// run.started -> uses log_db
	if recorded[0].spid != "1000" || recorded[0].appName != "log_app" || recorded[0].dbUser != "log_user" {
		t.Errorf("event 1 (run.started) expected log_db identity, got spid=%q app=%q user=%q", recorded[0].spid, recorded[0].appName, recorded[0].dbUser)
	}

	// fetch_step -> uses source_db
	if recorded[1].spid != "2000" || recorded[1].appName != "src_app" || recorded[1].dbUser != "src_user" {
		t.Errorf("event 2 (fetch_step) expected source_db identity, got spid=%q app=%q user=%q", recorded[1].spid, recorded[1].appName, recorded[1].dbUser)
	}

	// insert_step -> uses target_db
	if recorded[2].spid != "3000" || recorded[2].appName != "tgt_app" || recorded[2].dbUser != "tgt_user" {
		t.Errorf("event 3 (insert_step) expected target_db identity, got spid=%q app=%q user=%q", recorded[2].spid, recorded[2].appName, recorded[2].dbUser)
	}

	// transform_script -> non-db step, falls back to log_db
	if recorded[3].spid != "1000" || recorded[3].appName != "log_app" || recorded[3].dbUser != "log_user" {
		t.Errorf("event 4 (transform_script) expected log_db fallback, got spid=%q app=%q user=%q", recorded[3].spid, recorded[3].appName, recorded[3].dbUser)
	}

	// run.finished -> uses log_db
	if recorded[4].spid != "1000" || recorded[4].appName != "log_app" || recorded[4].dbUser != "log_user" {
		t.Errorf("event 5 (run.finished) expected log_db identity, got spid=%q app=%q user=%q", recorded[4].spid, recorded[4].appName, recorded[4].dbUser)
	}

	// Verify pipeline_runs: uses log_db session identity
	logSPID, logUser, logApp := sink.SessionIdentity()
	summaryRec := RunSummaryRecord{
		RunID:            "run-indiv-1",
		FilePath:         "pipeline.xml",
		Status:           "succeeded",
		StartedAt:        now,
		FinishedAt:       now.Add(time.Second),
		Duration:         time.Second,
		TaskCount:        3,
		UserName:         "runner",
		OSUserName:       "runner",
		DatabaseUserName: logUser,
		DBUserName:       logUser,
		SPID:             logSPID,
		ApplicationName:  logApp,
		Hostname:         "host1",
	}
	if err := LogRunSummaryToDB(ctx, db, "sqlite", summaryRec, false); err != nil {
		t.Fatalf("failed LogRunSummaryToDB: %v", err)
	}

	var runSPID int64
	var runUser, runApp string
	err = db.QueryRowContext(ctx, "SELECT spid, db_user_name, application_name FROM pipeline_runs WHERE run_id = 'run-indiv-1'").Scan(&runSPID, &runUser, &runApp)
	if err != nil {
		t.Fatalf("failed querying pipeline_runs: %v", err)
	}
	if runSPID != 1000 || runUser != "log_user" || runApp != "log_app" {
		t.Errorf("pipeline_runs expected log_db identity (1000, log_user, log_app), got (%d, %s, %s)", runSPID, runUser, runApp)
	}
}

func TestDatabaseSink_SQLBulkCrossDatabaseConcatenation(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	_, err = db.ExecContext(ctx, `
		CREATE TABLE pipeline_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			execution_id TEXT NOT NULL,
			sequence_num INTEGER NOT NULL,
			occurred_at TEXT NOT NULL,
			event_type TEXT NOT NULL,
			node_kind TEXT NOT NULL,
			node_id TEXT NOT NULL,
			status TEXT NOT NULL,
			user_name TEXT,
			os_user_name TEXT,
			db_user_name TEXT,
			spid TEXT,
			application_name TEXT,
			hostname TEXT,
			options_path TEXT,
			error_message TEXT,
			rows_read INTEGER DEFAULT 0,
			rows_written INTEGER DEFAULT 0,
			rows_affected INTEGER DEFAULT 0
		);
	`)
	if err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	sink := NewDatabaseSink(db, "sqlite")
	sink.SetSessionIdentity(999, "log_user", "log_app")

	sink.SetDBSessionIdentity("source_db", 111, "alice", "ETL_Source")
	sink.SetDBSessionIdentity("target_db", 222, "bob", "ETL_Target")

	nodes := []flow.PipelineNode{
		{
			Kind: flow.NodeSQLBulk,
			SQLBulk: &flow.SQLBulkElement{
				ID:       "cross_db_bulk",
				DBName:   "source_db",
				TargetDB: "target_db",
			},
		},
		{
			Kind: flow.NodeSQLBulk,
			SQLBulk: &flow.SQLBulkElement{
				ID:       "same_db_bulk",
				DBName:   "source_db",
				TargetDB: "source_db",
			},
		},
	}
	sink.RegisterNodes(nodes)

	now := time.Now().UTC()

	// 1. Cross-database transfer
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-bulk-1",
		Sequence:   1,
		OccurredAt: now,
		Type:       flow.EventNodeFinished,
		NodeKind:   "sql_bulk",
		NodeID:     "cross_db_bulk",
		NodePath:   "sql_bulk[0]",
		Status:     flow.RunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("failed emitting cross_db_bulk: %v", err)
	}

	// 2. Same-database transfer
	err = sink.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-bulk-1",
		Sequence:   2,
		OccurredAt: now,
		Type:       flow.EventNodeFinished,
		NodeKind:   "sql_bulk",
		NodeID:     "same_db_bulk",
		NodePath:   "sql_bulk[1]",
		Status:     flow.RunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("failed emitting same_db_bulk: %v", err)
	}

	var spid1, user1, app1 string
	err = db.QueryRowContext(ctx, "SELECT spid, db_user_name, application_name FROM pipeline_events WHERE sequence_num = 1").Scan(&spid1, &user1, &app1)
	if err != nil {
		t.Fatalf("failed querying event 1: %v", err)
	}

	if spid1 != "111->222" {
		t.Errorf("expected cross-db SPID '111->222', got %q", spid1)
	}
	if user1 != "alice->bob" {
		t.Errorf("expected cross-db db_user_name 'alice->bob', got %q", user1)
	}
	if app1 != "ETL_Source->ETL_Target" {
		t.Errorf("expected cross-db application_name 'ETL_Source->ETL_Target', got %q", app1)
	}

	var spid2, user2, app2 string
	err = db.QueryRowContext(ctx, "SELECT spid, db_user_name, application_name FROM pipeline_events WHERE sequence_num = 2").Scan(&spid2, &user2, &app2)
	if err != nil {
		t.Fatalf("failed querying event 2: %v", err)
	}

	if spid2 != "111" {
		t.Errorf("expected same-db SPID '111', got %q", spid2)
	}
	if user2 != "alice" {
		t.Errorf("expected same-db db_user_name 'alice', got %q", user2)
	}
	if app2 != "ETL_Source" {
		t.Errorf("expected same-db application_name 'ETL_Source', got %q", app2)
	}
}

func TestDatabaseSink_NumericSPIDFallback(t *testing.T) {
	sink := &DatabaseSink{
		Driver:        "sqlserver",
		isNumericSPID: true,
	}
	sink.SetSessionIdentity(999, "log_user", "log_app")
	sink.SetDBSessionIdentity("source_db", 111, "alice", "ETL_Source")
	sink.SetDBSessionIdentity("target_db", 222, "bob", "ETL_Target")

	sink.RegisterNodes([]flow.PipelineNode{
		{
			Kind: flow.NodeSQLBulk,
			SQLBulk: &flow.SQLBulkElement{
				ID:       "cross_db_bulk",
				DBName:   "source_db",
				TargetDB: "target_db",
			},
		},
	})

	ctx := context.Background()
	spid, srcSPIDInt, appName, dbUser := sink.resolveEventIdentity(ctx, flow.ExecutionEvent{
		NodeKind: "sql_bulk",
		NodeID:   "cross_db_bulk",
	})

	if spid != "111->222" {
		t.Errorf("expected string SPID '111->222', got %v", spid)
	}
	if srcSPIDInt != 111 {
		t.Errorf("expected numeric fallback SPID 111, got %d", srcSPIDInt)
	}
	if appName != "ETL_Source->ETL_Target" {
		t.Errorf("expected concatenated app 'ETL_Source->ETL_Target', got %q", appName)
	}
	if dbUser != "alice->bob" {
		t.Errorf("expected concatenated dbUser 'alice->bob', got %q", dbUser)
	}
}

func TestDatabaseSink_NodeOutputLogging(t *testing.T) {
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
			user_name VARCHAR(256),
			hostname VARCHAR(256),
			options_path VARCHAR(2048),
			error_message TEXT,
			rows_read BIGINT,
			rows_written BIGINT,
			rows_affected BIGINT,
			node_output TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create pipeline_events table: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Default off: node_output should remain NULL
	sinkOff := NewDatabaseSink(db, "sqlite")
	if sinkOff.LogNodeOutput() {
		t.Fatalf("expected LogNodeOutput to be false by default")
	}
	_ = sinkOff.Emit(ctx, flow.ExecutionEvent{
		RunID:       "run-off",
		ExecutionID: "exec-1",
		Sequence:    1,
		OccurredAt:  now,
		Type:        flow.EventNodeFinished,
		NodeKind:    "cmd",
		NodeID:      "step_1",
		Status:      flow.RunStatusSucceeded,
	})
	if err := sinkOff.RecordNodeOutputs(ctx, []flow.ScriptResult{
		{ScriptID: "step_1", ResultsString: "hello world"},
	}); err != nil {
		t.Fatalf("RecordNodeOutputs (off) returned error: %v", err)
	}

	var outOff sql.NullString
	if err := db.QueryRow("SELECT node_output FROM pipeline_events WHERE run_id = 'run-off' AND sequence_num = 1").Scan(&outOff); err != nil {
		t.Fatalf("failed querying run-off row: %v", err)
	}
	if outOff.Valid {
		t.Errorf("expected node_output to be NULL when LogNodeOutput is false, got %q", outOff.String)
	}

	// 2. Enabled: node_output populated ONLY on node.finished with credential redaction
	sinkOn := NewDatabaseSink(db, "sqlite")
	sinkOn.SetLogNodeOutput(true)
	if !sinkOn.LogNodeOutput() {
		t.Fatalf("expected LogNodeOutput to be true after SetLogNodeOutput(true)")
	}

	_ = sinkOn.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-on",
		Sequence:   1,
		OccurredAt: now,
		Type:       flow.EventRunStarted,
	})
	_ = sinkOn.Emit(ctx, flow.ExecutionEvent{
		RunID:       "run-on",
		ExecutionID: "exec-step-a",
		Sequence:    2,
		OccurredAt:  now,
		Type:        flow.EventNodeStarted,
		NodeKind:    "cmd",
		NodeID:      "step_a",
	})
	_ = sinkOn.Emit(ctx, flow.ExecutionEvent{
		RunID:       "run-on",
		ExecutionID: "exec-step-a",
		Sequence:    3,
		OccurredAt:  now,
		Type:        flow.EventNodeFinished,
		NodeKind:    "cmd",
		NodeID:      "step_a",
		Status:      flow.RunStatusSucceeded,
	})
	_ = sinkOn.Emit(ctx, flow.ExecutionEvent{
		RunID:      "run-on",
		Sequence:   4,
		OccurredAt: now,
		Type:       flow.EventRunFinished,
		Status:     flow.RunStatusSucceeded,
	})

	if err := sinkOn.RecordNodeOutputs(ctx, []flow.ScriptResult{
		{ScriptID: "step_a", ResultsString: "Connected with password=mySecret123; rows=42"},
	}); err != nil {
		t.Fatalf("RecordNodeOutputs (on) returned error: %v", err)
	}

	var outStarted, outFinished sql.NullString
	if err := db.QueryRow("SELECT node_output FROM pipeline_events WHERE run_id = 'run-on' AND sequence_num = 2").Scan(&outStarted); err != nil {
		t.Fatalf("failed querying node.started row: %v", err)
	}
	if outStarted.Valid {
		t.Errorf("expected node.started node_output to be NULL, got %q", outStarted.String)
	}

	if err := db.QueryRow("SELECT node_output FROM pipeline_events WHERE run_id = 'run-on' AND sequence_num = 3").Scan(&outFinished); err != nil {
		t.Fatalf("failed querying node.finished row: %v", err)
	}
	if !outFinished.Valid {
		t.Fatalf("expected node.finished node_output to be populated, got NULL")
	}
	expected := "Connected with password=[REDACTED]; rows=42"
	if outFinished.String != expected {
		t.Errorf("expected redacted node_output %q, got %q", expected, outFinished.String)
	}

	// 3. Truncation: oversized output is safely capped at maxNodeOutputBytes with suffix
	hugeOutput := strings.Repeat("A", maxNodeOutputBytes+1024)
	redactedHuge := redactNodeOutput(hugeOutput)
	if len(redactedHuge) != maxNodeOutputBytes {
		t.Errorf("expected truncated output length %d, got %d", maxNodeOutputBytes, len(redactedHuge))
	}
	if !strings.HasSuffix(redactedHuge, nodeOutputTruncationSuffix) {
		t.Errorf("expected truncated output to end with %q", nodeOutputTruncationSuffix)
	}
}



