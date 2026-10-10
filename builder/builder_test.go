package builder

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/etl-madness/flow"
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
		"id":   "Step1Renamed",
		"db":   "local_sqlite",
		"into": "QueryResult",
	}, "SELECT * FROM logs;")
	if err != nil {
		t.Fatalf("failed to update node: %v", err)
	}

	updated, err := storage.GetNode(node1.ID)
	if err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}
	if updated.Attributes["id"] != "Step1Renamed" || updated.Attributes["into"] != "QueryResult" {
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

	if !strings.Contains(xml, `<pipeline`) {
		t.Errorf("expected pipeline root in xml: %s", xml)
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

func TestCatalogUpdatedFlowFeatures(t *testing.T) {
	cat := GetCatalog()

	// 1. database workload
	dbMeta := cat.GetComponentByType("database")
	if dbMeta == nil {
		t.Fatal("missing database component in catalog")
	}
	var hasWorkload bool
	for _, f := range dbMeta.Fields {
		if f.Name == "workload" {
			hasWorkload = true
			if len(f.Options) < 4 {
				t.Fatalf("expected workload options, got: %+v", f.Options)
			}
		}
	}
	if !hasWorkload {
		t.Errorf("expected database component to have workload field")
	}

	// 2. group db and timeout
	grpMeta := cat.GetComponentByType("group")
	if grpMeta == nil {
		t.Fatal("missing group component in catalog")
	}
	var grpHasDB, grpHasTimeout bool
	for _, f := range grpMeta.Fields {
		if f.Name == "db" {
			grpHasDB = true
		}
		if f.Name == "timeout" {
			grpHasTimeout = true
		}
	}
	if !grpHasDB || !grpHasTimeout {
		t.Errorf("expected group to have db and timeout fields, got db=%v timeout=%v", grpHasDB, grpHasTimeout)
	}

	// 3. foreach streaming / query content
	feMeta := cat.GetComponentByType("foreach")
	if feMeta == nil {
		t.Fatal("missing foreach component in catalog")
	}
	if !feMeta.HasContent {
		t.Errorf("expected foreach to have HasContent: true for query body")
	}
	var feHasDB, feHasStream, feHasBuffer, feHasMode bool
	for _, f := range feMeta.Fields {
		switch f.Name {
		case "db":
			feHasDB = true
		case "stream":
			feHasStream = true
		case "buffer":
			feHasBuffer = true
		case "mode":
			feHasMode = true
		}
	}
	if !feHasDB || !feHasStream || !feHasBuffer || !feHasMode {
		t.Errorf("foreach missing required fields: db=%v stream=%v buffer=%v mode=%v", feHasDB, feHasStream, feHasBuffer, feHasMode)
	}

	// 4. sql timeout and output_var
	sqlMeta := cat.GetComponentByType("sql")
	if sqlMeta == nil {
		t.Fatal("missing sql component in catalog")
	}
	var sqlHasTimeout, sqlHasOutputVar bool
	for _, f := range sqlMeta.Fields {
		if f.Name == "timeout" {
			sqlHasTimeout = true
		}
		if f.Name == "output_var" {
			sqlHasOutputVar = true
		}
	}
	if !sqlHasTimeout || !sqlHasOutputVar {
		t.Errorf("expected sql to have timeout and output_var, got timeout=%v output_var=%v", sqlHasTimeout, sqlHasOutputVar)
	}

	// 5. sql_bulk target_table, target_db, tablock, timeout
	bulkMeta := cat.GetComponentByType("sql_bulk")
	if bulkMeta == nil {
		t.Fatal("missing sql_bulk component in catalog")
	}
	if !bulkMeta.HasContent {
		t.Errorf("expected sql_bulk to have HasContent: true for source query")
	}
	var bulkHasTargetTable, bulkHasTargetDB, bulkHasTablock, bulkHasTimeout bool
	for _, f := range bulkMeta.Fields {
		switch f.Name {
		case "target_table":
			bulkHasTargetTable = true
		case "target_db":
			bulkHasTargetDB = true
		case "tablock":
			bulkHasTablock = true
		case "timeout":
			bulkHasTimeout = true
		}
	}
	if !bulkHasTargetTable || !bulkHasTargetDB || !bulkHasTablock || !bulkHasTimeout {
		t.Errorf("sql_bulk missing fields: target_table=%v target_db=%v tablock=%v timeout=%v",
			bulkHasTargetTable, bulkHasTargetDB, bulkHasTablock, bulkHasTimeout)
	}

	// 6. excel_write
	ewMeta := cat.GetComponentByType("excel_write")
	if ewMeta == nil {
		t.Fatal("missing excel_write component in catalog")
	}
	if !ewMeta.HasContent {
		t.Errorf("expected excel_write to have HasContent: true for export query")
	}
	var ewHasFile, ewHasDB bool
	for _, f := range ewMeta.Fields {
		if f.Name == "file" {
			ewHasFile = true
		}
		if f.Name == "db" {
			ewHasDB = true
		}
	}
	if !ewHasFile || !ewHasDB {
		t.Errorf("excel_write missing file or db: file=%v db=%v", ewHasFile, ewHasDB)
	}

	// 7. excel_read
	erMeta := cat.GetComponentByType("excel_read")
	if erMeta == nil {
		t.Fatal("missing excel_read component in catalog")
	}
	var erHasFile, erHasHeader, erHasOutVar bool
	for _, f := range erMeta.Fields {
		if f.Name == "file" {
			erHasFile = true
		}
		if f.Name == "header" {
			erHasHeader = true
		}
		if f.Name == "output_var" {
			erHasOutVar = true
		}
	}
	if !erHasFile || !erHasHeader || !erHasOutVar {
		t.Errorf("excel_read missing file/header/output_var: file=%v header=%v output_var=%v", erHasFile, erHasHeader, erHasOutVar)
	}

	// 8. html_template alias
	htmlMeta := cat.GetComponentByType("html_template")
	if htmlMeta == nil {
		t.Fatal("missing html_template component in catalog")
	}
}

func TestGenerateXMLWithNewFeatures(t *testing.T) {
	dbNodes := []PipelineNode{
		{
			NodeType: "database",
			Attributes: map[string]string{
				"name":              "dw_mssql",
				"driver":            "sqlserver",
				"connection_string": "sqlserver://sa:secret@localhost:1433",
				"workload":          "bulk",
			},
		},
	}
	flowNodes := []PipelineNode{
		{
			NodeType: "sql_bulk",
			Attributes: map[string]string{
				"id":           "CopyTxs",
				"db":           "source_pg",
				"target_db":    "dw_mssql",
				"target_table": "raw_txs",
				"batch_size":   "25000",
				"tablock":      "true",
				"timeout":      "30m",
				"output_var":   "COPIED_COUNT",
			},
			ContentText: "SELECT * FROM transactions WHERE created_at >= '2026-01-01';",
		},
		{
			NodeType: "foreach",
			Attributes: map[string]string{
				"id":     "ProcessBatches",
				"db":     "dw_mssql",
				"stream": "true",
			},
			ContentText: "SELECT batch_id FROM batches WHERE status = 'pending';",
		},
		{
			NodeType: "excel_write",
			Attributes: map[string]string{
				"id":    "ExportSummary",
				"file":  "./reports/summary.xlsx",
				"sheet": "Summary",
				"db":    "dw_mssql",
			},
			ContentText: "SELECT * FROM summary_metrics;",
		},
	}

	xml := GenerateXML("bulk_pipeline", nil, dbNodes, nil, flowNodes)

	if !strings.Contains(xml, `workload="bulk"`) {
		t.Errorf("expected workload attribute in xml: %s", xml)
	}
	if !strings.Contains(xml, `<sql_bulk`) || !strings.Contains(xml, `target_table="raw_txs"`) || !strings.Contains(xml, `tablock="true"`) {
		t.Errorf("expected sql_bulk attributes in xml: %s", xml)
	}
	if !strings.Contains(xml, `<foreach`) || !strings.Contains(xml, `stream="true"`) {
		t.Errorf("expected foreach in xml: %s", xml)
	}
	if !strings.Contains(xml, `<excel_write`) || !strings.Contains(xml, `file="./reports/summary.xlsx"`) {
		t.Errorf("expected excel_write in xml: %s", xml)
	}
}

func TestCatalogScriptComponent(t *testing.T) {
	cat := GetCatalog()
	scMeta := cat.GetComponentByType("script")
	if scMeta == nil {
		t.Fatal("missing script component in catalog")
	}

	if scMeta.Tag != "script" || scMeta.Category != "Control Flow" {
		t.Errorf("unexpected script metadata: tag=%s, category=%s", scMeta.Tag, scMeta.Category)
	}

	if !scMeta.HasContent {
		t.Errorf("expected script to have HasContent: true")
	}

	var hasID, hasLang, hasOutVar, hasTimeout, hasVar, hasDesc bool
	var langOptions []string
	for _, f := range scMeta.Fields {
		switch f.Name {
		case "id":
			hasID = true
			if !f.Mandatory {
				t.Errorf("expected script id to be mandatory")
			}
		case "language":
			hasLang = true
			if !f.Mandatory {
				t.Errorf("expected script language to be mandatory")
			}
			langOptions = f.Options
		case "output_var":
			hasOutVar = true
		case "timeout":
			hasTimeout = true
		case "var":
			hasVar = true
		case "description":
			hasDesc = true
		}
	}

	if !hasID || !hasLang || !hasOutVar || !hasTimeout || !hasVar || !hasDesc {
		t.Errorf("script component missing expected fields: id=%v lang=%v outVar=%v timeout=%v var=%v desc=%v",
			hasID, hasLang, hasOutVar, hasTimeout, hasVar, hasDesc)
	}

	if len(langOptions) == 0 {
		t.Errorf("expected script language to have runtime options")
	}
}

func TestScriptNodeXMLSerialization(t *testing.T) {
	flowNodes := []PipelineNode{
		{
			NodeType: "script",
			Attributes: map[string]string{
				"id":         "RunPowerShellTask",
				"language":   "powershell",
				"output_var": "ScriptOutput",
				"timeout":    "45s",
				"on_error":   "continue",
			},
			ContentText: `Write-Host "Running custom step"`,
		},
	}

	xml := GenerateXML("script_pipeline", nil, nil, nil, flowNodes)

	if !strings.Contains(xml, `<script`) {
		t.Fatalf("expected <script tag in xml: %s", xml)
	}
	if !strings.Contains(xml, `id="RunPowerShellTask"`) {
		t.Errorf("expected id in script tag: %s", xml)
	}
	if !strings.Contains(xml, `language="powershell"`) {
		t.Errorf("expected language in script tag: %s", xml)
	}
	if !strings.Contains(xml, `output_var="ScriptOutput"`) {
		t.Errorf("expected output_var in script tag: %s", xml)
	}
	if !strings.Contains(xml, `timeout="45s"`) {
		t.Errorf("expected timeout in script tag: %s", xml)
	}
	if !strings.Contains(xml, `on_error="continue"`) {
		t.Errorf("expected on_error in script tag: %s", xml)
	}
	if !strings.Contains(xml, `Write-Host "Running custom step"`) {
		t.Errorf("expected script body content in xml: %s", xml)
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

func TestFileBrowsingEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp(".", "test_browse_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_browse.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Create test files and directories
	subDir := filepath.Join(tmpDir, "subfolder")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subfolder: %v", err)
	}
	_ = os.WriteFile(filepath.Join(tmpDir, "scripts.xml"), []byte("<pipeline></pipeline>"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("notes"), 0644)
	_ = os.WriteFile(filepath.Join(subDir, "nested.xml"), []byte("<config></config>"), 0644)

	// Test 1: Browse tmpDir with .xml filter
	req := httptest.NewRequest(http.MethodGet, "/api/files/browse?dir="+tmpDir+"&ext=.xml", nil)
	rec := httptest.NewRecorder()
	server.handleBrowseFiles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var browseResp BrowseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &browseResp); err != nil {
		t.Fatalf("failed to unmarshal browse response: %v", err)
	}

	// Should contain "subfolder" (dir) and "scripts.xml" (file), but NOT "readme.txt"
	hasSubfolder := false
	hasScriptsXML := false
	hasReadme := false
	for _, entry := range browseResp.Entries {
		if entry.Name == "subfolder" && entry.IsDir {
			hasSubfolder = true
		}
		if entry.Name == "scripts.xml" && !entry.IsDir {
			hasScriptsXML = true
		}
		if entry.Name == "readme.txt" {
			hasReadme = true
		}
	}

	if !hasSubfolder {
		t.Errorf("expected 'subfolder' directory in entries")
	}
	if !hasScriptsXML {
		t.Errorf("expected 'scripts.xml' file in entries")
	}
	if hasReadme {
		t.Errorf("did not expect 'readme.txt' to match .xml filter")
	}

	// Test 2: Quick files search from root
	req2 := httptest.NewRequest(http.MethodGet, "/api/files/quick?root="+tmpDir+"&ext=.xml", nil)
	rec2 := httptest.NewRecorder()
	server.handleQuickFiles(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200 for quick files, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var quickFiles []string
	if err := json.Unmarshal(rec2.Body.Bytes(), &quickFiles); err != nil {
		t.Fatalf("failed to unmarshal quick files response: %v", err)
	}

	if len(quickFiles) != 2 {
		t.Errorf("expected 2 XML files in quick search, got %d: %v", len(quickFiles), quickFiles)
	}
}

func TestFileBrowsingIncludesHiddenDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp(".", "test_browse_hidden_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hiddenDir := filepath.Join(tmpDir, ".config")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("failed to create hidden directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "settings.xml"), []byte("<settings></settings>"), 0644); err != nil {
		t.Fatalf("failed to create hidden xml file: %v", err)
	}

	storage, err := NewStorage(filepath.Join(tmpDir, "test_hidden.db"))
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/browse?dir="+tmpDir+"&ext=.xml", nil)
	rec := httptest.NewRecorder()
	server.handleBrowseFiles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var browseResp BrowseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &browseResp); err != nil {
		t.Fatalf("failed to unmarshal browse response: %v", err)
	}

	foundHiddenDir := false
	for _, entry := range browseResp.Entries {
		if entry.Name == ".config" && entry.IsDir {
			foundHiddenDir = true
			break
		}
	}
	if !foundHiddenDir {
		t.Fatalf("expected hidden directory .config to appear in browse results: %+v", browseResp.Entries)
	}
}

func TestExecuteStreamOptionsWithoutScript(t *testing.T) {
	tmpDir := t.TempDir()
	optionsPath := filepath.Join(tmpDir, "options.xml")
	if err := os.WriteFile(optionsPath, []byte("<options><option name='example' value='true'/></options>"), 0644); err != nil {
		t.Fatalf("failed to write options file: %v", err)
	}

	storage, err := NewStorage(filepath.Join(tmpDir, "runner_options.db"))
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file=&config=&options="+optionsPath, nil)
	rec := httptest.NewRecorder()
	server.handleExecuteStream(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "[FLOW] Running: ") {
		t.Fatalf("expected execution log output, got: %s", body)
	}
	if strings.Contains(body, "-file") && strings.Contains(body, "scripts.xml") {
		t.Fatalf("expected no default script argument when options are set, got: %s", body)
	}
	if !strings.Contains(body, "-options") || !strings.Contains(body, filepath.Base(optionsPath)) {
		t.Fatalf("expected options argument in execution log, got: %s", body)
	}
}

func TestExecuteStreamNodeEvents(t *testing.T) {
	storage, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Missing file returns ERROR done event
	reqMissing := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file=nonexistent_file.xml", nil)
	recMissing := httptest.NewRecorder()
	server.handleExecuteStream(recMissing, reqMissing)

	bodyMissing := recMissing.Body.String()
	if !strings.Contains(bodyMissing, "Script file not found") {
		t.Errorf("expected 'Script file not found' in response, got: %s", bodyMissing)
	}
}

func TestPurgeDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_purge.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	// Ensure default script exists first
	_, _ = storage.GetOrCreateDefaultScript()

	// Create extra scripts and nodes
	sc, err := storage.CreateScript("extra_script", "desc")
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}
	_, err = storage.AddNode(sc.ID, "flow", "sql", map[string]string{"id": "node1"}, "SELECT 1")
	if err != nil {
		t.Fatalf("failed to add node: %v", err)
	}

	_ = storage.SaveConfigFile("custom_config.xml", "<config/>")

	// Verify extra data exists
	scripts, _ := storage.ListScripts()
	if len(scripts) < 2 {
		t.Fatalf("expected at least 2 scripts before purge, got %d", len(scripts))
	}

	// Purge
	defaultScript, err := storage.PurgeDatabase()
	if err != nil {
		t.Fatalf("PurgeDatabase failed: %v", err)
	}
	if defaultScript == nil || defaultScript.Name != "default_pipeline" {
		t.Fatalf("expected default_pipeline after purge, got: %+v", defaultScript)
	}

	// After purge, only 1 default script should exist
	scriptsAfter, err := storage.ListScripts()
	if err != nil || len(scriptsAfter) != 1 {
		t.Fatalf("expected exactly 1 script after purge, got: %d", len(scriptsAfter))
	}
	if scriptsAfter[0].Name != "default_pipeline" {
		t.Errorf("expected script name 'default_pipeline', got: %s", scriptsAfter[0].Name)
	}
}

