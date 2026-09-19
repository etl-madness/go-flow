package main

import (
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
