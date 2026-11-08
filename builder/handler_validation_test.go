package builder

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorHandlerValidationAndExport(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	storage, err := NewStorage(filepath.Join(dir, "builder.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.Close() })
	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatal(err)
	}
	script, err := storage.CreateScript("export-test", "")
	if err != nil {
		t.Fatal(err)
	}
	call := func(handler http.HandlerFunc, method, target, body string, csrf bool, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		if csrf {
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s => %d %s; want %d", method, target, rec.Code, rec.Body, status)
		}
		return rec
	}
	for _, handler := range []http.HandlerFunc{
		server.handleAddNode, server.handleUpdateNode, server.handleDeleteNode, server.handleMoveNode,
		server.handleReorderNodes, server.handleNewScript, server.handleCopyScript,
		server.handleImportScript, server.handlePurgeDatabase, server.handleSaveFile,
		server.handleSaveConfig, server.handleSaveOptions, server.handleSaveDatabase,
		server.handleDeleteDatabase, server.handleTestDatabase,
	} {
		call(handler, "POST", "/", "{}", false, 403)
		call(handler, "GET", "/", "", false, 405)
	}
	call(server.handleAddNode, "POST", "/", "{", true, 400)
	call(server.handleAddNode, "POST", "/", fmt.Sprintf(`{"script_id":%d,"node_type":"if","attributes":{"id":"branch"}}`, script.ID), true, 200)
	nodes, err := storage.GetNodeTree(script.ID, "flow")
	if err != nil || len(nodes) != 1 || nodes[0].GetThenBranch() == nil || nodes[0].GetElseBranch() == nil {
		t.Fatalf("if node tree = %+v, %v", nodes, err)
	}
	call(server.handleUpdateNode, "POST", "/", "{", true, 400)
	call(server.handleMoveNode, "POST", "/?id=bad", "", true, 400)
	call(server.handleDeleteNode, "POST", "/?id=bad", "", true, 400)
	call(server.handleReorderNodes, "POST", "/", "{", true, 400)
	call(server.handleCopyScript, "POST", "/", "{", true, 400)
	call(server.handleImportScript, "POST", "/", `{}`, true, 400)
	call(server.handleImportScript, "POST", "/", `{"file_path":"file.txt"}`, true, 400)
	call(server.handleImportScript, "POST", "/", `{"file_path":"../file.xml"}`, true, 403)
	call(server.handleSaveFile, "POST", "/", fmt.Sprintf(`{"script_id":%d,"filename":"export.xml"}`, script.ID), true, 200)
	data, err := os.ReadFile(filepath.Join("exports", "export.xml"))
	if err != nil || !strings.Contains(string(data), `id="branch"`) {
		t.Fatalf("export = %q, %v", data, err)
	}
	call(server.handleSaveFile, "POST", "/", `{"script_id":999999}`, true, 404)
	call(server.handleGetNode, "GET", "/?id=999999", "", false, 404)
}