func TestDeleteScript(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_delete.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	sc1, err := storage.CreateScript("script_1", "")
	if err != nil {
		t.Fatalf("failed to create script_1: %v", err)
	}
	_, _ = storage.AddNode(sc1.ID, "flow", "sql", map[string]string{"id": "s1_node"}, "SELECT 1")

	sc2, err := storage.CreateScript("script_2", "")
	if err != nil {
		t.Fatalf("failed to create script_2: %v", err)
	}
	_, _ = storage.AddNode(sc2.ID, "flow", "sql", map[string]string{"id": "s2_node"}, "SELECT 2")

	// Delete script_1
	if err := storage.DeleteScript(sc1.ID); err != nil {
		t.Fatalf("DeleteScript failed: %v", err)
	}

	// Verify sc1 nodes deleted
	nodes1, _ := storage.GetNodes(sc1.ID, "")
	if len(nodes1) != 0 {
		t.Errorf("expected 0 nodes for deleted script, got %d", len(nodes1))
	}

	// Verify sc1 does not exist
	_, err = storage.GetScript(sc1.ID)
	if err == nil {
		t.Errorf("expected error getting deleted script, got nil")
	}

	// Delete remaining scripts until 0
	scripts, _ := storage.ListScripts()
	for _, sc := range scripts {
		_ = storage.DeleteScript(sc.ID)
	}

	// Verify auto-re-seed fallback
	scriptsRemaining, _ := storage.ListScripts()
	if len(scriptsRemaining) == 0 {
		t.Errorf("expected fallback default script to be seeded, got 0 scripts")
	}
}

func TestCopyScript(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_copy.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	src, err := storage.CreateScript("original_pipeline", "My Original Pipeline")
	if err != nil {
		t.Fatalf("failed to create original script: %v", err)
	}
	_, _ = storage.AddNode(src.ID, "variables", "variable", map[string]string{"name": "Var1", "value": "Val1"}, "")
	_, _ = storage.AddNode(src.ID, "flow", "sql", map[string]string{"id": "step1"}, "SELECT 42;")

	// Copy with custom name
	copied, err := storage.CopyScript(src.ID, "original_pipeline (Copy)")
	if err != nil {
		t.Fatalf("CopyScript failed: %v", err)
	}
	if copied.ID == src.ID {
		t.Errorf("expected different script ID for copy")
	}
	if copied.Name != "original_pipeline (Copy)" {
		t.Errorf("expected name 'original_pipeline (Copy)', got: %s", copied.Name)
	}

	nodes, err := storage.GetNodes(copied.ID, "")
	if err != nil || len(nodes) != 2 {
		t.Fatalf("expected 2 copied nodes, got %d (err: %v)", len(nodes), err)
	}

	// Test collision handling: copy again with same requested name
	copied2, err := storage.CopyScript(src.ID, "original_pipeline (Copy)")
	if err != nil {
		t.Fatalf("CopyScript second time failed: %v", err)
	}
	if copied2.Name != "original_pipeline (Copy) (2)" {
		t.Errorf("expected name collision resolution 'original_pipeline (Copy) (2)', got: %s", copied2.Name)
	}
}

