package main

import (
	"database/sql"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/etl-madness/go-flow/pkg/flowcrypto"
)

func runFlowCLI(t *testing.T, args ...string) {
	t.Helper()
	oldFlags, oldArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })
	flag.CommandLine = flag.NewFlagSet("flow", flag.ContinueOnError)
	os.Args = append([]string{"flow"}, args...)
	main()
}

func TestCLIBuilderImportExport(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "builder.db")
	t.Setenv("FLOW_SECURE_KEY", "test-passphrase")
	for _, kind := range []string{"options", "config", "pipeline"} {
		t.Run(kind, func(t *testing.T) {
			source := filepath.Join(dir, kind+".xml")
			xml := "<pipeline><flow/></pipeline>"
			if kind == "options" {
				xml = "<options></options>"
			} else if kind == "config" {
				xml = "<pipeline><variables/></pipeline>"
			}
			if err := os.WriteFile(source, []byte(xml), 0600); err != nil {
				t.Fatal(err)
			}
			runFlowCLI(t, "-builder-db", db, "-import-file", source, "-import-type", kind, "-import-name", "example")
			export := filepath.Join(dir, kind+".export")
			runFlowCLI(t, "-builder-db", db, "-export-file", export, "-export-type", kind, "-export-name", "example", "-encrypted")
			encrypted, err := os.ReadFile(export)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := flowcrypto.DecryptArmored(string(encrypted), "test-passphrase")
			if err != nil {
				t.Fatal(err)
			}
			if kind != "pipeline" && string(plain) != xml {
				t.Fatalf("export = %q, want %q", plain, xml)
			}
			if kind == "pipeline" && !strings.Contains(string(plain), "<pipeline") {
				t.Fatalf("unexpected pipeline export %q", plain)
			}
		})
	}
}

func TestDatabaseImportExportSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()
	for _, stmt := range []string{
		"CREATE TABLE flow_pipeline_content (Name TEXT PRIMARY KEY, Description TEXT, PipelineXML TEXT)",
		"CREATE TABLE flow_options_content (Name TEXT PRIMARY KEY, Description TEXT, OptionsXML TEXT)",
		"CREATE TABLE flow_config_content (Name TEXT PRIMARY KEY, Description TEXT, ConfigXML TEXT)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	dsn := "sql://sqlite@" + path
	for _, kind := range []string{"pipeline", "options", "config"} {
		t.Run(kind, func(t *testing.T) {
			for _, content := range []string{"first", "updated"} {
				if err := importToDatabase(dsn, kind, "example", "description", content); err != nil {
					t.Fatal(err)
				}
				got, err := exportFromDatabase(dsn, kind, "example")
				if err != nil || got != content {
					t.Fatalf("export = %q, %v; want %q", got, err, content)
				}
			}
		})
	}
	if _, err := exportFromDatabase(dsn, "invalid", "example"); err == nil {
		t.Fatal("expected invalid item type error")
	}
	if _, err := exportFromDatabase(dsn, "pipeline", "missing"); err == nil {
		t.Fatal("expected missing item error")
	}
	input := filepath.Join(t.TempDir(), "config.xml")
	if err := os.WriteFile(input, []byte("<config/>"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLOW_SECURE_KEY", "database-test-key")
	runFlowCLI(t, "-dsn", dsn, "-import-file", input, "-import-type", "config", "-import-name", "cli", "-encrypted")
	stored, err := exportFromDatabase(dsn, "config", "cli")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := flowcrypto.DecryptArmored(stored, "database-test-key")
	if err != nil || string(plain) != "<config/>" {
		t.Fatalf("stored content = %q, %v", plain, err)
	}
	output := filepath.Join(t.TempDir(), "output.xml")
	runFlowCLI(t, "-dsn", dsn, "-export-file", output, "-export-type", "config", "-export-name", "cli")
	got, err := os.ReadFile(output)
	if err != nil || string(got) != stored {
		t.Fatalf("export file = %q, %v; want stored content", got, err)
	}
}

func TestResolveSecureKeyEnvironmentFallback(t *testing.T) {
	t.Setenv("FLOW_SECURE_KEY", "flow-key")
	t.Setenv("SECURE_KEY", "fallback-key")
	path := filepath.Join(t.TempDir(), "missing-key")
	if got := resolveSecureKeyWithFile("", path); got != "flow-key" {
		t.Fatalf("missing key file fallback = %q, want flow-key", got)
	}
	t.Setenv("FLOW_SECURE_KEY", "")
	if got := resolveSecureKey(""); got != "fallback-key" {
		t.Fatalf("fallback key = %q", got)
	}
}

func TestCLIExecuteEmptyPipelineFormats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.xml")
	if err := os.WriteFile(path, []byte("<pipeline><flow/></pipeline>"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLOW_SECURE_KEY", "")
	t.Setenv("SECURE_KEY", "")
	for _, format := range []string{"json", "jsonpretty", "text", "markdown", "csv", "stream"} {
		t.Run(format, func(t *testing.T) {
			output, err := os.CreateTemp(t.TempDir(), "output")
			if err != nil {
				t.Fatal(err)
			}
			oldStdout := os.Stdout
			os.Stdout = output
			t.Cleanup(func() { os.Stdout = oldStdout; output.Close() })
			runFlowCLI(t, "-file", path, "-format", format, "-vars", "count=3,enabled=true")
			if err := output.Close(); err != nil {
				t.Fatal(err)
			}
			os.Stdout = oldStdout
			data, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if !strings.Contains(text, "Pipeline Start Time:") || !strings.Contains(text, "Pipeline End Time:") {
				t.Fatalf("missing execution timestamps in %q", text)
			}
			if format == "markdown" && !strings.Contains(text, "| Script ID | Return Code |") {
				t.Fatalf("missing markdown header in %q", text)
			}
			if format == "stream" && !strings.Contains(text, "utc_runtime,file,config,status") {
				t.Fatalf("missing stream summary in %q", text)
			}
		})
	}
}
