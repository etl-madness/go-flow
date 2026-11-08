package main

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/etl-madness/flow"
)

func TestNodeIdentityForKinds(t *testing.T) {
	for _, tc := range []struct {
		kind flow.NodeKind
		want string
	}{
		{flow.NodeKV, "kv"}, {flow.NodeKVBulk, "kv_bulk"}, {flow.NodeAssert, "assert"},
		{flow.NodeSQL, "sql"}, {flow.NodeSQLBulk, "sql_bulk"}, {flow.NodeScript, "script"},
		{flow.NodeHTTPClient, "http_client"}, {flow.NodeTemplate, "template"}, {flow.NodeHtmlTemplate, "template_html"},
		{flow.NodeFileSave, "file_save"}, {flow.NodeFileRead, "file_read"}, {flow.NodeExcelRead, "excel_read"},
		{flow.NodeExcelWrite, "excel_write"}, {flow.NodeXMLXPath, "xml_xpath"}, {flow.NodeJSONPath, "json_path"},
		{flow.NodeYAMLPath, "yaml_path"}, {flow.NodeGroup, "group"}, {flow.NodeParallel, "parallel"},
		{flow.NodeIf, "if"}, {flow.NodeForEach, "foreach"}, {flow.NodeWhile, "while"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			got, id := nodeIdentityFromAST(flow.PipelineNode{Kind: tc.kind})
			if got != tc.want || id != "" {
				t.Fatalf("identity = %q, %q, want %q and empty id", got, id, tc.want)
			}
		})
	}
}

func TestSessionIdentityDriverErrors(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, driver := range []string{"sqlserver", "postgres", "mysql", "oracle"} {
		t.Run(driver, func(t *testing.T) {
			spid, _, app, err := FetchDatabaseSessionIdentity(context.Background(), db, driver)
			if err != nil || spid == 0 || app == "" {
				t.Fatalf("fallback identity: spid=%d app=%q err=%v", spid, app, err)
			}
		})
	}
	if _, _, app, err := FetchDatabaseSessionIdentity(context.Background(), nil, "sqlite"); err == nil || app == "" {
		t.Fatalf("nil querier: app=%q err=%v", app, err)
	}
}

func TestResolveConnectionStringOverrides(t *testing.T) {
	variables := []flow.VariableConfig{{Name: "server", Value: "host"}, {Name: "dsn", Value: "{{server}}/database"}}
	got := resolveConnectionString("{{dsn}}/{{missing}}", variables, "server=override,invalid")
	if got != "override/database/{{missing}}" {
		t.Fatalf("resolved = %q", got)
	}
}

func TestEventColumnFilteringAndNormalization(t *testing.T) {
	got := filterAvailableEventCols([]string{" RUN_ID ", "bad", "STATUS"})
	if strings.Join(got, ",") != "run_id,status" {
		t.Fatalf("filtered = %v", got)
	}
	if got := filterAvailableEventCols([]string{"bad"}); len(got) != len(defaultEventCols) {
		t.Fatalf("expected default columns, got %v", got)
	}
	if got := formatLineReturns("a\r\nb\\r\\nc\r"); got != "a\nb\nc" {
		t.Fatalf("normalized = %q", got)
	}
}