func TestImportPipelineFromXML(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_import.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<pipeline description="Test pipeline for import" name="imported_sample">
    <variables>
        <variable name="DB_CONN" type="string" value="sqlite://test.db" />
    </variables>
    <databases>
        <database name="main_db" driver="sqlite" connection_string="file:memdb1?mode=memory" />
    </databases>
    <preflight>
        <sql id="check_version" db="main_db">SELECT 1;</sql>
    </preflight>
    <flow>
        <sql id="step_insert" db="main_db">INSERT INTO logs VALUES(1);</sql>
    </flow>
</pipeline>`

	tmpFile := filepath.Join(t.TempDir(), "sample_pipeline.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlContent), 0644); err != nil {
		t.Fatalf("failed to write tmp xml: %v", err)
	}

	script, err := ImportPipelineFromXML(tmpFile, "", storage)
	if err != nil {
		t.Fatalf("ImportPipelineFromXML failed: %v", err)
	}

	if script.Name != "imported_sample" {
		t.Errorf("expected script name 'imported_sample', got: %s", script.Name)
	}

	varNodes, _ := storage.GetNodes(script.ID, "variables")
	if len(varNodes) != 1 || varNodes[0].Attributes["name"] != "DB_CONN" {
		t.Errorf("unexpected variables nodes: %+v", varNodes)
	}

	dbNodes, _ := storage.GetNodes(script.ID, "databases")
	if len(dbNodes) != 1 || dbNodes[0].Attributes["name"] != "main_db" {
		t.Errorf("unexpected databases nodes: %+v", dbNodes)
	}

	preNodes, _ := storage.GetNodes(script.ID, "preflight")
	if len(preNodes) != 1 || preNodes[0].Attributes["id"] != "check_version" {
		t.Errorf("unexpected preflight nodes: %+v", preNodes)
	}

	flowNodes, _ := storage.GetNodes(script.ID, "flow")
	if len(flowNodes) != 1 || flowNodes[0].Attributes["id"] != "step_insert" {
		t.Errorf("unexpected flow nodes: %+v", flowNodes)
	}

	// Test importing an existing file in the repo
	repoExample := filepath.Join("..", "examples", "check_two_tables_in_parallel_take_action.xml")
	if _, err := os.Stat(repoExample); err == nil {
		importedRepo, err := ImportPipelineFromXML(repoExample, "parallel_check_custom", storage)
		if err != nil {
			t.Fatalf("failed to import repo example: %v", err)
		}
		if importedRepo.Name != "parallel_check_custom" {
			t.Errorf("expected custom name 'parallel_check_custom', got %s", importedRepo.Name)
		}
		repoFlow, _ := storage.GetNodes(importedRepo.ID, "flow")
		if len(repoFlow) == 0 {
			t.Errorf("expected flow nodes from repo example, got 0")
		}
	}
}

func TestPipelineManagementAPIs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_apis.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	sc, err := storage.GetOrCreateDefaultScript()
	if err != nil {
		t.Fatalf("failed to get default script: %v", err)
	}

	// 1. Copy API
	copyReqBody, _ := json.Marshal(map[string]any{"id": sc.ID, "new_name": "API Cloned Pipeline"})
	reqCopy := httptest.NewRequest(http.MethodPost, "/api/scripts/copy", bytes.NewReader(copyReqBody))
	reqCopy.Header.Set("X-Requested-With", "XMLHttpRequest")
	recCopy := httptest.NewRecorder()
	server.handleCopyScript(recCopy, reqCopy)
	if recCopy.Code != http.StatusOK {
		t.Fatalf("handleCopyScript failed with code %d: %s", recCopy.Code, recCopy.Body.String())
	}
	var copyResp struct {
		Success bool    `json:"success"`
		Script  *Script `json:"script"`
	}
	_ = json.Unmarshal(recCopy.Body.Bytes(), &copyResp)
	if !copyResp.Success || copyResp.Script == nil || copyResp.Script.Name != "API Cloned Pipeline" {
		t.Fatalf("unexpected copy API response: %s", recCopy.Body.String())
	}

	// 2. Import API
	tmpXMLDir, err := os.MkdirTemp(".", "test_api_import_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpXMLDir)
	tmpXML := filepath.Join(tmpXMLDir, "api_import.xml")
	_ = os.WriteFile(tmpXML, []byte("<pipeline name=\"api_import_pipe\"><flow><sql id=\"1\">SELECT 1</sql></flow></pipeline>"), 0644)
	importReqBody, _ := json.Marshal(map[string]any{"file_path": tmpXML, "name": "Imported via API"})
	reqImport := httptest.NewRequest(http.MethodPost, "/api/scripts/import", bytes.NewReader(importReqBody))
	reqImport.Header.Set("X-Requested-With", "XMLHttpRequest")
	recImport := httptest.NewRecorder()
	server.handleImportScript(recImport, reqImport)
	if recImport.Code != http.StatusOK {
		t.Fatalf("handleImportScript failed with code %d: %s", recImport.Code, recImport.Body.String())
	}

	// 3. Delete API
	reqDelete := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/scripts/delete?id=%d", copyResp.Script.ID), nil)
	reqDelete.Header.Set("X-Requested-With", "XMLHttpRequest")
	recDelete := httptest.NewRecorder()
	server.handleDeleteScript(recDelete, reqDelete)
	if recDelete.Code != http.StatusOK {
		t.Fatalf("handleDeleteScript failed with code %d: %s", recDelete.Code, recDelete.Body.String())
	}

	// 4. Purge API
	reqPurge := httptest.NewRequest(http.MethodPost, "/api/db/purge", nil)
	reqPurge.Header.Set("X-Requested-With", "XMLHttpRequest")
	recPurge := httptest.NewRecorder()
	server.handlePurgeDatabase(recPurge, reqPurge)
	if recPurge.Code != http.StatusOK {
		t.Fatalf("handlePurgeDatabase failed with code %d: %s", recPurge.Code, recPurge.Body.String())
	}
	var purgeResp struct {
		Success         bool   `json:"success"`
		Message         string `json:"message"`
		DefaultScriptID int64  `json:"default_script_id"`
	}
	_ = json.Unmarshal(recPurge.Body.Bytes(), &purgeResp)
	if !purgeResp.Success || purgeResp.DefaultScriptID == 0 {
		t.Fatalf("unexpected purge response: %s", recPurge.Body.String())
	}
}

func TestIndexSectionNavigation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_index_nav.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.handleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	html := rec.Body.String()
	requiredElements := []string{
		`id="nav-section-variables"`,
		`id="nav-section-databases"`,
		`id="nav-section-preflight"`,
		`id="nav-section-flow"`,
		`navigateToSection('variables'`,
		`navigateToSection('databases'`,
		`navigateToSection('preflight'`,
		`navigateToSection('flow'`,
		`highlightSectionNav`,
		`initSectionNavigation`,
	}

	for _, elem := range requiredElements {
		if !strings.Contains(html, elem) {
			t.Errorf("expected HTML to contain %q", elem)
		}
	}
}

func TestNestedContainersAndConditionalsStorage(t *testing.T) {
	tmpDir, err := os.MkdirTemp(".", "test_nested_storage_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_nested.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	sc, err := storage.CreateScript("nested_flow", "Testing nested containers and conditionals")
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	// 1. Add <group> root container
	groupNode, err := storage.AddNode(sc.ID, "flow", "group", map[string]string{"transaction": "true"}, "")
	if err != nil {
		t.Fatalf("failed to add group node: %v", err)
	}

	// 2. Add <if> container under <group>
	ifNode, err := storage.AddNodeWithParent(sc.ID, "flow", "if", map[string]string{"condition": "status == 'ACTIVE'"}, "", &groupNode.ID)
	if err != nil {
		t.Fatalf("failed to add if node: %v", err)
	}

	// Ensure then/else branches exist for <if>
	thenBranch, elseBranch, err := storage.EnsureIfBranches(sc.ID, "flow", ifNode.ID)
	if err != nil {
		t.Fatalf("failed to ensure if branches: %v", err)
	}
	if thenBranch == nil || elseBranch == nil {
		t.Fatalf("expected then and else branches, got then=%v, else=%v", thenBranch, elseBranch)
	}

	// 3. Add <parallel> inside <then>
	parallelNode, err := storage.AddNodeWithParent(sc.ID, "flow", "parallel", map[string]string{"max_threads": "4"}, "", &thenBranch.ID)
	if err != nil {
		t.Fatalf("failed to add parallel node: %v", err)
	}

	// 4. Add 2 <sql> nodes inside <parallel>
	pSql1, err := storage.AddNodeWithParent(sc.ID, "flow", "sql", map[string]string{"id": "p1"}, "SELECT 1;", &parallelNode.ID)
	if err != nil {
		t.Fatalf("failed to add pSql1: %v", err)
	}
	pSql2, err := storage.AddNodeWithParent(sc.ID, "flow", "sql", map[string]string{"id": "p2"}, "SELECT 2;", &parallelNode.ID)
	if err != nil {
		t.Fatalf("failed to add pSql2: %v", err)
	}

	// 5. Add <script> inside <else>
	elseScript, err := storage.AddNodeWithParent(sc.ID, "flow", "script", map[string]string{"id": "e1", "language": "powershell"}, "Write-Host 'fallback'", &elseBranch.ID)
	if err != nil {
		t.Fatalf("failed to add elseScript: %v", err)
	}

	// 6. Add <while> under <group>
	whileNode, err := storage.AddNodeWithParent(sc.ID, "flow", "while", map[string]string{"condition": "more == true"}, "", &groupNode.ID)
	if err != nil {
		t.Fatalf("failed to add whileNode: %v", err)
	}
	_, err = storage.AddNodeWithParent(sc.ID, "flow", "sql", map[string]string{"id": "w1"}, "SELECT next();", &whileNode.ID)
	if err != nil {
		t.Fatalf("failed to add while sql: %v", err)
	}

	// 7. Add <foreach> under <group> with driver query
	foreachNode, err := storage.AddNodeWithParent(sc.ID, "flow", "foreach", map[string]string{"var": "rec", "stream": "true"}, "SELECT id FROM items;", &groupNode.ID)
	if err != nil {
		t.Fatalf("failed to add foreachNode: %v", err)
	}
	_, err = storage.AddNodeWithParent(sc.ID, "flow", "sql", map[string]string{"id": "fe1"}, "INSERT INTO dst VALUES ({{.rec.id}});", &foreachNode.ID)
	if err != nil {
		t.Fatalf("failed to add foreach child sql: %v", err)
	}

	// Query tree
	tree, err := storage.GetNodeTree(sc.ID, "flow")
	if err != nil {
		t.Fatalf("failed to get node tree: %v", err)
	}

	// Verify root level
	if len(tree) != 1 {
		t.Fatalf("expected 1 root node (group), got %d", len(tree))
	}
	if tree[0].NodeType != "group" {
		t.Fatalf("expected root node to be group, got %s", tree[0].NodeType)
	}

	// Verify group children: if, while, foreach
	if len(tree[0].Children) != 3 {
		t.Fatalf("expected 3 children under group, got %d", len(tree[0].Children))
	}

	ifChild := tree[0].Children[0]
	if ifChild.NodeType != "if" {
		t.Fatalf("expected first child of group to be if, got %s", ifChild.NodeType)
	}
	// Verify if branches: then and else
	then := ifChild.GetThenBranch()
	elseB := ifChild.GetElseBranch()
	if then == nil || elseB == nil {
		t.Fatalf("expected both then and else branches on if child")
	}

	// Verify then children: parallel
	if len(then.Children) != 1 || then.Children[0].NodeType != "parallel" {
		t.Fatalf("expected parallel child under then, got: %+v", then.Children)
	}
	// Verify parallel children: 2 sql nodes
	parallelChild := then.Children[0]
	if len(parallelChild.Children) != 2 {
		t.Fatalf("expected 2 children under parallel, got %d", len(parallelChild.Children))
	}
	if parallelChild.Children[0].Attributes["id"] != "p1" || parallelChild.Children[1].Attributes["id"] != "p2" {
		t.Fatalf("unexpected parallel children IDs: %s, %s", parallelChild.Children[0].Attributes["id"], parallelChild.Children[1].Attributes["id"])
	}

	// Verify else children: script
	if len(elseB.Children) != 1 || elseB.Children[0].NodeType != "script" {
		t.Fatalf("expected script child under else, got: %+v", elseB.Children)
	}
	if elseB.Children[0].ID != elseScript.ID {
		t.Fatalf("expected elseScript ID %d, got %d", elseScript.ID, elseB.Children[0].ID)
	}

	// Verify while children: 1 sql
	whileChild := tree[0].Children[1]
	if len(whileChild.Children) != 1 || whileChild.Children[0].NodeType != "sql" {
		t.Fatalf("expected 1 sql child under while, got: %+v", whileChild.Children)
	}

	// Verify foreach children: 1 sql and driver query retained
	foreachChild := tree[0].Children[2]
	if len(foreachChild.Children) != 1 || foreachChild.Children[0].NodeType != "sql" {
		t.Fatalf("expected 1 sql child under foreach, got: %+v", foreachChild.Children)
	}
	if !strings.Contains(foreachChild.ContentText, "SELECT id FROM items;") {
		t.Fatalf("expected driver query in foreach content, got %q", foreachChild.ContentText)
	}

	// Test MoveNode within container (swap parallel sql steps)
	if err := storage.MoveNode(pSql2.ID, "up"); err != nil {
		t.Fatalf("failed to move pSql2 up: %v", err)
	}
	updatedTree, _ := storage.GetNodeTree(sc.ID, "flow")
	updatedParallel := updatedTree[0].Children[0].GetThenBranch().Children[0]
	if updatedParallel.Children[0].ID != pSql2.ID || updatedParallel.Children[1].ID != pSql1.ID {
		t.Fatalf("expected pSql2 to be before pSql1 after move up")
	}

	// Test CopyScript with nested hierarchy
	copied, err := storage.CopyScript(sc.ID, "cloned_nested_flow")
	if err != nil {
		t.Fatalf("failed to copy script: %v", err)
	}
	copiedTree, err := storage.GetNodeTree(copied.ID, "flow")
	if err != nil {
		t.Fatalf("failed to get node tree for copied script: %v", err)
	}
	if len(copiedTree) != 1 || copiedTree[0].NodeType != "group" {
		t.Fatalf("expected copied tree root to be group")
	}
	if len(copiedTree[0].Children) != 3 {
		t.Fatalf("expected copied group to have 3 children, got %d", len(copiedTree[0].Children))
	}
	copiedThen := copiedTree[0].Children[0].GetThenBranch()
	if copiedThen == nil || len(copiedThen.Children) != 1 {
		t.Fatalf("expected copied then branch to have parallel child")
	}
	if len(copiedThen.Children[0].Children) != 2 {
		t.Fatalf("expected copied parallel to have 2 children")
	}

	// Test DeleteNode recursive purge (delete parallel container)
	if err := storage.DeleteNode(parallelNode.ID); err != nil {
		t.Fatalf("failed to delete parallel container: %v", err)
	}
	treeAfterParallelDelete, _ := storage.GetNodeTree(sc.ID, "flow")
	thenAfterDelete := treeAfterParallelDelete[0].Children[0].GetThenBranch()
	if len(thenAfterDelete.Children) != 0 {
		t.Fatalf("expected then branch to be empty after parallel deleted, got %d", len(thenAfterDelete.Children))
	}
	// Verify else branch is still intact
	elseAfterDelete := treeAfterParallelDelete[0].Children[0].GetElseBranch()
	if len(elseAfterDelete.Children) != 1 {
		t.Fatalf("expected else branch to remain intact with 1 child")
	}

	// Test DeleteNode on root group (purges entire tree)
	if err := storage.DeleteNode(groupNode.ID); err != nil {
		t.Fatalf("failed to delete root group: %v", err)
	}
	treeAfterRootDelete, _ := storage.GetNodeTree(sc.ID, "flow")
	if len(treeAfterRootDelete) != 0 {
		t.Fatalf("expected empty flow tree after root group deleted, got %d", len(treeAfterRootDelete))
	}
}

func TestRecursiveXMLGeneration(t *testing.T) {
	// Construct an in-memory tree
	sql1 := PipelineNode{
		NodeType:    "sql",
		Attributes:  map[string]string{"id": "StepParallel1"},
		ContentText: "SELECT 1;",
	}
	sql2 := PipelineNode{
		NodeType:    "sql",
		Attributes:  map[string]string{"id": "StepParallel2"},
		ContentText: "SELECT 2;",
	}
	parallel := PipelineNode{
		NodeType:   "parallel",
		Attributes: map[string]string{"max_threads": "4"},
		Children:   []PipelineNode{sql1, sql2},
	}
	thenBranch := PipelineNode{
		NodeType: "then",
		Children: []PipelineNode{parallel},
	}
	elseScript := PipelineNode{
		NodeType:    "script",
		Attributes:  map[string]string{"id": "fallback_script", "language": "powershell"},
		ContentText: "Write-Output 'fallback executed'",
	}
	elseBranch := PipelineNode{
		NodeType: "else",
		Children: []PipelineNode{elseScript},
	}
	ifNode := PipelineNode{
		NodeType:   "if",
		Attributes: map[string]string{"condition": "error_count == 0"},
		Children:   []PipelineNode{thenBranch, elseBranch},
	}
	whileSql := PipelineNode{
		NodeType:    "sql",
		Attributes:  map[string]string{"id": "loop_step"},
		ContentText: "SELECT loop_batch();",
	}
	whileNode := PipelineNode{
		NodeType:   "while",
		Attributes: map[string]string{"condition": "has_more == 'yes'"},
		Children:   []PipelineNode{whileSql},
	}
	foreachSql := PipelineNode{
		NodeType:    "sql",
		Attributes:  map[string]string{"id": "fe_step"},
		ContentText: "INSERT INTO sink VALUES ({{.row.id}});",
	}
	foreachNode := PipelineNode{
		NodeType:    "foreach",
		Attributes:  map[string]string{"var": "row", "stream": "true"},
		ContentText: "SELECT id FROM source_stream;",
		Children:    []PipelineNode{foreachSql},
	}
	groupNode := PipelineNode{
		NodeType:   "group",
		Attributes: map[string]string{"transaction": "true"},
		Children:   []PipelineNode{ifNode, whileNode, foreachNode},
	}

	xml := GenerateXML("nested_demo", nil, nil, nil, []PipelineNode{groupNode})

	// Assertions on the generated XML structure
	expectedSnippets := []string{
		`<group transaction="true">`,
		`<if condition="error_count == 0">`,
		`<then>`,
		`<parallel max_threads="4">`,
		`<sql id="StepParallel1">`,
		`<sql id="StepParallel2">`,
		`</parallel>`,
		`</then>`,
		`<else>`,
		`<script id="fallback_script" language="powershell">`,
		`</else>`,
		`</if>`,
		`<while condition="has_more == 'yes'">`,
		`<sql id="loop_step">`,
		`</while>`,
		`<foreach`,
		`var="row"`,
		`stream="true"`,
		`SELECT id FROM source_stream;`,
		`<sql id="fe_step">`,
		`</foreach>`,
		`</group>`,
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(xml, snippet) {
			t.Errorf("expected generated XML to contain snippet %q\nGenerated XML:\n%s", snippet, xml)
		}
	}

	// Verify that empty else is omitted
	ifNoElse := PipelineNode{
		NodeType:   "if",
		Attributes: map[string]string{"condition": "x &gt; 0"},
		Children: []PipelineNode{
			{NodeType: "then", Children: []PipelineNode{sql1}},
			{NodeType: "else", Children: nil},
		},
	}
	xmlNoElse := GenerateXML("no_else", nil, nil, nil, []PipelineNode{ifNoElse})
	if strings.Contains(xmlNoElse, "<else>") || strings.Contains(xmlNoElse, "</else>") {
		t.Errorf("expected empty <else> to be omitted from XML, got:\n%s", xmlNoElse)
	}
}

func findNodeRecursive(nodes []PipelineNode, nodeType string) *PipelineNode {
	for i := range nodes {
		if nodes[i].NodeType == nodeType {
			return &nodes[i]
		}
		if found := findNodeRecursive(nodes[i].Children, nodeType); found != nil {
			return found
		}
	}
	return nil
}

func TestRecursiveXMLImport(t *testing.T) {
	tmpDir, err := os.MkdirTemp(".", "test_xml_import_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewStorage(filepath.Join(tmpDir, "test_import.db"))
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	// 1. Import if_than_else.xml
	ifPath := filepath.Join("..", "examples", "if_than_else.xml")
	scIf, err := ImportPipelineFromXML(ifPath, "If-Then-Else Test", storage)
	if err != nil {
		t.Fatalf("failed to import if_than_else.xml: %v", err)
	}

	flowTree, err := storage.GetNodeTree(scIf.ID, "flow")
	if err != nil {
		t.Fatalf("failed to get flow tree for if pipeline: %v", err)
	}

	ifNode := findNodeRecursive(flowTree, "if")
	if ifNode == nil {
		t.Fatalf("expected if node in imported if pipeline")
	}
	thenB := ifNode.GetThenBranch()
	elseB := ifNode.GetElseBranch()
	if thenB == nil || len(thenB.Children) == 0 {
		t.Errorf("expected then branch with children in imported if node")
	}
	if elseB == nil || len(elseB.Children) == 0 {
		t.Errorf("expected else branch with children in imported if node")
	}

	// 2. Import parallel_example.xml (contains <parallel> and <foreach> nested in <then>)
	parallelPath := filepath.Join("..", "examples", "parallel_example.xml")
	scPar, err := ImportPipelineFromXML(parallelPath, "Parallel Test", storage)
	if err != nil {
		t.Fatalf("failed to import parallel_example.xml: %v", err)
	}

	parTree, err := storage.GetNodeTree(scPar.ID, "flow")
	if err != nil {
		t.Fatalf("failed to get flow tree for parallel pipeline: %v", err)
	}

	parallelNode := findNodeRecursive(parTree, "parallel")
	if parallelNode == nil || len(parallelNode.Children) == 0 {
		t.Fatalf("expected parallel node with children in imported parallel pipeline")
	}
	if len(parallelNode.Children) < 3 {
		t.Errorf("expected at least 3 children in parallel container, got %d", len(parallelNode.Children))
	}

	// 3. Import true_false_foreach_example.xml (multi-level: <if> -> <then> -> <foreach> -> <group> -> <sql>)
	fePath := filepath.Join("..", "examples", "true_false_foreach_example.xml")
	scFE, err := ImportPipelineFromXML(fePath, "ForEach Test", storage)
	if err != nil {
		t.Fatalf("failed to import true_false_foreach_example.xml: %v", err)
	}

	feTree, err := storage.GetNodeTree(scFE.ID, "flow")
	if err != nil {
		t.Fatalf("failed to get flow tree for foreach pipeline: %v", err)
	}

	feNode := findNodeRecursive(feTree, "foreach")
	if feNode == nil || len(feNode.Children) == 0 {
		t.Fatalf("expected foreach node with children in imported foreach pipeline")
	}
	groupChild := findNodeRecursive(feNode.Children, "group")
	if groupChild == nil || len(groupChild.Children) == 0 {
		t.Fatalf("expected nested group inside foreach with child steps (multi-level hierarchy)")
	}
	if groupChild.Children[0].NodeType != "sql" {
		t.Errorf("expected sql step inside nested group, got %s", groupChild.Children[0].NodeType)
	}
}

func TestServerNestedNodeAPI(t *testing.T) {
	tmpDir, err := os.MkdirTemp(".", "test_api_nested_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewStorage(filepath.Join(tmpDir, "test_api_nested.db"))
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	sc, err := storage.CreateScript("api_nested_test", "API test for nested nodes")
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	// Add an <if> node via /api/nodes/add
	addIfBody, _ := json.Marshal(NodeRequest{
		ScriptID:   sc.ID,
		NodeType:   "if",
		Section:    "flow",
		Attributes: map[string]string{"condition": "x == 1"},
	})
	reqIf := httptest.NewRequest(http.MethodPost, "/api/nodes/add", bytes.NewReader(addIfBody))
	reqIf.Header.Set("X-Requested-With", "XMLHttpRequest")
	recIf := httptest.NewRecorder()
	server.handleAddNode(recIf, reqIf)
	if recIf.Code != http.StatusOK {
		t.Fatalf("handleAddNode failed for if node: code %d: %s", recIf.Code, recIf.Body.String())
	}

	// Query tree to find the auto-created then branch
	tree, err := storage.GetNodeTree(sc.ID, "flow")
	if err != nil || len(tree) != 1 {
		t.Fatalf("expected 1 root if node, got %d, err=%v", len(tree), err)
	}
	ifNode := tree[0]
	thenBranch := ifNode.GetThenBranch()
	if thenBranch == nil {
		t.Fatalf("expected then branch to be created automatically for if node")
	}

	// Add a <sql> node inside the <then> branch via /api/nodes/add
	addSqlBody, _ := json.Marshal(NodeRequest{
		ScriptID:     sc.ID,
		ParentNodeID: &thenBranch.ID,
		NodeType:     "sql",
		Section:      "flow",
		Attributes:   map[string]string{"id": "StepInThen"},
		Content:      "SELECT 'inside then';",
	})
	reqSql := httptest.NewRequest(http.MethodPost, "/api/nodes/add", bytes.NewReader(addSqlBody))
	reqSql.Header.Set("X-Requested-With", "XMLHttpRequest")
	recSql := httptest.NewRecorder()
	server.handleAddNode(recSql, reqSql)
	if recSql.Code != http.StatusOK {
		t.Fatalf("handleAddNode failed for child sql node: code %d: %s", recSql.Code, recSql.Body.String())
	}

	// Request canvas rendering via /api/canvas
	reqCanvas := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/canvas?script_id=%d", sc.ID), nil)
	recCanvas := httptest.NewRecorder()
	server.handleCanvas(recCanvas, reqCanvas)
	if recCanvas.Code != http.StatusOK {
		t.Fatalf("handleCanvas failed with code %d: %s", recCanvas.Code, recCanvas.Body.String())
	}

	canvasHTML := recCanvas.Body.String()
	// Assert that conditional elements and child steps are rendered in HTML
	if !strings.Contains(canvasHTML, "THEN") {
		t.Errorf("expected canvas HTML to contain 'THEN' branch indicator")
	}
	if !strings.Contains(canvasHTML, "ELSE") {
		t.Errorf("expected canvas HTML to contain 'ELSE' branch indicator")
	}
	if !strings.Contains(canvasHTML, "StepInThen") {
		t.Errorf("expected canvas HTML to contain child step 'StepInThen'")
	}
	if !strings.Contains(canvasHTML, "container-drop-zone") {
		t.Errorf("expected canvas HTML to contain 'container-drop-zone' for nested drop targets")
	}

	// Request preview rendering via /api/preview
	reqPreview := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/preview?script_id=%d", sc.ID), nil)
	recPreview := httptest.NewRecorder()
	server.handlePreview(recPreview, reqPreview)
	if recPreview.Code != http.StatusOK {
		t.Fatalf("handlePreview failed with code %d: %s", recPreview.Code, recPreview.Body.String())
	}
	previewXML := recPreview.Body.String()
	if !strings.Contains(previewXML, "<if condition=\"x == 1\">") ||
		!strings.Contains(previewXML, "<then>") ||
		!strings.Contains(previewXML, "<sql id=\"StepInThen\">") {
		t.Errorf("preview XML does not reflect nested conditional structure:\n%s", previewXML)
	}
}

func TestDeleteLoadedPipelineScripts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "delete_test.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 8089)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Create multiple pipeline scripts
	s1, err := storage.CreateScript("Pipeline_A", "First pipeline")
	if err != nil {
		t.Fatalf("failed to create pipeline A: %v", err)
	}
	s2, err := storage.CreateScript("Pipeline_B", "Second pipeline")
	if err != nil {
		t.Fatalf("failed to create pipeline B: %v", err)
	}
	s3, err := storage.CreateScript("Pipeline_C", "Third pipeline")
	if err != nil {
		t.Fatalf("failed to create pipeline C: %v", err)
	}

	// Verify list endpoint returns all scripts
	reqList := httptest.NewRequest(http.MethodGet, "/api/scripts/list", nil)
	recList := httptest.NewRecorder()
	server.handleListScripts(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("handleListScripts failed: %d, body: %s", recList.Code, recList.Body.String())
	}
	var listedScripts []Script
	if err := json.Unmarshal(recList.Body.Bytes(), &listedScripts); err != nil {
		t.Fatalf("failed to decode scripts list: %v", err)
	}
	if len(listedScripts) < 3 {
		t.Fatalf("expected at least 3 scripts, got %d", len(listedScripts))
	}

	// 2. Attempt deletion without CSRF token -> MUST return 403 Forbidden
	reqNoCSRF := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/scripts/delete?id=%d", s2.ID), nil)
	recNoCSRF := httptest.NewRecorder()
	server.handleDeleteScript(recNoCSRF, reqNoCSRF)
	if recNoCSRF.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden when deleting without CSRF, got %d", recNoCSRF.Code)
	}

	// 3. Attempt deletion with wrong CSRF token -> MUST return 403 Forbidden
	reqWrongCSRF := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/scripts/delete?id=%d", s2.ID), nil)
	reqWrongCSRF.Header.Set("X-CSRF-Token", "invalid-fake-token")
	recWrongCSRF := httptest.NewRecorder()
	server.handleDeleteScript(recWrongCSRF, reqWrongCSRF)
	if recWrongCSRF.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden with invalid CSRF token, got %d", recWrongCSRF.Code)
	}

	// Verify Pipeline_B is still present in storage
	if _, err := storage.GetScript(s2.ID); err != nil {
		t.Errorf("Pipeline_B should NOT have been deleted after failed CSRF: %v", err)
	}

	// 4. Successful deletion of non-active loaded script with valid CSRF token
	reqValidCSRF := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/scripts/delete?id=%d", s2.ID), nil)
	reqValidCSRF.Header.Set("X-Requested-With", "XMLHttpRequest")
	recValidCSRF := httptest.NewRecorder()
	server.handleDeleteScript(recValidCSRF, reqValidCSRF)
	if recValidCSRF.Code != http.StatusOK {
		t.Fatalf("handleDeleteScript failed with valid CSRF: code %d, body: %s", recValidCSRF.Code, recValidCSRF.Body.String())
	}

	var delResp struct {
		Success      bool     `json:"success"`
		NextScriptID int64    `json:"next_script_id"`
		Remaining    []Script `json:"remaining"`
	}
	if err := json.Unmarshal(recValidCSRF.Body.Bytes(), &delResp); err != nil {
		t.Fatalf("failed to parse delete response: %v", err)
	}
	if !delResp.Success {
		t.Errorf("expected success true in delete response")
	}

	// Verify Pipeline_B is gone from storage, but Pipeline_A and Pipeline_C remain
	if _, err := storage.GetScript(s2.ID); err == nil {
		t.Errorf("expected Pipeline_B to be deleted from storage")
	}
	if _, err := storage.GetScript(s1.ID); err != nil {
		t.Errorf("Pipeline_A should still exist: %v", err)
	}
	if _, err := storage.GetScript(s3.ID); err != nil {
		t.Errorf("Pipeline_C should still exist: %v", err)
	}

	// 5. Delete via JSON body payload instead of query param
	delBody, _ := json.Marshal(map[string]int64{"id": s3.ID})
	reqJSONBody := httptest.NewRequest(http.MethodPost, "/api/scripts/delete", bytes.NewReader(delBody))
	reqJSONBody.Header.Set("X-Requested-With", "XMLHttpRequest")
	recJSONBody := httptest.NewRecorder()
	server.handleDeleteScript(recJSONBody, reqJSONBody)
	if recJSONBody.Code != http.StatusOK {
		t.Fatalf("handleDeleteScript via JSON body failed: %d, body: %s", recJSONBody.Code, recJSONBody.Body.String())
	}
	if _, err := storage.GetScript(s3.ID); err == nil {
		t.Errorf("expected Pipeline_C to be deleted via JSON body")
	}

	// 6. Delete the last remaining script (s1) -> must re-seed default_pipeline
	reqDelLast := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/scripts/delete?id=%d", s1.ID), nil)
	reqDelLast.Header.Set("X-Requested-With", "XMLHttpRequest")
	recDelLast := httptest.NewRecorder()
	server.handleDeleteScript(recDelLast, reqDelLast)
	if recDelLast.Code != http.StatusOK {
		t.Fatalf("handleDeleteScript on last script failed: %d, body: %s", recDelLast.Code, recDelLast.Body.String())
	}
	var lastDelResp struct {
		Success      bool     `json:"success"`
		NextScriptID int64    `json:"next_script_id"`
		Remaining    []Script `json:"remaining"`
	}
	if err := json.Unmarshal(recDelLast.Body.Bytes(), &lastDelResp); err != nil {
		t.Fatalf("failed to decode last delete response: %v", err)
	}
	if len(lastDelResp.Remaining) == 0 {
		t.Errorf("expected auto re-seeded default script after deleting all scripts, got 0 remaining")
	}
	if lastDelResp.NextScriptID <= 0 {
		t.Errorf("expected valid positive next_script_id, got %d", lastDelResp.NextScriptID)
	}

	// 7. Verify Index page renders CSRF token and Manage Pipelines modal in HTML
	reqIndex := httptest.NewRequest(http.MethodGet, "/", nil)
	recIndex := httptest.NewRecorder()
	server.handleIndex(recIndex, reqIndex)
	if recIndex.Code != http.StatusOK {
		t.Fatalf("handleIndex failed: %d", recIndex.Code)
	}
	indexHTML := recIndex.Body.String()
	if !strings.Contains(indexHTML, "manage-pipelines-modal") {
		t.Errorf("index HTML does not contain manage-pipelines-modal")
	}
	if !strings.Contains(indexHTML, "openManagePipelinesModal") {
		t.Errorf("index HTML does not contain openManagePipelinesModal function")
	}
}

func TestXMLEntityEscapingAndBuilderDraftRun(t *testing.T) {
	// 1. Test escapeXMLAttr directly
	csRaw := "sqlserver://sa:Password123!@localhost:1433?database=database1&trustServerCertificate=true"
	escapedCS := escapeXMLAttr(csRaw)
	expectedCS := "sqlserver://sa:Password123!@localhost:1433?database=database1&amp;trustServerCertificate=true"
	if escapedCS != expectedCS {
		t.Errorf("expected escaped connection string %q, got %q", expectedCS, escapedCS)
	}

	// Verify no double-escaping
	doubleEscaped := escapeXMLAttr(escapedCS)
	if doubleEscaped != expectedCS {
		t.Errorf("expected no double-escaping %q, got %q", expectedCS, doubleEscaped)
	}

	// Verify quote escaping
	quoted := `Say "Hello" <World>`
	escapedQuoted := escapeXMLAttr(quoted)
	expectedQuoted := `Say &quot;Hello&quot; &lt;World&gt;`
	if escapedQuoted != expectedQuoted {
		t.Errorf("expected %q, got %q", expectedQuoted, escapedQuoted)
	}

	// 2. Test GenerateXML with connection strings and SQL containing special XML characters
	varNodes := []PipelineNode{
		{
			NodeType: "variable",
			Attributes: map[string]string{
				"name":        "Database1ConnStr",
				"value":       csRaw,
				"description": "Connection string with ampersands",
				"type":        "string",
			},
		},
		{
			NodeType: "variable",
			Attributes: map[string]string{
				"name":        "AlreadyEscaped",
				"value":       "https://example.com/api?a=1&amp;b=2",
				"description": "Already escaped ampersand",
				"type":        "string",
			},
		},
	}
	dbNodes := []PipelineNode{
		{
			NodeType: "database",
			Attributes: map[string]string{
				"name":              "database1",
				"driver":            "sqlserver",
				"connection_string": "{{Database1ConnStr}}",
			},
		},
	}
	flowNodes := []PipelineNode{
		{
			NodeType: "sql",
			Attributes: map[string]string{
				"id": "query_step",
				"db": "database1",
			},
			ContentText: "SELECT * FROM users WHERE age < 30 AND points > 50;",
		},
	}

	xmlContent := GenerateXML("test_pipeline", varNodes, dbNodes, nil, flowNodes)

	// Verify raw ampersand was escaped in output
	if strings.Contains(xmlContent, "&trustServerCertificate") {
		t.Errorf("xmlContent should not contain unescaped &trustServerCertificate: %s", xmlContent)
	}
	if !strings.Contains(xmlContent, "&amp;trustServerCertificate=true") {
		t.Errorf("xmlContent missing expected &amp;trustServerCertificate=true: %s", xmlContent)
	}
	// Verify already-escaped was not double escaped
	if strings.Contains(xmlContent, "&amp;amp;") {
		t.Errorf("xmlContent should not contain double-escaped &amp;amp;: %s", xmlContent)
	}

	// Verify Flow engine parser accepts the generated XML without syntax errors
	cfg, err := flow.ParseXMLConfig([]byte(xmlContent))
	if err != nil {
		t.Fatalf("flow.ParseXMLConfig failed to parse generated XML: %v\nGenerated XML:\n%s", err, xmlContent)
	}
	if len(cfg.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(cfg.Variables))
	}
	// Verify that the unmarshaled value in Flow has the raw '&' character as expected by connection strings
	if cfg.Variables[0].Value != csRaw {
		t.Errorf("expected unmarshaled variable value to be raw string %q, got %q", csRaw, cfg.Variables[0].Value)
	}

	// 3. Test handleExecuteStream in builder draft mode
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "builder.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	script, err := storage.CreateScript("draft_run_test", "Draft test")
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	_, err = storage.AddNode(script.ID, "variables", "variable", map[string]string{
		"name":  "Database1ConnStr",
		"value": csRaw,
		"type":  "string",
	}, "")
	if err != nil {
		t.Fatalf("failed to add variable node: %v", err)
	}
	_, err = storage.AddNode(script.ID, "databases", "database", map[string]string{
		"name":              "database1",
		"driver":            "sqlserver",
		"connection_string": "{{Database1ConnStr}}",
	}, "")
	if err != nil {
		t.Fatalf("failed to add database node: %v", err)
	}

	server, err := NewServer(storage, 8080)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	draftExportFile := filepath.Join(tmpDir, "exported_run_script.xml")
	reqStream := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/execute/stream?source=builder&script_id=%d&file=%s", script.ID, draftExportFile), nil)
	recStream := httptest.NewRecorder()

	// Execute stream handler
	server.handleExecuteStream(recStream, reqStream)

	// Verify the exported file was written and is valid XML
	writtenBytes, err := os.ReadFile(draftExportFile)
	if err != nil {
		t.Fatalf("expected draft XML file to be written: %v", err)
	}
	if strings.Contains(string(writtenBytes), "&trustServerCertificate") {
		t.Errorf("exported file should not contain unescaped &trustServerCertificate: %s", string(writtenBytes))
	}
	parsedExportCfg, err := flow.ParseXMLConfig(writtenBytes)
	if err != nil {
		t.Fatalf("flow.ParseXMLConfig failed on exported builder draft: %v\nExported XML:\n%s", err, string(writtenBytes))
	}
	if len(parsedExportCfg.Variables) != 1 || parsedExportCfg.Variables[0].Value != csRaw {
		t.Errorf("unexpected parsed variables from exported builder draft: %+v", parsedExportCfg.Variables)
	}
}

func TestExecuteStreamPreflightOption(t *testing.T) {
	tmpDir := t.TempDir()
	testScript := filepath.Join(tmpDir, "test_preflight.xml")
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<pipeline name="preflight_test">
    <variables>
        <variable name="foo" value="bar"/>
    </variables>
    <preflight>
        <sql id="pre_1" description="preflight check">SELECT 1;</sql>
    </preflight>
    <flow>
        <sql id="flow_1" description="flow task">SELECT 2;</sql>
    </flow>
</pipeline>`
	if err := os.WriteFile(testScript, []byte(xmlContent), 0644); err != nil {
		t.Fatalf("failed to write test XML: %v", err)
	}

	storage, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Preflight enabled: verify -preflight flag is passed in execution args
	reqPreflight := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file="+testScript+"&preflight=true", nil)
	recPreflight := httptest.NewRecorder()
	server.handleExecuteStream(recPreflight, reqPreflight)

	bodyPreflight := recPreflight.Body.String()
	if !strings.Contains(bodyPreflight, "-preflight") {
		t.Errorf("expected execution log to contain -preflight flag, got:\n%s", bodyPreflight)
	}

	// 2. Preflight disabled/omitted: verify -preflight flag is NOT passed
	reqNormal := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file="+testScript, nil)
	recNormal := httptest.NewRecorder()
	server.handleExecuteStream(recNormal, reqNormal)

	bodyNormal := recNormal.Body.String()
	if strings.Contains(bodyNormal, "-preflight") {
		t.Errorf("expected execution log NOT to contain -preflight flag when omitted, got:\n%s", bodyNormal)
	}

	// 3. Preflight with builder draft mode
	script, err := storage.CreateScript("Draft Script", "Draft Description")
	if err != nil {
		t.Fatalf("failed to create draft script: %v", err)
	}
	_, err = storage.AddNode(script.ID, "preflight", "sql", map[string]string{"id": "draft_preflight_1"}, "SELECT 1")
	if err != nil {
		t.Fatalf("failed to add preflight node to draft: %v", err)
	}

	draftExportFile := filepath.Join(tmpDir, "draft_preflight_exported.xml")
	reqDraftPreflight := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/execute/stream?source=builder&script_id=%d&file=%s&preflight=true", script.ID, draftExportFile), nil)
	recDraftPreflight := httptest.NewRecorder()
	server.handleExecuteStream(recDraftPreflight, reqDraftPreflight)

	bodyDraftPreflight := recDraftPreflight.Body.String()
	if !strings.Contains(bodyDraftPreflight, "-preflight") {
		t.Errorf("expected builder draft execution log to contain -preflight flag, got:\n%s", bodyDraftPreflight)
	}

	// 4. Verify template rendering contains preflight runner UI controls
	reqIndex := httptest.NewRequest(http.MethodGet, "/", nil)
	recIndex := httptest.NewRecorder()
	server.handleIndex(recIndex, reqIndex)
	if recIndex.Code != http.StatusOK {
		t.Fatalf("handleIndex failed: %d", recIndex.Code)
	}
	indexHTML := recIndex.Body.String()
	if !strings.Contains(indexHTML, "runner-preflight") {
		t.Errorf("expected index HTML to contain runner-preflight checkbox")
	}
	if !strings.Contains(indexHTML, "preflight-btn") {
		t.Errorf("expected index HTML to contain preflight-btn")
	}
	if !strings.Contains(indexHTML, "updatePreflightUI") {
		t.Errorf("expected index HTML to contain updatePreflightUI JS function")
	}
	if !strings.Contains(indexHTML, "-preflight") {
		t.Errorf("expected index HTML to mention -preflight flag")
	}
}

func TestUpdateNodeEndpointAndSaveComponent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_update_node.db")

	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	sc, err := storage.CreateScript("SaveTest", "Pipeline for save component testing")
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	node, err := storage.AddNode(sc.ID, "flow", "sql", map[string]string{"id": "OldId", "db": "main"}, "SELECT 1;")
	if err != nil {
		t.Fatalf("failed to add initial node: %v", err)
	}

	// 1. Mutating POST without X-Requested-With should be rejected with 403 Forbidden
	updatePayload := map[string]interface{}{
		"script_id":  sc.ID,
		"node_id":    node.ID,
		"node_type":  "sql",
		"section":    "preflight",
		"attributes": map[string]string{"id": "NewId", "db": "analytics"},
		"content":    "SELECT 42;",
	}
	payloadBytes, _ := json.Marshal(updatePayload)

	reqForbidden := httptest.NewRequest(http.MethodPost, "/api/nodes/update", bytes.NewReader(payloadBytes))
	reqForbidden.Header.Set("Content-Type", "application/json")
	recForbidden := httptest.NewRecorder()
	server.handleUpdateNode(recForbidden, reqForbidden)

	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden without CSRF header, got: %d", recForbidden.Code)
	}

	// 2. Mutating POST with X-Requested-With: XMLHttpRequest should succeed and update attributes, section, and content
	reqValid := httptest.NewRequest(http.MethodPost, "/api/nodes/update", bytes.NewReader(payloadBytes))
	reqValid.Header.Set("Content-Type", "application/json")
	reqValid.Header.Set("X-Requested-With", "XMLHttpRequest")
	recValid := httptest.NewRecorder()
	server.handleUpdateNode(recValid, reqValid)

	if recValid.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with CSRF header, got: %d, body: %s", recValid.Code, recValid.Body.String())
	}

	// Verify persistence in storage
	updatedNode, err := storage.GetNode(node.ID)
	if err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}
	if updatedNode.Section != "preflight" {
		t.Errorf("expected section 'preflight', got '%s'", updatedNode.Section)
	}
	if updatedNode.Attributes["id"] != "NewId" || updatedNode.Attributes["db"] != "analytics" {
		t.Errorf("unexpected attributes: %+v", updatedNode.Attributes)
	}
	if updatedNode.ContentText != "SELECT 42;" {
		t.Errorf("expected content 'SELECT 42;', got '%s'", updatedNode.ContentText)
	}

	// 3. Test template files contain X-Requested-With and proper error handling in fetch
	indexBytes, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatalf("failed to read tmpl/index.html: %v", err)
	}
	indexStr := string(indexBytes)

	if !strings.Contains(indexStr, "options.headers.set('X-Requested-With', 'XMLHttpRequest')") {
		t.Errorf("expected tmpl/index.html to set X-Requested-With in window.fetch interceptor")
	}
	if !strings.Contains(indexStr, "alert('Save failed: ' + (err.message || err))") {
		t.Errorf("expected tmpl/index.html to alert errors in submitNodeModal")
	}

	if !strings.Contains(IndexHTML, "options.headers.set('X-Requested-With', 'XMLHttpRequest')") {
		t.Errorf("expected IndexHTML in html_content.go to set X-Requested-With in window.fetch interceptor")
	}
	if !strings.Contains(IndexHTML, "alert('Save failed: ' + (err.message || err))") {
		t.Errorf("expected IndexHTML in html_content.go to alert errors in submitNodeModal")
	}
}

