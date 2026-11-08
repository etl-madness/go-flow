package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/etl-madness/flow"
)

func TestNodeIdentityForPayloads(t *testing.T) {
	tests := []struct {
		node flow.PipelineNode
		kind string
	}{
		{flow.PipelineNode{Kind: flow.NodeKV, KV: &flow.KVElement{ID: "node"}}, "kv"},
		{flow.PipelineNode{Kind: flow.NodeKVBulk, KVBulk: &flow.KVBulkElement{ID: "node"}}, "kv_bulk"},
		{flow.PipelineNode{Kind: flow.NodeAssert, Assert: &flow.AssertElement{ID: "node"}}, "assert"},
		{flow.PipelineNode{Kind: flow.NodeSQL, SQL: &flow.SQLElement{ID: "node"}}, "sql"},
		{flow.PipelineNode{Kind: flow.NodeSQLBulk, SQLBulk: &flow.SQLBulkElement{ID: "node"}}, "sql_bulk"},
		{flow.PipelineNode{Kind: flow.NodeScript, Script: &flow.ScriptItem{ID: "node"}}, "script"},
		{flow.PipelineNode{Kind: flow.NodeHTTPClient, HTTPClient: &flow.HTTPClientElement{ID: "node"}}, "http_client"},
		{flow.PipelineNode{Kind: flow.NodeTemplate, Template: &flow.TemplateElement{ID: "node"}}, "template"},
		{flow.PipelineNode{Kind: flow.NodeHtmlTemplate, HtmlTemplate: &flow.HtmlTemplateElement{ID: "node"}}, "template_html"},
		{flow.PipelineNode{Kind: flow.NodeFileSave, FileSave: &flow.FileSaveElement{ID: "node"}}, "file_save"},
		{flow.PipelineNode{Kind: flow.NodeFileRead, FileRead: &flow.FileReadElement{ID: "node"}}, "file_read"},
		{flow.PipelineNode{Kind: flow.NodeExcelRead, ExcelRead: &flow.ExcelReadElement{ID: "node"}}, "excel_read"},
		{flow.PipelineNode{Kind: flow.NodeExcelWrite, ExcelWrite: &flow.ExcelWriteElement{ID: "node"}}, "excel_write"},
		{flow.PipelineNode{Kind: flow.NodeXMLXPath, XmlXPath: &flow.XmlXPathElement{ID: "node"}}, "xml_xpath"},
		{flow.PipelineNode{Kind: flow.NodeJSONPath, JsonPath: &flow.JsonPathElement{ID: "node"}}, "json_path"},
		{flow.PipelineNode{Kind: flow.NodeYAMLPath, YamlPath: &flow.YamlPathElement{ID: "node"}}, "yaml_path"},
	}
	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			kind, id := nodeIdentityFromAST(tc.node)
			if kind != tc.kind || id != "node" {
				t.Fatalf("identity = %q, %q", kind, id)
			}
		})
	}
}

func TestDatabaseSinkResolvesRegisteredDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "log.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sink := NewDatabaseSink(db, "sqlite")
	registry := flow.NewRegistry()
	if err := registry.InitDatabases([]flow.DatabaseConfig{{Name: "source", Driver: "sqlite", ConnectionString: ":memory:"}}); err != nil {
		t.Fatal(err)
	}
	defer registry.CloseDatabases()
	registry.SetVar("db", "source")
	sink.SetRegistry(registry)
	sink.RegisterNodeTarget("step", NodeDBTarget{NodeKind: "sql", DBName: "{{db}}"})
	if got := sink.resolveDBName("{{db}}"); got != "source" {
		t.Fatalf("resolved database = %q", got)
	}
	identity, err := sink.getOrFetchSessionIdentity(context.Background(), "source")
	if err != nil || identity.SPID == 0 || identity.ApplicationName == "" {
		t.Fatalf("identity = %+v, %v", identity, err)
	}
	cached, err := sink.getOrFetchSessionIdentity(context.Background(), "source")
	if err != nil || cached != identity {
		t.Fatalf("cached identity = %+v, %v", cached, err)
	}
	spid, _, app, _ := sink.resolveEventIdentity(context.Background(), flow.ExecutionEvent{NodeID: "step"})
	if spid != identity.SPID || app != identity.ApplicationName {
		t.Fatalf("event identity = %v, %q; want %+v", spid, app, identity)
	}
	if _, err := sink.getOrFetchSessionIdentity(context.Background(), "missing"); err == nil {
		t.Fatal("expected missing database error")
	}
}
