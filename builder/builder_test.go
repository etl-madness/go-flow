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
	tmpDir := t.TempDir()
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
		t.Errorf("expected subfolder in browse results")
	}
	if !hasScriptsXML {
		t.Errorf("expected scripts.xml in browse results")
	}
	if hasReadme {
		t.Errorf("expected readme.txt to be filtered out by .xml ext filter")
	}

	// Test 2: Quick files list
	qReq := httptest.NewRequest(http.MethodGet, "/api/files/quick?root="+tmpDir+"&ext=.xml", nil)
	qRec := httptest.NewRecorder()
	server.handleQuickFiles(qRec, qReq)

	if qRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for quick files, got %d", qRec.Code)
	}

	var quickFiles []string
	if err := json.Unmarshal(qRec.Body.Bytes(), &quickFiles); err != nil {
		t.Fatalf("failed to unmarshal quick files: %v", err)
	}
	if len(quickFiles) < 2 {
		t.Fatalf("expected at least two xml files in tmpDir, got %d", len(quickFiles))
	}
}

func TestFileBrowsingIncludesHiddenDirectories(t *testing.T) {
	tmpDir := t.TempDir()
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
	tmpXML := filepath.Join(t.TempDir(), "api_import.xml")
	_ = os.WriteFile(tmpXML, []byte("<pipeline name=\"api_import_pipe\"><flow><sql id=\"1\">SELECT 1</sql></flow></pipeline>"), 0644)
	importReqBody, _ := json.Marshal(map[string]any{"file_path": tmpXML, "name": "Imported via API"})
	reqImport := httptest.NewRequest(http.MethodPost, "/api/scripts/import", bytes.NewReader(importReqBody))
	recImport := httptest.NewRecorder()
	server.handleImportScript(recImport, reqImport)
	if recImport.Code != http.StatusOK {
		t.Fatalf("handleImportScript failed with code %d: %s", recImport.Code, recImport.Body.String())
	}

	// 3. Delete API
	reqDelete := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/scripts/delete?id=%d", copyResp.Script.ID), nil)
	recDelete := httptest.NewRecorder()
	server.handleDeleteScript(recDelete, reqDelete)
	if recDelete.Code != http.StatusOK {
		t.Fatalf("handleDeleteScript failed with code %d: %s", recDelete.Code, recDelete.Body.String())
	}

	// 4. Purge API
	reqPurge := httptest.NewRequest(http.MethodPost, "/api/db/purge", nil)
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
