package main

import (
	"path/filepath"
	"testing"

	"github.com/etl-madness/go-flow/builder"
)

func TestCLIPurgesBuilderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "builder.db")
	storage, err := builder.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateScript("to-purge", "temporary pipeline"); err != nil {
		storage.Close()
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
	runFlowCLI(t, "-builder-db", path, "-purge-db")
	storage, err = builder.NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	scripts, err := storage.ListScripts()
	if err != nil || len(scripts) != 1 || scripts[0].Name == "to-purge" {
		t.Fatalf("scripts after purge = %+v, %v", scripts, err)
	}
}
