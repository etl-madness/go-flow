package builder

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionStreamValidatesInputs(t *testing.T) {
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
	script, err := storage.CreateScript("stream-test", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"pipeline.xml", "config.xml", "options.xml"} {
		if err := os.WriteFile(file, []byte("<pipeline><flow/></pipeline>"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	outside := url.QueryEscape(filepath.Join(filepath.VolumeName(dir)+string(os.PathSeparator), "flow-test-outside.xml"))
	for _, tc := range []struct {
		name, query, want string
	}{
		{"missing-script", "?file=missing.xml", "Script file not found"},
		{"outside-script", "?file=" + outside, "Access denied for script file"},
		{"missing-config", "?file=pipeline.xml&config=missing.xml", "Config file not found"},
		{"missing-options", "?file=pipeline.xml&options=missing.xml", "Options file not found"},
		{"outside-config", "?file=pipeline.xml&config=" + outside, "Access denied for config"},
		{"outside-options", "?file=pipeline.xml&options=" + outside, "Access denied for options"},
		{"builder-outside", "?source=builder&file=" + outside, "access denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			server.handleExecuteStream(rec, httptest.NewRequest(http.MethodGet, "/api/execute/stream"+tc.query, nil))
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("stream response = %d %q, want %q", rec.Code, rec.Body, tc.want)
			}
		})
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/execute/stream", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	server.handleExecuteStream(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site request = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/execute/stream", nil)
	req.Header.Set("Origin", "https://other.invalid")
	server.handleExecuteStream(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin request = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	target := fmt.Sprintf("/api/execute/stream?source=builder&script_id=%d&file=draft.xml&format=json", script.ID)
	server.handleExecuteStream(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if !strings.Contains(rec.Body.String(), "Exported builder draft") || !strings.Contains(rec.Body.String(), `"status":"FAILED"`) {
		t.Fatalf("expected draft export and child failure in %s", rec.Body)
	}
	data, err := os.ReadFile("draft.xml")
	if err != nil || !strings.Contains(string(data), "<pipeline") {
		t.Fatalf("draft = %q, %v", data, err)
	}
}
