package builder

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsHandlersReportStorageFailure(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	storage, err := NewStorage(filepath.Join(dir, "builder.db"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(storage, 0)
	if err != nil {
		storage.Close()
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []struct {
		name string
		fn   http.HandlerFunc
	}{
		{"config", server.handleSaveConfig},
		{"options", server.handleSaveOptions},
	} {
		t.Run(handler.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"settings.xml","content":"<settings/>"}`))
			request.Header.Set("X-Requested-With", "XMLHttpRequest")
			response := httptest.NewRecorder()
			handler.fn(response, request)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
			}
		})
	}
}
