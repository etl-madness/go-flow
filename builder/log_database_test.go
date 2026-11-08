package builder

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogHandlersDatabaseLookupErrors(t *testing.T) {
	storage, err := NewStorage(filepath.Join(t.TempDir(), "builder.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.Close() })
	server, err := NewServer(storage, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, handler := range []struct {
		name string
		fn   http.HandlerFunc
	}{
		{"runs", server.handleListLogRuns},
		{"events", server.handleListLogEvents},
	} {
		for _, tc := range []struct {
			query string
			code  int
			body  string
		}{
			{"run_id=run", http.StatusBadRequest, "Missing"},
			{"db_id=nope&run_id=run", http.StatusBadRequest, "Invalid db_id"},
			{"db_id=0&run_id=run", http.StatusBadRequest, "Invalid db_id"},
			{"db_id=999999&run_id=run", http.StatusNotFound, "Database connection not found"},
		} {
			t.Run(handler.name+"/"+tc.query, func(t *testing.T) {
				response := httptest.NewRecorder()
				handler.fn(response, httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil))
				if response.Code != tc.code || !strings.Contains(response.Body.String(), tc.body) {
					t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
				}
			})
		}
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{server.handleListLogRuns, server.handleListLogEvents} {
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequest(http.MethodGet, "/?db_id=1&run_id=run", nil))
		if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "Failed to fetch database connection") {
			t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
		}
	}
}
