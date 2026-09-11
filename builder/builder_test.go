package builder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	reqCopy.Header.Set("X-CSRF-Token", server.csrfToken)
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
	reqImport.Header.Set("X-CSRF-Token", server.csrfToken)
	recImport := httptest.NewRecorder()
	server.handleImportScript(recImport, reqImport)
	if recImport.Code != http.StatusOK {
		t.Fatalf("handleImportScript failed with code %d: %s", recImport.Code, recImport.Body.String())
	}

	// 3. Delete API
	reqDelete := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/scripts/delete?id=%d", copyResp.Script.ID), nil)
	reqDelete.Header.Set("X-CSRF-Token", server.csrfToken)
	recDelete := httptest.NewRecorder()
	server.handleDeleteScript(recDelete, reqDelete)
	if recDelete.Code != http.StatusOK {
		t.Fatalf("handleDeleteScript failed with code %d: %s", recDelete.Code, recDelete.Body.String())
	}

	// 4. Purge API
	reqPurge := httptest.NewRequest(http.MethodPost, "/api/db/purge", nil)
	reqPurge.Header.Set("X-CSRF-Token", server.csrfToken)
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
		NodeType:   "sql",
		Attributes: map[string]string{"id": "StepParallel1"},
		ContentText: "SELECT 1;",
	}
	sql2 := PipelineNode{
		NodeType:   "sql",
		Attributes: map[string]string{"id": "StepParallel2"},
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
		NodeType:   "script",
		Attributes: map[string]string{"id": "fallback_script", "language": "powershell"},
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
		NodeType:   "sql",
		Attributes: map[string]string{"id": "loop_step"},
		ContentText: "SELECT loop_batch();",
	}
	whileNode := PipelineNode{
		NodeType:   "while",
		Attributes: map[string]string{"condition": "has_more == 'yes'"},
		Children:   []PipelineNode{whileSql},
	}
	foreachSql := PipelineNode{
		NodeType:   "sql",
		Attributes: map[string]string{"id": "fe_step"},
		ContentText: "INSERT INTO sink VALUES ({{.row.id}});",
	}
	foreachNode := PipelineNode{
		NodeType:    "foreach",
		Attributes: map[string]string{"var": "row", "stream": "true"},
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
	reqIf.Header.Set("X-CSRF-Token", server.csrfToken)
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
	reqSql.Header.Set("X-CSRF-Token", server.csrfToken)
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
	reqValidCSRF.Header.Set("X-CSRF-Token", server.csrfToken)
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
	reqJSONBody.Header.Set("X-CSRF-Token", server.csrfToken)
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
	reqDelLast.Header.Set("X-CSRF-Token", server.csrfToken)
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
	if !strings.Contains(indexHTML, fmt.Sprintf(`const csrfToken = "%s";`, server.csrfToken)) {
		t.Errorf("index HTML does not contain expected csrfToken declaration with value %s", server.csrfToken)
	}
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



