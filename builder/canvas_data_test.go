package builder

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCanvasData(t *testing.T) {
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
	script, err := storage.CreateScript("canvas test", "")
	if err != nil {
		t.Fatal(err)
	}

	for _, section := range []struct{ name, kind string }{
		{"variables", "variable"},
		{"databases", "database"},
		{"preflight", "script"},
		{"flow", "script"},
	} {
		if _, err := storage.AddNode(script.ID, section.name, section.kind, map[string]string{"id": section.name}, ""); err != nil {
			t.Fatalf("add %s node: %v", section.name, err)
		}
	}

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?script_id=%d", script.ID), nil)
	data, err := server.loadCanvasData(request)
	if err != nil {
		t.Fatal(err)
	}
	if data.ActiveScript.ID != script.ID {
		t.Fatalf("active script ID = %d, want %d", data.ActiveScript.ID, script.ID)
	}
	for _, section := range []struct {
		name  string
		nodes []PipelineNode
	}{
		{"variables", data.VariableNodes},
		{"databases", data.DatabaseNodes},
		{"preflight", data.PreflightNodes},
		{"flow", data.FlowNodes},
	} {
		if len(section.nodes) != 1 || section.nodes[0].Attributes["id"] != section.name {
			t.Errorf("%s nodes = %+v", section.name, section.nodes)
		}
	}

	if _, err := storage.db.Exec("DROP TABLE pipeline_nodes"); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []struct {
		name string
		fn   http.HandlerFunc
	}{
		{"index", server.handleIndex},
		{"canvas", server.handleCanvas},
	} {
		t.Run(handler.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.fn(response, request)
			if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "failed to get variable nodes") {
				t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
			}
		})
	}
}
