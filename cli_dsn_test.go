package main

import "testing"

func TestImportExportDriver(t *testing.T) {
	for _, tc := range []struct {
		dsn, driver, clean string
	}{
		{"plain", "sqlserver", "plain"},
		{"sql:// SQLITE @test.db", "sqlite", "test.db"},
		{"db://postgres@host", "postgres", "host"},
		{"sql://missing-separator", "sqlserver", "sql://missing-separator"},
		{"postgres://localhost/db", "postgres", "postgres://localhost/db"},
		{"postgresql://localhost/db", "postgres", "postgresql://localhost/db"},
		{"mysql://localhost/db", "mysql", "mysql://localhost/db"},
	} {
		t.Run(tc.dsn, func(t *testing.T) {
			driver, clean := importExportDriver(tc.dsn)
			if driver != tc.driver || clean != tc.clean {
				t.Fatalf("got (%q, %q), want (%q, %q)", driver, clean, tc.driver, tc.clean)
			}
		})
	}
}
