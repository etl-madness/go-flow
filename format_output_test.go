package main

import (
	"os"
	"strings"
	"testing"

	"github.com/etl-madness/flow"
)

func TestFormattedOutput(t *testing.T) {
	results := []flow.ScriptResult{
		{ScriptID: "one", ReturnCode: 0, ResultsString: `{"count":2}`},
		{ScriptID: "two", ReturnCode: 1, ResultsString: "a|b\nc"},
	}
	for _, tc := range []struct {
		name string
		fn   func(*[]flow.ScriptResult)
		want []string
	}{
		{"raw-json", outputRawJSON, []string{`"script_id":"one"`, `"script_id":"two"`}},
		{"json", outputJSON, []string{`"script_id": "one"`, `"count": 2`}},
		{"text", outputText, []string{"ScriptID: one", "ScriptID: two"}},
		{"markdown", outputMarkdownTable, []string{"| one |", `a\|b<br>c`}},
		{"csv", outputCSV, []string{"one,", "two,"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := os.CreateTemp(t.TempDir(), "output")
			if err != nil {
				t.Fatal(err)
			}
			old := os.Stdout
			os.Stdout = output
			t.Cleanup(func() { os.Stdout = old; output.Close() })
			tc.fn(&results)
			if err := output.Close(); err != nil {
				t.Fatal(err)
			}
			os.Stdout = old
			data, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(string(data), want) {
					t.Errorf("missing %q in %q", want, data)
				}
			}
		})
	}
}

func TestDatabaseDialectDetectionAndInsertQueries(t *testing.T) {
	for _, tc := range []struct{ dsn, want string }{
		{"server=localhost;fedauth=ActiveDirectory", "azuresql"},
		{"postgres://localhost/db", "postgres"},
		{"server=localhost", "sqlserver"},
		{"user@tcp(localhost)/db", "mysql"},
		{"test.sqlite", "sqlite3"},
		{"oracle://localhost", "oracle"},
		{"unknown", "postgres"},
	} {
		if got := detectDriverFromDSN(tc.dsn); got != tc.want {
			t.Errorf("detectDriverFromDSN(%q) = %q, want %q", tc.dsn, got, tc.want)
		}
	}
	for _, tc := range []struct{ driver, want string }{
		{"postgres", "INSERT INTO events (a, b) VALUES ($1, $2)"},
		{"sqlserver", "INSERT INTO dbo.events (a, b) VALUES (@p1, @p2)"},
		{"oracle", "INSERT INTO events (a, b) VALUES (:1, :2)"},
		{"sqlite", "INSERT INTO events (a, b) VALUES (?, ?)"},
	} {
		if got := buildInsertQuery(tc.driver, "events", []string{"a", "b"}); got != tc.want {
			t.Errorf("%s query = %q, want %q", tc.driver, got, tc.want)
		}
	}
}
