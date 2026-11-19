package main

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestURIResolverLocalAndHTTP(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("document.xml", []byte{0xef, 0xbb, 0xbf, '<', 'x', '/', '>'}, 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.NotFound(w, r)
		case "/error":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		default:
			w.Write([]byte("<remote/>"))
		}
	}))
	defer server.Close()
	resolver := newDefaultURIResolver()
	for _, tc := range []struct {
		uri, want string
	}{
		{"document.xml", "<x/>"},
		{"file:///" + filepath.ToSlash(filepath.Join(dir, "document.xml")), "<x/>"},
		{server.URL + "/document", "<remote/>"},
		{"pipeline.xsd", "<"},
	} {
		reader, err := resolver.Resolve(tc.uri)
		if err != nil {
			t.Fatalf("resolve %q: %v", tc.uri, err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || !strings.Contains(string(data), tc.want) {
			t.Fatalf("resolve %q = %q, %v", tc.uri, data, err)
		}
	}
	for _, uri := range []string{"../outside.xml", server.URL + "/error", "http://169.254.169.254/metadata", "not-found.xml"} {
		if reader, err := resolver.ResolveURI(uri); err == nil {
			reader.Close()
			t.Errorf("expected resolver error for %q", uri)
		}
	}
	if _, err := resolver.ResolveURI(server.URL + "/missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("404 error = %v, want os.ErrNotExist", err)
	}
}

func TestLoadResourceFromSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resource.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE resources (name TEXT, content TEXT); INSERT INTO resources VALUES ('p1', '<pipeline/>'), ('empty', ''); CREATE TABLE flow_pipeline_content (ID INTEGER, Name TEXT, PipelineXML TEXT); INSERT INTO flow_pipeline_content VALUES (7, 'std_pipe', '<std/>')`); err != nil {
		t.Fatal(err)
	}
	base := "sql://sqlite3@" + filepath.ToSlash(path) + "#"
	data, err := loadFromDatabase(context.Background(), base+"table=resources&column=content")
	if err != nil || string(data) != "<pipeline/>" {
		t.Fatalf("resource = %q, %v", data, err)
	}
	byName, err := loadFromDatabase(context.Background(), base+"table=flow_pipeline_content&name=std_pipe")
	if err != nil || string(byName) != "<std/>" {
		t.Fatalf("byName = %q, %v", byName, err)
	}
	byID, err := loadFromDatabase(context.Background(), base+"table=flow_pipeline_content&id=7")
	if err != nil || string(byID) != "<std/>" {
		t.Fatalf("byID = %q, %v", byID, err)
	}
	for _, frag := range []string{
		"table=resources&column=content&key_col=name&key=empty",
		"table=nonexistent&column=content",
		"SELECT content FROM resources LIMIT 1",
		"table=resources;DROP TABLE resources&column=content",
	} {
		if _, err := loadFromDatabase(context.Background(), base+frag); err == nil {
			t.Errorf("expected error for %q", frag)
		}
	}
}