func TestXMLPreviewAdjustablePanel(t *testing.T) {
	// Verify template files contain resizer, panel, and functions
	indexBytes, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatalf("failed to read tmpl/index.html: %v", err)
	}
	tmplStr := string(indexBytes)

	requiredSnippets := []string{
		`id="xml-preview-resizer"`,
		`id="xml-preview-panel"`,
		`id="xml-preview-reset-btn"`,
		`id="xml-preview-toggle-btn"`,
		`initXMLPreviewResizer()`,
		`resetXMLPreviewWidth()`,
		`toggleXMLPreviewCollapse()`,
		`collapseXMLPreview(`,
		`expandXMLPreview()`,
		`flow_builder_preview_width`,
		`cursor-col-resize`,
		`resizing-active`,
		`/flow-mascot.jpg`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(tmplStr, snippet) {
			t.Errorf("expected tmpl/index.html to contain %q", snippet)
		}
		if !strings.Contains(IndexHTML, snippet) {
			t.Errorf("expected IndexHTML in html_content.go to contain %q", snippet)
		}
	}

	// Verify server renders index with resizer and panel
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_preview_panel.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	srv, err := NewServer(storage, 8089)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.handleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleIndex failed: %d", rec.Code)
	}

	body := rec.Body.String()
	for _, snippet := range requiredSnippets {
		if !strings.Contains(body, snippet) {
			t.Errorf("expected rendered page to contain %q", snippet)
		}
	}
}

