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

func TestSettingsAndFileHandlers(t *testing.T) {
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
	call := func(handler http.HandlerFunc, method, url, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s: %d %s, want %d", method, url, rec.Code, rec.Body, status)
		}
		return rec
	}
	dsn := filepath.ToSlash(filepath.Join(dir, "logs.db"))
	payload, err := json.Marshal(SavedDatabaseConnection{Name: "logs", Driver: "sqlite", ConnectionString: dsn})
	if err != nil {
		t.Fatal(err)
	}
	rec := call(server.handleSaveDatabase, "POST", "/", string(payload), 200)
	var saved struct {
		Connection SavedDatabaseConnection `json:"connection"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil || saved.Connection.ID == 0 {
		t.Fatalf("saved connection: %s (%v)", rec.Body, err)
	}
	id := saved.Connection.ID
	rec = call(server.handleListDatabases, "GET", "/", "", 200)
	if !strings.Contains(rec.Body.String(), "logs") {
		t.Fatalf("missing connection: %s", rec.Body)
	}
	rec = call(server.handleTestDatabase, "POST", "/", fmt.Sprintf(`{"id":%d}`, id), 200)
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("connection test failed: %s", rec.Body)
	}
	rec = call(server.handleTestDatabase, "POST", "/", `{"driver":"invalid","connection_string":"x"}`, 200)
	if !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Fatalf("invalid driver accepted: %s", rec.Body)
	}
	for _, handler := range []http.HandlerFunc{server.handleListLogRuns, server.handleListLogEvents} {
		target := fmt.Sprintf("/?db_id=%d&run_id=missing", id)
		rec = call(handler, "GET", target, "", 200)
		if !strings.Contains(rec.Body.String(), `"table_exists":false`) {
			t.Fatalf("expected missing table: %s", rec.Body)
		}
		call(handler, "GET", "/", "", 400)
	}
	call(server.handleDeleteDatabase, "POST", "/", fmt.Sprintf(`{"id":%d}`, id), 200)
	call(server.handleTestDatabase, "POST", "/", fmt.Sprintf(`{"id":%d}`, id), 404)
	call(server.handleSaveDatabase, "POST", "/", "{", 400)
	call(server.handleDeleteDatabase, "POST", "/", `{}`, 400)
	if err := os.Mkdir("nested", 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"one.xml", "ignore.txt", filepath.Join("nested", "two.xml")} {
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	rec = call(server.handleBrowseFiles, "GET", "/?ext=.xml", "", 200)
	var browse BrowseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &browse); err != nil {
		t.Fatal(err)
	}
	if len(browse.Entries) == 0 || !browse.Entries[0].IsDir || strings.Contains(rec.Body.String(), "ignore.txt") {
		t.Fatalf("unexpected file list: %s", rec.Body)
	}
	rec = call(server.handleQuickFiles, "GET", "/", "", 200)
	if !strings.Contains(rec.Body.String(), "one.xml") || !strings.Contains(rec.Body.String(), "two.xml") || strings.Contains(rec.Body.String(), "ignore.txt") {
		t.Fatalf("unexpected quick files: %s", rec.Body)
	}
	call(server.handleBrowseFiles, "GET", "/?dir=..", "", 400)
}
