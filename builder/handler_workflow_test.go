package builder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorHandlerWorkflows(t *testing.T) {
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
	request := func(handler http.HandlerFunc, method, target, body string, csrf bool, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		if csrf {
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s: got %d: %s; want %d", method, target, rec.Code, rec.Body, status)
		}
		return rec
	}
	rec := request(server.handleNewScript, "POST", "/", `{"name":"example","description":"test"}`, true, 200)
	var script Script
	if err := json.Unmarshal(rec.Body.Bytes(), &script); err != nil {
		t.Fatal(err)
	}
	n1, err := storage.AddNode(script.ID, "flow", "sql", map[string]string{"id": "one"}, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	n2, err := storage.AddNode(script.ID, "flow", "sql", map[string]string{"id": "two"}, "SELECT 2")
	if err != nil {
		t.Fatal(err)
	}
	request(server.handleGetNode, "GET", fmt.Sprintf("/?id=%d", n1.ID), "", false, 200)
	request(server.handleGetNode, "GET", "/?id=bad", "", false, 400)
	request(server.handleMoveNode, "POST", fmt.Sprintf("/?id=%d&dir=up", n2.ID), "", true, 200)
	nodes, err := storage.GetNodes(script.ID, "flow")
	if err != nil || len(nodes) != 2 || nodes[0].ID != n2.ID {
		t.Fatalf("move nodes = %+v, %v", nodes, err)
	}
	request(server.handleReorderNodes, "POST", "/", fmt.Sprintf(`{"script_id":%d,"section":"flow","node_ids":[%d,%d]}`, script.ID, n1.ID, n2.ID), true, 200)
	request(server.handleDeleteNode, "POST", fmt.Sprintf("/?id=%d", n1.ID), "", false, 403)
	request(server.handleDeleteNode, "POST", fmt.Sprintf("/?id=%d", n1.ID), "", true, 200)
	if _, err := storage.GetNode(n1.ID); err == nil {
		t.Fatal("deleted node still exists")
	}
	rec = request(server.handleCopyScript, "POST", "/", fmt.Sprintf(`{"id":%d,"new_name":"copy"}`, script.ID), true, 200)
	var copied struct {
		Script Script `json:"script"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &copied); err != nil || copied.Script.Name != "copy" {
		t.Fatalf("copy response = %s, error %v", rec.Body, err)
	}
	request(server.handleDeleteScript, "POST", "/", fmt.Sprintf(`{"id":%d}`, copied.Script.ID), true, 200)
	request(server.handleNewScript, "POST", "/", "{", true, 400)

	for _, item := range []struct {
		name    string
		handler http.HandlerFunc
		get     func(string) (string, error)
	}{
		{"config", server.handleSaveConfig, storage.GetConfigFile},
		{"options", server.handleSaveOptions, storage.GetOptionsFile},
	} {
		body := fmt.Sprintf(`{"name":"%s.xml","content":"<value/>"}`, item.name)
		request(item.handler, "POST", "/", body, true, 200)
		got, err := item.get(item.name + ".xml")
		if err != nil || got != "<value/>" {
			t.Fatalf("%s saved value = %q, %v", item.name, got, err)
		}
		file, err := os.ReadFile(filepath.Join(dir, "exports", item.name+".xml"))
		if err != nil || string(file) != got {
			t.Fatalf("%s exported file = %q, %v", item.name, file, err)
		}
		request(item.handler, "POST", "/", "{", true, 400)
	}
}