func TestSoftWhiteTheme(t *testing.T) {
	// Verify tmpl/index.html and IndexHTML contain soft-white theme
	indexBytes, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatalf("failed to read tmpl/index.html: %v", err)
	}
	tmplStr := string(indexBytes)

	requiredSnippets := []string{
		`html[data-theme="soft-white"]`,
		`<option value="soft-white"`,
		`Soft White (Blue & Black)`,
		`background-color: #f8fafc !important`,
		`color: #0f172a !important`,
		`color: #1d4ed8 !important`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(tmplStr, snippet) {
			t.Errorf("expected tmpl/index.html to contain %q", snippet)
		}
		if !strings.Contains(IndexHTML, snippet) {
			t.Errorf("expected IndexHTML to contain %q", snippet)
		}
	}

	// Verify server renders index with soft-white theme option and CSS
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_soft_white_theme.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	srv, err := NewServer(storage, 8089)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.handleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleIndex failed: %d", rec.Code)
	}

	body := rec.Body.String()
	for _, snippet := range requiredSnippets {
		if !strings.Contains(body, snippet) {
			t.Errorf("expected rendered page to contain %q", snippet)
		}
	}
}

func TestSavedDatabaseConnectionStorageCRUD(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_db_conns.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Initial list should be empty
	conns, err := storage.ListSavedDatabaseConnections()
	if err != nil {
		t.Fatalf("ListSavedDatabaseConnections error: %v", err)
	}
	if len(conns) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(conns))
	}

	// Create a new connection
	newConn := &SavedDatabaseConnection{
		Name:             "test_pg",
		Driver:           "postgres",
		ConnectionString: "postgresql://user:pass@localhost:5432/testdb?sslmode=disable",
		MaxOpenConns:     15,
		MaxIdleConns:     7,
		Description:      "Main logging database",
	}

	saved, err := storage.SaveDatabaseConnection(newConn)
	if err != nil {
		t.Fatalf("SaveDatabaseConnection error: %v", err)
	}
	if saved.ID <= 0 {
		t.Fatalf("expected positive ID, got %d", saved.ID)
	}
	if saved.Name != "test_pg" {
		t.Fatalf("expected name 'test_pg', got %q", saved.Name)
	}

	// Retrieve by ID
	byID, err := storage.GetSavedDatabaseConnection(saved.ID)
	if err != nil {
		t.Fatalf("GetSavedDatabaseConnection error: %v", err)
	}
	if byID == nil || byID.ConnectionString != newConn.ConnectionString {
		t.Fatalf("unexpected conn retrieved: %+v", byID)
	}

	// Retrieve by Name
	byName, err := storage.GetSavedDatabaseConnectionByName("test_pg")
	if err != nil {
		t.Fatalf("GetSavedDatabaseConnectionByName error: %v", err)
	}
	if byName == nil || byName.ID != saved.ID {
		t.Fatalf("unexpected conn retrieved by name: %+v", byName)
	}

	// Update existing connection
	saved.Description = "Updated description"
	saved.MaxOpenConns = 25
	updated, err := storage.SaveDatabaseConnection(saved)
	if err != nil {
		t.Fatalf("SaveDatabaseConnection update error: %v", err)
	}
	if updated.Description != "Updated description" || updated.MaxOpenConns != 25 {
		t.Fatalf("updated fields mismatch: %+v", updated)
	}

	// List connections should contain 1
	all, err := storage.ListSavedDatabaseConnections()
	if err != nil {
		t.Fatalf("ListSavedDatabaseConnections error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(all))
	}

	// Delete connection
	if err := storage.DeleteSavedDatabaseConnection(saved.ID); err != nil {
		t.Fatalf("DeleteSavedDatabaseConnection error: %v", err)
	}

	// List connections should be empty again
	allAfterDelete, err := storage.ListSavedDatabaseConnections()
	if err != nil {
		t.Fatalf("ListSavedDatabaseConnections after delete error: %v", err)
	}
	if len(allAfterDelete) != 0 {
		t.Fatalf("expected 0 connections after delete, got %d", len(allAfterDelete))
	}
}

