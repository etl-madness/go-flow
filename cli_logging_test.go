package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLILogDatabaseAndStreaming(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "logs.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()
	schema := `
CREATE TABLE pipeline_events (
 run_id TEXT, execution_id TEXT, sequence_num INTEGER, occurred_at TEXT, event_type TEXT,
 node_kind TEXT, node_id TEXT, status TEXT, user_name TEXT, hostname TEXT, options_path TEXT,
 error_message TEXT, rows_read INTEGER, rows_written INTEGER, rows_affected INTEGER, node_output TEXT
);
CREATE TABLE pipeline_runs (
 run_id TEXT PRIMARY KEY, file_path TEXT, config_path TEXT, status TEXT, started_at TEXT,
 finished_at TEXT, duration_ms INTEGER, task_count INTEGER, user_name TEXT, hostname TEXT,
 options_path TEXT, error_class TEXT, error_message TEXT
);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	pipeline := filepath.Join(dir, "pipeline.xml")
	xml := fmt.Sprintf(`<pipeline><databases>
<database name="log_db" driver="sqlite" connection_string="%s"/>
<database name="main_db" driver="sqlite" connection_string="%s"/>
</databases><flow><sql id="create" db="main_db">CREATE TABLE IF NOT EXISTS work (value TEXT)</sql></flow></pipeline>`,
		filepath.ToSlash(dbPath), filepath.ToSlash(filepath.Join(dir, "work.db")))
	if err := os.WriteFile(pipeline, []byte(xml), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-file", pipeline, "-format", "stream"},
		{"-file", pipeline, "-format", "stream", "-node-output", "-debug"},
		{"-file", pipeline, "-format", "json", "-node-output"},
	} {
		runFlowCLI(t, args...)
	}
	var runs int
	if err := db.QueryRow("SELECT count(*) FROM pipeline_runs WHERE status = 'succeeded'").Scan(&runs); err != nil || runs != 3 {
		t.Fatalf("run summaries = %d, %v; want 3", runs, err)
	}
	var events int
	if err := db.QueryRow("SELECT count(*) FROM pipeline_events").Scan(&events); err != nil || events == 0 {
		t.Fatalf("events = %d, %v; want nonzero", events, err)
	}
	var finished int
	if err := db.QueryRow("SELECT count(*) FROM pipeline_events WHERE node_id = 'create'").Scan(&finished); err != nil || finished == 0 {
		t.Fatalf("node events = %d, %v; want nonzero", finished, err)
	}
}

func TestCLIExecutesSQLitePipelineWithOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "data.db"))
	pipeline := filepath.Join(dir, "pipeline.xml")
	config := filepath.Join(dir, "config.xml")
	options := filepath.Join(dir, "options.xml")
	source := fmt.Sprintf(`<pipeline>
<variables><variable name="value" type="string" value="base"/></variables>
<databases><database name="main_db" driver="sqlite" connection_string="%s"/></databases>
<flow><sql id="create" db="main_db">CREATE TABLE items (value TEXT)</sql>
<sql id="insert" db="main_db">INSERT INTO items VALUES ('{{value}}')</sql></flow>
</pipeline>`, path)
	for filename, content := range map[string]string{
		pipeline: source,
		config:   `<pipeline><variables><variable name="value" type="string" value="config"/></variables></pipeline>`,
		options:  `<flow_cli_options><options><format>text</format></options></flow_cli_options>`,
	} {
		if err := os.WriteFile(filename, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	runFlowCLI(t, "-file", pipeline, "-config", config, "-options", options, "-vars", "value=override", "-format", "json")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow("SELECT value FROM items").Scan(&value); err != nil || !strings.EqualFold(value, "override") {
		t.Fatalf("inserted value = %q, err=%v; want override", value, err)
	}
}
