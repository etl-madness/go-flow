package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/etl-madness/flow"
)

func TestRegisterDatabaseNodeTargets(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "log.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sink := NewDatabaseSink(db, "sqlite")
	nodes := []flow.PipelineNode{
		{Kind: flow.NodeSQL, SQL: &flow.SQLElement{ID: "query", DBName: "source"}},
		{Kind: flow.NodeSQLBulk, SQLBulk: &flow.SQLBulkElement{ID: "bulk", DBName: "source"}},
		{Kind: flow.NodeKV, KV: &flow.KVElement{ID: "get", DBName: "cache"}},
		{Kind: flow.NodeKVBulk, KVBulk: &flow.KVBulkElement{ID: "copy", DBName: "cache"}},
		{Kind: flow.NodeGroup, GroupID: "transaction", Transaction: true, DBName: "source", Children: []flow.PipelineNode{
			{Kind: flow.NodeSQL, SQL: &flow.SQLElement{ID: "nested", DBName: "source"}},
		}},
	}
	sink.RegisterNodes(nodes)
	for id, want := range map[string]NodeDBTarget{
		"query":       {NodeKind: "sql", DBName: "source"},
		"bulk":        {NodeKind: "sql_bulk", DBName: "source", TargetDB: "source"},
		"get":         {NodeKind: "kv", DBName: "cache"},
		"copy":        {NodeKind: "kv_bulk", DBName: "cache"},
		"transaction": {NodeKind: "group", DBName: "source"},
		"nested":      {NodeKind: "sql", DBName: "source"},
	} {
		if got, ok := sink.nodeTargets[id]; !ok || got != want {
			t.Errorf("target %s = %+v (%t), want %+v", id, got, ok, want)
		}
	}
	if got := sink.nodeTargets["group[4]/sql[0]"]; got.DBName != "source" {
		t.Fatalf("nested path target = %+v", got)
	}
}