func TestPurgeDatabaseWithSavedConnections(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_purge_db_conns.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Add a connection
	_, err = storage.SaveDatabaseConnection(&SavedDatabaseConnection{
		Name:             "analytics_db",
		Driver:           "mysql",
		ConnectionString: "root:pass@tcp(127.0.0.1:3306)/analytics",
	})
	if err != nil {
		t.Fatalf("SaveDatabaseConnection error: %v", err)
	}

	// Purge database
	if _, err := storage.PurgeDatabase(); err != nil {
		t.Fatalf("PurgeDatabase error: %v", err)
	}

	// Saved database connections should be cleared
	conns, err := storage.ListSavedDatabaseConnections()
	if err != nil {
		t.Fatalf("ListSavedDatabaseConnections after purge error: %v", err)
	}
	if len(conns) != 0 {
		t.Fatalf("expected 0 connections after purge, got %d", len(conns))
	}
}

func TestSettingsDatabaseHTTPHandlers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_settings_http.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	srv, err := NewServer(storage, 8091)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	srv.authToken = ""

	handler := srv.Handler()

	// 1. GET /api/settings/databases (initially empty)
	req1 := httptest.NewRequest(http.MethodGet, "/api/settings/databases", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var list1 []SavedDatabaseConnection
	if err := json.Unmarshal(rec1.Body.Bytes(), &list1); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(list1) != 0 {
		t.Fatalf("expected empty list, got %d items", len(list1))
	}

	// 2. POST /api/settings/databases/save (create)
	savePayload := map[string]interface{}{
		"name":              "audit_sqlite",
		"driver":            "sqlite",
		"connection_string": ":memory:",
		"description":       "In-memory sqlite for telemetry",
		"max_open_conns":    5,
		"max_idle_conns":    2,
	}
	bodyBytes, _ := json.Marshal(savePayload)
	req2 := httptest.NewRequest(http.MethodPost, "/api/settings/databases/save", bytes.NewReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200 on save, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var saveResult struct {
		Success    bool                    `json:"success"`
		Message    string                  `json:"message"`
		Connection SavedDatabaseConnection `json:"connection"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &saveResult); err != nil {
		t.Fatalf("failed to unmarshal saved response: %v", err)
	}
	savedResp := saveResult.Connection
	if savedResp.ID <= 0 || savedResp.Name != "audit_sqlite" {
		t.Fatalf("unexpected saved response: %+v", savedResp)
	}

	// 3. POST /api/settings/databases/test (ping in-memory sqlite)
	testPayload := map[string]interface{}{
		"id": savedResp.ID,
	}
	testBytes, _ := json.Marshal(testPayload)
	req3 := httptest.NewRequest(http.MethodPost, "/api/settings/databases/test", bytes.NewReader(testBytes))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected status 200 on test, got %d: %s", rec3.Code, rec3.Body.String())
	}
	var testResp map[string]interface{}
	if err := json.Unmarshal(rec3.Body.Bytes(), &testResp); err != nil {
		t.Fatalf("failed to unmarshal test response: %v", err)
	}
	if testResp["success"] != true {
		t.Fatalf("expected test ping success, got: %+v", testResp)
	}

	// 4. POST /api/settings/databases/delete
	delPayload := map[string]interface{}{
		"id": savedResp.ID,
	}
	delBytes, _ := json.Marshal(delPayload)
	req4 := httptest.NewRequest(http.MethodPost, "/api/settings/databases/delete", bytes.NewReader(delBytes))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected status 200 on delete, got %d: %s", rec4.Code, rec4.Body.String())
	}

	// 5. Verify list is empty again
	req5 := httptest.NewRequest(http.MethodGet, "/api/settings/databases", nil)
	rec5 := httptest.NewRecorder()
	handler.ServeHTTP(rec5, req5)
	var list2 []SavedDatabaseConnection
	if err := json.Unmarshal(rec5.Body.Bytes(), &list2); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(list2) != 0 {
		t.Fatalf("expected 0 connections after delete, got %d", len(list2))
	}
}

func TestSettingsTabUIRendering(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_settings_ui.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	srv, err := NewServer(storage, 8092)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.handleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleIndex failed: %d", rec.Code)
	}

	body := rec.Body.String()
	expectedUIElements := []string{
		`id="tab-btn-settings"`,
		`switchTab('settings')`,
		`id="tab-settings"`,
		`id="saved-databases-tbody"`,
		`id="database-modal"`,
		`confirmPurgeDatabase()`,
		`Purge SQLite Database`,
		`pipeline_runs`,
		`pipeline_events`,
	}

	for _, elem := range expectedUIElements {
		if !strings.Contains(body, elem) {
			t.Errorf("expected HTML index page to contain %q", elem)
		}
	}
}

func TestLogPageTelemetryQueries(t *testing.T) {
	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "audit_telemetry.db")

	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	// Create tables according to standard audit schema
	createRunsSQL := `
	CREATE TABLE pipeline_runs (
		run_id TEXT PRIMARY KEY,
		file_path TEXT,
		config_path TEXT,
		status TEXT,
		started_at TEXT,
		finished_at TEXT,
		duration_ms INTEGER,
		task_count INTEGER,
		user_name TEXT,
		hostname TEXT,
		options_path TEXT,
		error_class TEXT,
		error_message TEXT
	);`
	if _, err := db.Exec(createRunsSQL); err != nil {
		t.Fatalf("failed to create pipeline_runs table: %v", err)
	}

	createEventsSQL := `
	CREATE TABLE pipeline_events (
		run_id TEXT,
		execution_id TEXT,
		sequence_num INTEGER,
		occurred_at TEXT,
		event_type TEXT,
		node_kind TEXT,
		node_id TEXT,
		status TEXT,
		user_name TEXT,
		hostname TEXT,
		options_path TEXT,
		error_message TEXT,
		rows_read INTEGER,
		rows_written INTEGER,
		rows_affected INTEGER
	);`
	if _, err := db.Exec(createEventsSQL); err != nil {
		t.Fatalf("failed to create pipeline_events table: %v", err)
	}

	// Insert test runs
	insertRun := `INSERT INTO pipeline_runs (
		run_id, file_path, config_path, status, started_at, finished_at, duration_ms, task_count, user_name, hostname, options_path, error_class, error_message
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	now := time.Now().UTC()
	start1 := now.Add(-10 * time.Minute).Format(time.RFC3339)
	end1 := now.Add(-8 * time.Minute).Format(time.RFC3339)
	_, err = db.Exec(insertRun, "run-101", "scripts/daily_etl.xml", "config/prod.xml", "succeeded", start1, end1, 120000, 3, "alice", "srv-app-01", "", "", "")
	if err != nil {
		t.Fatalf("failed to insert run-101: %v", err)
	}

	start2 := now.Add(-5 * time.Minute).Format(time.RFC3339)
	end2 := now.Add(-4 * time.Minute).Format(time.RFC3339)
	_, err = db.Exec(insertRun, "run-102", "scripts/hourly_sync.xml", "", "failed", start2, end2, 60000, 2, "bob", "srv-app-02", "", "AssertionError", "expected 100 rows, got 0")
	if err != nil {
		t.Fatalf("failed to insert run-102: %v", err)
	}

	// Insert test events
	insertEvent := `INSERT INTO pipeline_events (
		run_id, execution_id, sequence_num, occurred_at, event_type, node_kind, node_id, status, user_name, hostname, options_path, error_message, rows_read, rows_written, rows_affected
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	_, err = db.Exec(insertEvent, "run-101", "exec-1", 1, start1, "step_start", "sql", "extract_orders", "success", "alice", "srv-app-01", "", "", 1000, 0, 0)
	if err != nil {
		t.Fatalf("failed to insert event 1: %v", err)
	}
	_, err = db.Exec(insertEvent, "run-101", "exec-1", 2, end1, "step_complete", "sql", "load_dw", "success", "alice", "srv-app-01", "", "", 0, 1000, 1000)
	if err != nil {
		t.Fatalf("failed to insert event 2: %v", err)
	}
	_, err = db.Exec(insertEvent, "run-102", "exec-2", 1, start2, "step_fail", "assert", "check_metrics", "failure", "bob", "srv-app-02", "", "expected 100 rows, got 0", 0, 0, 0)
	if err != nil {
		t.Fatalf("failed to insert event 3: %v", err)
	}

	ctx := context.Background()

	// 1. Query all runs
	runs, err := QueryPipelineRuns(ctx, db, "sqlite", TablePipelineRuns, "", "", 10)
	if err != nil {
		t.Fatalf("QueryPipelineRuns failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	// 2. Query with status filter
	succRuns, err := QueryPipelineRuns(ctx, db, "sqlite", TablePipelineRuns, "", "succeeded", 10)
	if err != nil {
		t.Fatalf("QueryPipelineRuns with status failed: %v", err)
	}
	if len(succRuns) != 1 || succRuns[0].RunID != "run-101" {
		t.Errorf("expected 1 succeeded run (run-101), got: %+v", succRuns)
	}

	// 3. Query with text search
	searchRuns, err := QueryPipelineRuns(ctx, db, "sqlite", TablePipelineRuns, "AssertionError", "", 10)
	if err != nil {
		t.Fatalf("QueryPipelineRuns with search failed: %v", err)
	}
	if len(searchRuns) != 1 || searchRuns[0].RunID != "run-102" {
		t.Errorf("expected 1 search run (run-102), got: %+v", searchRuns)
	}

	// 4. Query events for run-101
	events101, err := QueryPipelineEvents(ctx, db, "sqlite", TablePipelineEvents, "run-101")
	if err != nil {
		t.Fatalf("QueryPipelineEvents failed: %v", err)
	}
	if len(events101) != 2 {
		t.Fatalf("expected 2 events for run-101, got %d", len(events101))
	}
	if events101[0].SequenceNum != 1 || events101[1].SequenceNum != 2 {
		t.Errorf("expected sequence numbers 1 and 2, got: %d, %d", events101[0].SequenceNum, events101[1].SequenceNum)
	}
	if events101[0].RowsRead != 1000 || events101[1].RowsAffected != 1000 {
		t.Errorf("row metrics mismatch: %+v, %+v", events101[0], events101[1])
	}

	// 5. Query table not found
	_, err = QueryPipelineRuns(ctx, db, "sqlite", "nonexistent_table", "", "", 10)
	if !errors.Is(err, ErrTableNotFound) {
		t.Errorf("expected ErrTableNotFound, got: %v", err)
	}
}

func TestLogPageHTTPHandlers(t *testing.T) {
	tempDir := t.TempDir()
	auditDbFile := filepath.Join(tempDir, "audit_telemetry_http.db")

	auditDB, err := sql.Open("sqlite", auditDbFile)
	if err != nil {
		t.Fatalf("failed to open audit sqlite DB: %v", err)
	}
	defer auditDB.Close()

	_, err = auditDB.Exec(`
	CREATE TABLE pipeline_runs (
		run_id TEXT PRIMARY KEY,
		file_path TEXT,
		config_path TEXT,
		status TEXT,
		started_at TEXT,
		finished_at TEXT,
		duration_ms INTEGER,
		task_count INTEGER,
		user_name TEXT,
		hostname TEXT,
		options_path TEXT,
		error_class TEXT,
		error_message TEXT
	);
	CREATE TABLE pipeline_events (
		run_id TEXT,
		execution_id TEXT,
		sequence_num INTEGER,
		occurred_at TEXT,
		event_type TEXT,
		node_kind TEXT,
		node_id TEXT,
		status TEXT,
		user_name TEXT,
		hostname TEXT,
		options_path TEXT,
		error_message TEXT,
		rows_read INTEGER,
		rows_written INTEGER,
		rows_affected INTEGER
	);
	INSERT INTO pipeline_runs (run_id, file_path, status, duration_ms, user_name) VALUES ('run-http-1', 'main.xml', 'succeeded', 4500, 'devuser');
	INSERT INTO pipeline_events (run_id, execution_id, sequence_num, event_type, status, rows_read) VALUES ('run-http-1', 'exec-1', 1, 'step_complete', 'success', 250);
	`)
	if err != nil {
		t.Fatalf("failed to seed audit DB: %v", err)
	}

	builderDbFile := filepath.Join(tempDir, "builder_storage.db")
	storage, err := NewStorage(builderDbFile)
	if err != nil {
		t.Fatalf("failed to init builder storage: %v", err)
	}
	defer storage.Close()

	// Save the audit database as a connection
	savedConn, err := storage.SaveDatabaseConnection(&SavedDatabaseConnection{
		Name:             "Audit DB",
		Driver:           "sqlite",
		ConnectionString: auditDbFile,
		Description:      "Test Audit DB",
		MaxOpenConns:     5,
		MaxIdleConns:     2,
	})
	if err != nil {
		t.Fatalf("failed to save db connection: %v", err)
	}

	srv, err := NewServer(storage, 8093)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. GET /api/logs/runs without db_id -> 400
	reqNoID := httptest.NewRequest(http.MethodGet, "/api/logs/runs", nil)
	recNoID := httptest.NewRecorder()
	srv.handleListLogRuns(recNoID, reqNoID)
	if recNoID.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing db_id, got %d", recNoID.Code)
	}

	// 2. GET /api/logs/runs with non-existent db_id -> 404
	req404 := httptest.NewRequest(http.MethodGet, "/api/logs/runs?db_id=99999", nil)
	rec404 := httptest.NewRecorder()
	srv.handleListLogRuns(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent db_id, got %d", rec404.Code)
	}

	// 3. GET /api/logs/runs with valid db_id -> 200 and runs array
	reqValid := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/logs/runs?db_id=%d", savedConn.ID), nil)
	recValid := httptest.NewRecorder()
	srv.handleListLogRuns(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recValid.Code, recValid.Body.String())
	}

	var runsResp struct {
		Success     bool                `json:"success"`
		TableExists bool                `json:"table_exists"`
		Runs        []PipelineRunRecord `json:"runs"`
		Count       int                 `json:"count"`
	}
	if err := json.Unmarshal(recValid.Body.Bytes(), &runsResp); err != nil {
		t.Fatalf("failed to parse runs response: %v", err)
	}
	if !runsResp.Success || !runsResp.TableExists || runsResp.Count != 1 || len(runsResp.Runs) != 1 {
		t.Errorf("unexpected runs response: %+v", runsResp)
	}
	if runsResp.Runs[0].RunID != "run-http-1" {
		t.Errorf("expected run-http-1, got %s", runsResp.Runs[0].RunID)
	}

	// 4. GET /api/logs/events with valid run_id -> 200 and events array
	reqEvt := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/logs/events?db_id=%d&run_id=run-http-1", savedConn.ID), nil)
	recEvt := httptest.NewRecorder()
	srv.handleListLogEvents(recEvt, reqEvt)
	if recEvt.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recEvt.Code, recEvt.Body.String())
	}

	var evtResp struct {
		Success     bool                  `json:"success"`
		TableExists bool                  `json:"table_exists"`
		Events      []PipelineEventRecord `json:"events"`
		Count       int                   `json:"count"`
	}
	if err := json.Unmarshal(recEvt.Body.Bytes(), &evtResp); err != nil {
		t.Fatalf("failed to parse events response: %v", err)
	}
	if !evtResp.Success || !evtResp.TableExists || evtResp.Count != 1 || len(evtResp.Events) != 1 {
		t.Errorf("unexpected events response: %+v", evtResp)
	}
	if evtResp.Events[0].RowsRead != 250 {
		t.Errorf("expected 250 rows read, got %d", evtResp.Events[0].RowsRead)
	}
}

func TestLogPageUIRendering(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_log_ui.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	srv, err := NewServer(storage, 8094)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.handleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleIndex failed: %d", rec.Code)
	}

	body := rec.Body.String()
	expectedUIElements := []string{
		`id="tab-btn-logs"`,
		`switchTab('logs')`,
		`id="tab-logs"`,
		`id="log-db-select"`,
		`id="log-refresh-btn"`,
		`id="log-runs-tbody"`,
		`id="log-events-tbody"`,
		`id="event-detail-modal"`,
		`pipeline_runs`,
		`pipeline_events`,
		`initLogPage()`,
	}

	for _, elem := range expectedUIElements {
		if !strings.Contains(body, elem) {
			t.Errorf("expected HTML index page to contain %q", elem)
		}
	}
}

func TestPipelineMetadataRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metadata.db")

	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	originalName := "MyDataPipeline"
	originalDesc := "Pipeline processing sales data"

	sc, err := storage.CreateScript(originalName, originalDesc)
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	_, err = storage.AddNode(sc.ID, "flow", "sql", map[string]string{"id": "step1"}, "SELECT 42;")
	if err != nil {
		t.Fatalf("failed to add node: %v", err)
	}

	exportedXML, err := storage.ExportPipelineXML(sc.Name)
	if err != nil {
		t.Fatalf("failed to export script to XML: %v", err)
	}

	if !strings.Contains(exportedXML, `name="MyDataPipeline"`) {
		t.Errorf("exported XML missing name attribute: %s", exportedXML)
	}
	if !strings.Contains(exportedXML, `description="Pipeline processing sales data"`) {
		t.Errorf("exported XML missing description attribute: %s", exportedXML)
	}

	// Re-import into a new storage database
	dbPath2 := filepath.Join(tmpDir, "test_reimport.db")
	storage2, err := NewStorage(dbPath2)
	if err != nil {
		t.Fatalf("failed to create storage 2: %v", err)
	}
	defer storage2.Close()

	importedScript, err := ImportPipelineFromBytes([]byte(exportedXML), "", "", "", storage2)
	if err != nil {
		t.Fatalf("failed to re-import pipeline: %v", err)
	}

	if importedScript.Name != originalName {
		t.Errorf("expected script name %q, got %q", originalName, importedScript.Name)
	}
	if importedScript.Description != originalDesc {
		t.Errorf("expected script description %q, got %q", originalDesc, importedScript.Description)
	}
}

func TestSanitizePathAndExecutionSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 8080)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Test sanitizePath rejects directory traversal
	if _, err := server.sanitizePath("../../escaped"); err == nil {
		t.Error("expected error for directory traversal path in sanitizePath, got nil")
	}

	// 2. Test isAllowedScriptWritePath rejects directory traversal
	if server.isAllowedScriptWritePath("../../evil.xml") {
		t.Error("expected isAllowedScriptWritePath to reject ../../evil.xml")
	}
	if !server.isAllowedScriptWritePath("safe_run_script.xml") {
		t.Error("expected isAllowedScriptWritePath to accept relative safe_run_script.xml")
	}
	if !server.isAllowedScriptWritePath(filepath.Join(os.TempDir(), "temp_script.xml")) {
		t.Error("expected isAllowedScriptWritePath to accept file inside os.TempDir()")
	}

	// 3. Test handleExecuteStream rejects cross-site requests
	reqCrossSite := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file=test.xml", nil)
	reqCrossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	recCrossSite := httptest.NewRecorder()
	server.handleExecuteStream(recCrossSite, reqCrossSite)
	if recCrossSite.Code != http.StatusForbidden {
		t.Errorf("expected 403 for Sec-Fetch-Site: cross-site, got: %d", recCrossSite.Code)
	}

	// 4. Test handleExecuteStream rejects cross-origin requests
	reqCrossOrigin := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file=test.xml", nil)
	reqCrossOrigin.Header.Set("Origin", "http://malicious.site")
	reqCrossOrigin.Host = "127.0.0.1:8080"
	recCrossOrigin := httptest.NewRecorder()
	server.handleExecuteStream(recCrossOrigin, reqCrossOrigin)
	if recCrossOrigin.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-origin request, got: %d", recCrossOrigin.Code)
	}

	// 5. Test handleExecuteStream rejects path traversal when writing builder draft
	script, err := storage.CreateScript("Draft Security Test", "Testing script path security")
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}
	reqBadPath := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/execute/stream?source=builder&script_id=%d&file=../../traversal.xml", script.ID), nil)
	recBadPath := httptest.NewRecorder()
	server.handleExecuteStream(recBadPath, reqBadPath)
	respBody := recBadPath.Body.String()
	if !strings.Contains(respBody, "access denied") {
		t.Errorf("expected access denied error in SSE response for path traversal, got: %s", respBody)
	}

	// 6. Test handleExecuteStream rejects cross-origin Referer when Origin is omitted
	reqCrossReferer := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file=test.xml", nil)
	reqCrossReferer.Header.Set("Referer", "http://attacker.com/dashboard")
	reqCrossReferer.Host = "127.0.0.1:8080"
	recCrossReferer := httptest.NewRecorder()
	server.handleExecuteStream(recCrossReferer, reqCrossReferer)
	if recCrossReferer.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-origin Referer, got: %d", recCrossReferer.Code)
	}

	// 7. Test handleExecuteStream rejects path traversal in filesystem mode
	reqFsTraversal := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file=../../windows/system.xml", nil)
	recFsTraversal := httptest.NewRecorder()
	server.handleExecuteStream(recFsTraversal, reqFsTraversal)
	respFs := recFsTraversal.Body.String()
	if !strings.Contains(respFs, "Access denied") {
		t.Errorf("expected Access denied for filesystem mode traversal, got: %s", respFs)
	}
}

func TestExecuteStreamFormatOptionWithoutOptions(t *testing.T) {
	tmpDir := t.TempDir()
	testScript := filepath.Join(tmpDir, "test_format.xml")
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<pipeline name="format_test">
    <flow>
        <sql id="flow_1" description="flow task">SELECT 1;</sql>
    </flow>
</pipeline>`
	if err := os.WriteFile(testScript, []byte(xmlContent), 0644); err != nil {
		t.Fatalf("failed to write test XML: %v", err)
	}

	storage, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Without options file and format=jsonpretty: args must contain -format jsonpretty
	reqJSONPretty := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file="+testScript+"&format=jsonpretty", nil)
	recJSONPretty := httptest.NewRecorder()
	server.handleExecuteStream(recJSONPretty, reqJSONPretty)
	bodyJSONPretty := recJSONPretty.Body.String()
	if !strings.Contains(bodyJSONPretty, "-format jsonpretty") {
		t.Errorf("expected execution log to contain '-format jsonpretty', got:\n%s", bodyJSONPretty)
	}
	if strings.Contains(bodyJSONPretty, "-options") {
		t.Errorf("expected execution log NOT to contain -options flag, got:\n%s", bodyJSONPretty)
	}

	// 2. Without options file and format=markdown: args must contain -format markdown
	reqMD := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file="+testScript+"&format=markdown", nil)
	recMD := httptest.NewRecorder()
	server.handleExecuteStream(recMD, reqMD)
	bodyMD := recMD.Body.String()
	if !strings.Contains(bodyMD, "-format markdown") {
		t.Errorf("expected execution log to contain '-format markdown', got:\n%s", bodyMD)
	}

	// 3. Without options file and format=csv: args must contain -format csv
	reqCSV := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file="+testScript+"&format=csv", nil)
	recCSV := httptest.NewRecorder()
	server.handleExecuteStream(recCSV, reqCSV)
	bodyCSV := recCSV.Body.String()
	if !strings.Contains(bodyCSV, "-format csv") {
		t.Errorf("expected execution log to contain '-format csv', got:\n%s", bodyCSV)
	}

	// 4. Without options file and format omitted: defaults to stream
	reqDefault := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file="+testScript, nil)
	recDefault := httptest.NewRecorder()
	server.handleExecuteStream(recDefault, reqDefault)
	bodyDefault := recDefault.Body.String()
	if !strings.Contains(bodyDefault, "-format stream") {
		t.Errorf("expected execution log to default to '-format stream', got:\n%s", bodyDefault)
	}

	// 5. Without options file and unknown format: safely falls back to stream
	reqUnknown := httptest.NewRequest(http.MethodGet, "/api/execute/stream?source=file&file="+testScript+"&format=invalid_format", nil)
	recUnknown := httptest.NewRecorder()
	server.handleExecuteStream(recUnknown, reqUnknown)
	bodyUnknown := recUnknown.Body.String()
	if !strings.Contains(bodyUnknown, "-format stream") {
		t.Errorf("expected execution log for invalid format to fall back to '-format stream', got:\n%s", bodyUnknown)
	}
}

func TestExecuteStreamFormatIgnoredWithOptionsFile(t *testing.T) {
	tmpDir := t.TempDir()
	testScript := filepath.Join(tmpDir, "test_with_options.xml")
	if err := os.WriteFile(testScript, []byte("<pipeline name=\"opt\"><flow><sql id=\"1\">SELECT 1;</sql></flow></pipeline>"), 0644); err != nil {
		t.Fatalf("failed to write test script XML: %v", err)
	}
	testOptions := filepath.Join(tmpDir, "test_options.xml")
	if err := os.WriteFile(testOptions, []byte("<options><format>jsonpretty</format></options>"), 0644); err != nil {
		t.Fatalf("failed to write test options XML: %v", err)
	}

	storage, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// When options file is supplied, -format CLI flag must NOT be appended even if format param is passed
	reqWithOptions := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/execute/stream?source=file&file=%s&options=%s&format=csv", testScript, testOptions), nil)
	recWithOptions := httptest.NewRecorder()
	server.handleExecuteStream(recWithOptions, reqWithOptions)
	bodyWithOptions := recWithOptions.Body.String()

	if !strings.Contains(bodyWithOptions, "-options") {
		t.Errorf("expected execution log to contain -options flag, got:\n%s", bodyWithOptions)
	}
	if strings.Contains(bodyWithOptions, "-format") {
		t.Errorf("expected execution log NOT to contain -format flag when options file is loaded, got:\n%s", bodyWithOptions)
	}
}

func TestExecuteStreamFormatUIElements(t *testing.T) {
	indexBytes, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatalf("failed to read tmpl/index.html: %v", err)
	}
	tmplStr := string(indexBytes)

	requiredSnippets := []string{
		`id="runner-format"`,
		`id="runner-format-hint"`,
		`id="runner-format-badge"`,
		`id="runner-live-badge"`,
		`updateOptionsFileUI()`,
		`function updateOptionsFileUI()`,
		`<option value="stream" selected>stream (Live Event Stream)</option>`,
		`<option value="csv">csv (Comma-Separated)</option>`,
		`<option value="json">json (Compact JSON)</option>`,
		`<option value="jsonpretty">jsonpretty (Formatted JSON)</option>`,
		`<option value="text">text (Plain Text)</option>`,
		`<option value="markdown">markdown (Markdown Table)</option>`,
		`url += '&format=' + encodeURIComponent(selectedFormat);`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(tmplStr, snippet) {
			t.Errorf("expected tmpl/index.html to contain %q", snippet)
		}
		if !strings.Contains(IndexHTML, snippet) {
			t.Errorf("expected IndexHTML in html_content.go to contain %q", snippet)
		}
	}
}

func TestFileBrowsingParentDirBounds(t *testing.T) {
	storage, err := NewStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Browsing at working directory root "." should yield ParentDir == ""
	reqRoot := httptest.NewRequest(http.MethodGet, "/api/files/browse?dir=.", nil)
	recRoot := httptest.NewRecorder()
	server.handleBrowseFiles(recRoot, reqRoot)
	if recRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 browsing root, got %d: %s", recRoot.Code, recRoot.Body.String())
	}
	var respRoot BrowseResponse
	if err := json.Unmarshal(recRoot.Body.Bytes(), &respRoot); err != nil {
		t.Fatalf("failed to unmarshal browse response: %v", err)
	}
	if respRoot.ParentDir != "" {
		t.Errorf("expected ParentDir to be empty when browsing working directory root, got: %q", respRoot.ParentDir)
	}

	// 2. Browsing a subdirectory inside working directory should have non-empty ParentDir pointing to root
	tmpSubDir, err := os.MkdirTemp(".", "test_sub_browse_")
	if err != nil {
		t.Fatalf("failed to create temp sub dir: %v", err)
	}
	defer os.RemoveAll(tmpSubDir)

	reqSub := httptest.NewRequest(http.MethodGet, "/api/files/browse?dir="+tmpSubDir, nil)
	recSub := httptest.NewRecorder()
	server.handleBrowseFiles(recSub, reqSub)
	if recSub.Code != http.StatusOK {
		t.Fatalf("expected 200 browsing sub dir, got %d: %s", recSub.Code, recSub.Body.String())
	}
	var respSub BrowseResponse
	if err := json.Unmarshal(recSub.Body.Bytes(), &respSub); err != nil {
		t.Fatalf("failed to unmarshal sub dir browse response: %v", err)
	}
	if respSub.ParentDir == "" {
		t.Errorf("expected ParentDir to be non-empty when browsing subfolder %s", tmpSubDir)
	}
}

func TestFileBrowserErrorRecoveryUI(t *testing.T) {
	indexBytes, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatalf("failed to read tmpl/index.html: %v", err)
	}
	tmplStr := string(indexBytes)

	requiredSnippets := []string{
		`id="file-browser-error"`,
		`id="file-browser-error-text"`,
		`browserLastGoodDir`,
		`showFileBrowserError(`,
		`dismissFileBrowserError()`,
		`updateFileBrowserUpButton(`,
		`Returned to last known good directory`,
		`returned to ' + fallbackDir`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(tmplStr, snippet) {
			t.Errorf("expected tmpl/index.html to contain %q", snippet)
		}
		if !strings.Contains(IndexHTML, snippet) {
			t.Errorf("expected IndexHTML in html_content.go to contain %q", snippet)
		}
	}
}

