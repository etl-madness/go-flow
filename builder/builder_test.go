package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	cat := GetCatalog()
	if len(cat.Components) == 0 {
		t.Fatal("expected non-empty component catalog")
	}

	meta := cat.GetComponentByType("sql")
	if meta == nil {
		t.Fatal("expected sql component to exist in catalog")
	}
	if meta.Tag != "sql" || meta.Section != "flow" {
		t.Fatalf("unexpected metadata for sql component: %+v", meta)
	}

	varMeta := cat.GetComponentByType("variable")
	if varMeta == nil || varMeta.Section != "variables" {
		t.Fatalf("unexpected metadata for variable component: %+v", varMeta)
	}
}

func TestStorageAndSequencing(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_builder.db")

	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	sc, err := storage.CreateScript("etl_test", "Test pipeline")
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	node1, err := storage.AddNode(sc.ID, "flow", "sql", map[string]string{"id": "Step1", "db": "local_sqlite"}, "SELECT 1;")
	if err != nil {
		t.Fatalf("failed to add node 1: %v", err)
	}
	if node1.SequenceOrder != 1 {
		t.Fatalf("expected node1 sequence order 1, got %d", node1.SequenceOrder)
	}

	node2, err := storage.AddNode(sc.ID, "flow", "sql", map[string]string{"id": "Step2", "db": "local_sqlite"}, "SELECT 2;")
	if err != nil {
		t.Fatalf("failed to add node 2: %v", err)
	}
	if node2.SequenceOrder != 2 {
		t.Fatalf("expected node2 sequence order 2, got %d", node2.SequenceOrder)
	}

	// Move node 2 up
	if err := storage.MoveNode(node2.ID, "up"); err != nil {
		t.Fatalf("failed to move node up: %v", err)
	}

	flowNodes, err := storage.GetNodes(sc.ID, "flow")
	if err != nil {
		t.Fatalf("failed to get flow nodes: %v", err)
	}
	if len(flowNodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(flowNodes))
	}
	if flowNodes[0].ID != node2.ID {
		t.Fatalf("expected node2 to be first after move up, got node %d", flowNodes[0].ID)
	}

	// Reorder
	if err := storage.ReorderNodes(sc.ID, "flow", []int64{node1.ID, node2.ID}); err != nil {
		t.Fatalf("failed to reorder nodes: %v", err)
	}
	flowNodes, _ = storage.GetNodes(sc.ID, "flow")
	if flowNodes[0].ID != node1.ID {
		t.Fatalf("expected node1 to be first after reorder")
	}

	// Test GetNode and UpdateNode
	fetched, err := storage.GetNode(node1.ID)
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	if fetched.Attributes["id"] != "Step1" {
		t.Fatalf("expected attribute id 'Step1', got '%s'", fetched.Attributes["id"])
	}

	err = storage.UpdateNode(node1.ID, map[string]string{
		"id":     "Step1Renamed",
		"db":     "local_sqlite",
		"action": "query",
		"into":   "QueryResult",
	}, "SELECT * FROM logs;")
	if err != nil {
		t.Fatalf("failed to update node: %v", err)
	}

	updated, err := storage.GetNode(node1.ID)
	if err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}
	if updated.Attributes["id"] != "Step1Renamed" || updated.Attributes["action"] != "query" || updated.Attributes["into"] != "QueryResult" {
		t.Fatalf("updated attributes mismatch: %+v", updated.Attributes)
	}
	if updated.ContentText != "SELECT * FROM logs;" {
		t.Fatalf("updated content mismatch: %s", updated.ContentText)
	}

	// Delete
	if err := storage.DeleteNode(node2.ID); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}
	flowNodes, _ = storage.GetNodes(sc.ID, "flow")
	if len(flowNodes) != 1 {
		t.Fatalf("expected 1 node after delete, got %d", len(flowNodes))
	}
}

func TestGenerateXML(t *testing.T) {
	varNodes := []PipelineNode{
		{
			NodeType:   "variable",
			Attributes: map[string]string{"name": "BatchID", "value": "123", "type": "int"},
		},
	}
	dbNodes := []PipelineNode{
		{
			NodeType:   "database",
			Attributes: map[string]string{"name": "db_main", "driver": "sqlite", "connection_string": ":memory:"},
		},
	}
	flowNodes := []PipelineNode{
		{
			NodeType:    "sql",
			Attributes:  map[string]string{"id": "InsertBatch", "db": "db_main"},
			ContentText: "INSERT INTO batches (id) VALUES ({{.BatchID}});",
		},
	}

	xml := GenerateXML("sample_pipeline", varNodes, dbNodes, nil, flowNodes)

	if !strings.Contains(xml, `name="sample_pipeline"`) {
		t.Errorf("expected pipeline name attribute in xml: %s", xml)
	}
	if !strings.Contains(xml, `<variables>`) || !strings.Contains(xml, `name="BatchID"`) {
		t.Errorf("expected variable tag in xml: %s", xml)
	}
	if !strings.Contains(xml, `<databases>`) || !strings.Contains(xml, `name="db_main"`) {
		t.Errorf("expected database tag in xml: %s", xml)
	}
	if !strings.Contains(xml, `<flow>`) || !strings.Contains(xml, `INSERT INTO batches`) {
		t.Errorf("expected flow sql body in xml: %s", xml)
	}
}

func TestConfigAndOptionsDrafts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_config.db")

	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer storage.Close()

	cfg := storage.GetOrCreateDefaultConfig()
	if !strings.Contains(cfg, "<config>") {
		t.Fatalf("expected default config to contain <config>, got: %s", cfg)
	}

	opt := storage.GetOrCreateDefaultOptions()
	if !strings.Contains(opt, "<options") {
		t.Fatalf("expected default options to contain <options, got: %s", opt)
	}

	newCfg := "<config><custom>true</custom></config>"
	if err := storage.SaveConfigFile("CONFIG.xml", newCfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	if storage.GetOrCreateDefaultConfig() != newCfg {
		t.Fatalf("expected updated config")
	}

	_ = os.Remove(dbPath)
}