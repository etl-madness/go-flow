package main

import (
	"os"
	"strings"
	"testing"
)

func TestResolveImportTarget(t *testing.T) {
	tests := []struct {
		name    string
		table   string
		want    string
		wantCol string
		wantErr bool
	}{
		{name: "pipeline default", table: "pipeline", want: "dbo.flow_pipeline_content", wantCol: "PipelineXML"},
		{name: "options table", table: "options", want: "dbo.flow_options_content", wantCol: "OptionsXML"},
		{name: "config table", table: "config", want: "dbo.flow_config_content", wantCol: "ConfigXML"},
		{name: "unknown table", table: "bad", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTable, gotCol, err := resolveImportTarget(tt.table)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for table %q, got nil", tt.table)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveImportTarget(%q) returned error: %v", tt.table, err)
			}
			if gotTable != tt.want {
				t.Fatalf("resolveImportTarget(%q) table = %q, want %q", tt.table, gotTable, tt.want)
			}
			if gotCol != tt.wantCol {
				t.Fatalf("resolveImportTarget(%q) column = %q, want %q", tt.table, gotCol, tt.wantCol)
			}
		})
	}
}

func TestBuildImportQuery(t *testing.T) {
	query := buildImportQuery("dbo.flow_options_content", "OptionsXML")

	if query == "" {
		t.Fatal("buildImportQuery returned empty query")
	}

	wantTokens := []string{
		"MERGE INTO dbo.flow_options_content",
		"OptionsXML",
		"WHEN MATCHED THEN",
		"WHEN NOT MATCHED THEN",
	}

	for _, token := range wantTokens {
		if !strings.Contains(query, token) {
			t.Fatalf("query missing %q: %s", token, query)
		}
	}
}

func TestBuildExportQuery(t *testing.T) {
	tests := []struct {
		table   string
		col     string
		wantSub string
	}{
		{table: "dbo.flow_pipeline_content", col: "PipelineXML", wantSub: "SELECT PipelineXML FROM dbo.flow_pipeline_content WHERE Name = @p1"},
		{table: "dbo.flow_options_content", col: "OptionsXML", wantSub: "SELECT OptionsXML FROM dbo.flow_options_content WHERE Name = @p1"},
		{table: "dbo.flow_config_content", col: "ConfigXML", wantSub: "SELECT ConfigXML FROM dbo.flow_config_content WHERE Name = @p1"},
	}

	for _, tt := range tests {
		query := buildExportQuery(tt.table, tt.col)
		if query != tt.wantSub {
			t.Fatalf("expected %q, got %q", tt.wantSub, query)
		}
	}
}

func TestResolveSecureKey(t *testing.T) {
	os.Setenv("FLOW_SECURE_KEY", "ImporterEnvKey")
	defer os.Unsetenv("FLOW_SECURE_KEY")

	if got := resolveSecureKey("ExplicitKey"); got != "ExplicitKey" {
		t.Fatalf("expected ExplicitKey, got %s", got)
	}

	if got := resolveSecureKey(""); got != "ImporterEnvKey" {
		t.Fatalf("expected ImporterEnvKey, got %s", got)
	}

	// Test resolveSecureKeyWithFile
	tmpKeyFile, err := os.CreateTemp("", "importer_key_*.txt")
	if err != nil {
		t.Fatalf("failed creating temp key file: %v", err)
	}
	defer os.Remove(tmpKeyFile.Name())
	tmpKeyFile.WriteString("FileBasedSecretKey\n")
	tmpKeyFile.Close()

	if got := resolveSecureKeyWithFile("", tmpKeyFile.Name()); got != "FileBasedSecretKey" {
		t.Fatalf("expected FileBasedSecretKey from file, got %s", got)
	}

	// Explicit CLI key overrides file key
	if got := resolveSecureKeyWithFile("OverrideKey", tmpKeyFile.Name()); got != "OverrideKey" {
		t.Fatalf("expected OverrideKey, got %s", got)
	}
}