func TestDatabaseDriverHighlightOnTest(t *testing.T) {
	indexBytes, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatalf("failed to read tmpl/index.html: %v", err)
	}
	tmplStr := string(indexBytes)

	requiredSnippets := []string{
		`id="driver-badge-' + db.id + '"`,
		`savedDbTestStatuses`,
		`localStorage.getItem('flow_builder_db_test_statuses')`,
		`bg-emerald-950/80 text-emerald-300 border-emerald-700/60`,
		`bg-rose-950/80 text-rose-300 border-rose-700/60`,
		`driverBadge.className = 'px-2 py-0.5 rounded text-[10px] font-mono border bg-emerald-950/80 text-emerald-300 border-emerald-700/60'`,
		`driverBadge.className = 'px-2 py-0.5 rounded text-[10px] font-mono border bg-rose-950/80 text-rose-300 border-rose-700/60'`,
		`delete savedDbTestStatuses[id]`,
		`localStorage.removeItem('flow_builder_db_test_statuses')`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(tmplStr, snippet) {
			t.Errorf("expected tmpl/index.html to contain %q", snippet)
		}
		if !strings.Contains(IndexHTML, snippet) {
			t.Errorf("expected IndexHTML in html_content.go to contain %q", snippet)
		}
	}
}

func TestPipelineEventRecord_SPIDAndDBUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	createRunsSQL := `
	CREATE TABLE pipeline_runs (
		run_id TEXT PRIMARY KEY,
		file_path TEXT,
		config_path TEXT,
		status TEXT,
		started_at TEXT,
		finished_at TEXT,
		duration_ms INTEGER,
		task_count INTEGER,
		user_name TEXT,
		os_user_name TEXT,
		db_user_name TEXT,
		spid INTEGER,
		hostname TEXT,
		options_path TEXT,
		error_class TEXT,
		error_message TEXT
	);`
	if _, err := db.Exec(createRunsSQL); err != nil {
		t.Fatalf("failed to create pipeline_runs table: %v", err)
	}

	createEventsSQL := `
	CREATE TABLE pipeline_events (
		run_id TEXT,
		execution_id TEXT,
		sequence_num INTEGER,
		occurred_at TEXT,
		event_type TEXT,
		node_kind TEXT,
		node_id TEXT,
		status TEXT,
		user_name TEXT,
		os_user_name TEXT,
		db_user_name TEXT,
		spid INTEGER,
		hostname TEXT,
		options_path TEXT,
		error_message TEXT,
		rows_read INTEGER,
		rows_written INTEGER,
		rows_affected INTEGER
	);`
	if _, err := db.Exec(createEventsSQL); err != nil {
		t.Fatalf("failed to create pipeline_events table: %v", err)
	}

	now := time.Now().UTC()
	startStr := now.Format(time.RFC3339)
	endStr := now.Add(time.Minute).Format(time.RFC3339)

	insertRunSQL := `INSERT INTO pipeline_runs (
		run_id, file_path, config_path, status, started_at, finished_at, duration_ms, task_count,
		user_name, os_user_name, db_user_name, spid, hostname, options_path, error_class, error_message
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	_, err = db.Exec(insertRunSQL, "run-spid-100", "test.xml", "", "succeeded", startStr, endStr, 60000, 1,
		"srv_runner", "srv_runner", "analytics_svc", 4242, "host-01", "", "", "")
	if err != nil {
		t.Fatalf("failed to insert run: %v", err)
	}

	insertEventSQL := `INSERT INTO pipeline_events (
		run_id, execution_id, sequence_num, occurred_at, event_type, node_kind, node_id, status,
		user_name, os_user_name, db_user_name, spid, hostname, options_path, error_message, rows_read, rows_written, rows_affected
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	_, err = db.Exec(insertEventSQL, "run-spid-100", "exec-100", 1, startStr, "step_start", "query", "node_1", "success",
		"srv_runner", "srv_runner", "analytics_svc", 4242, "host-01", "", "", 100, 50, 0)
	if err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// 1. Test QueryPipelineEvents
	events, err := QueryPipelineEvents(ctx, db, "sqlite", "pipeline_events", "run-spid-100")
	if err != nil {
		t.Fatalf("QueryPipelineEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt := events[0]
	if evt.SPID != 4242 {
		t.Errorf("expected SPID 4242, got %d", evt.SPID)
	}
	if evt.DBUserName != "analytics_svc" || evt.DatabaseUserName != "analytics_svc" {
		t.Errorf("expected DBUserName 'analytics_svc', got DBUserName=%q DatabaseUserName=%q", evt.DBUserName, evt.DatabaseUserName)
	}
	if evt.OSUserName != "srv_runner" || evt.UserName != "srv_runner" {
		t.Errorf("expected OSUserName 'srv_runner', got OSUserName=%q UserName=%q", evt.OSUserName, evt.UserName)
	}

	// 2. Test QueryPipelineRuns
	runs, err := QueryPipelineRuns(ctx, db, "sqlite", "pipeline_runs", "", "", 10)
	if err != nil {
		t.Fatalf("QueryPipelineRuns failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	run := runs[0]
	if run.SPID != 4242 {
		t.Errorf("expected run SPID 4242, got %d", run.SPID)
	}
	if run.DBUserName != "analytics_svc" || run.DatabaseUserName != "analytics_svc" {
		t.Errorf("expected run DBUserName 'analytics_svc', got DBUserName=%q DatabaseUserName=%q", run.DBUserName, run.DatabaseUserName)
	}
	if run.OSUserName != "srv_runner" || run.UserName != "srv_runner" {
		t.Errorf("expected run OSUserName 'srv_runner', got OSUserName=%q UserName=%q", run.OSUserName, run.UserName)
	}
}

func TestPipelineEventModal_ContainsSPIDAndDBUser(t *testing.T) {
	indexBytes, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatalf("failed to read tmpl/index.html: %v", err)
	}
	tmplStr := string(indexBytes)

	requiredSnippets := []string{
		`id="event-detail-db-user"`,
		`id="event-detail-spid"`,
		`evt.os_user_name || evt.user_name`,
		`evt.db_user_name || evt.database_user_name`,
		`(evt.spid !== undefined && evt.spid !== null && evt.spid !== 0) ? evt.spid : '-'`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(tmplStr, snippet) {
			t.Errorf("expected tmpl/index.html to contain %q", snippet)
		}
		if !strings.Contains(IndexHTML, snippet) {
			t.Errorf("expected IndexHTML in html_content.go to contain %q", snippet)
		}
	}
}

func TestExecutionLogs_ContainsSPID(t *testing.T) {
	indexBytes, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatalf("failed to read tmpl/index.html: %v", err)
	}
	tmplStr := string(indexBytes)

	requiredSnippets := []string{
		`<th class="p-2.5">SPID</th>`,
		`<th class="p-2.5 text-center">SPID</th>`,
		`id="active-run-spid-display"`,
		`id="active-run-db-user-display"`,
		`run.spid ? '<span class="text-amber-400 font-semibold">' + escapeHtml(run.spid) + '</span>'`,
		`active-run-spid-display`,
		`(run.spid !== undefined && run.spid !== null && run.spid !== 0) ? run.spid : '-'`,
		`evt.spid ? escapeHtml(evt.spid) : '<span class="text-slate-600">-</span>'`,
		`+ (r.spid || '')`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(tmplStr, snippet) {
			t.Errorf("expected tmpl/index.html to contain %q", snippet)
		}
		if !strings.Contains(IndexHTML, snippet) {
			t.Errorf("expected IndexHTML in html_content.go to contain %q", snippet)
		}
	}
}




