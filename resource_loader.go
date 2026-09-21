package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"
	_ "modernc.org/sqlite"
)

func resourceSourceLabel(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}

	if strings.HasPrefix(source, "sql://") || strings.HasPrefix(source, "db://") ||
		strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return source
	}

	if absPath, err := filepath.Abs(source); err == nil {
		return absPath
	}
	return source
}

// LoadResource resolves paths from filesystem, SQL databases, or HTTP endpoints.
// Format for SQL URIs: sql://<driver>@<dsn>#<SQL_QUERY>
func LoadResource(ctx context.Context, source string) ([]byte, error) {
	switch {
	case strings.HasPrefix(source, "sql://") || strings.HasPrefix(source, "db://"):
		content, err := loadFromDatabase(ctx, source)
		if err != nil {
			return nil, err
		}
		return normalizeXMLBytes(content), nil

	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		content, err := loadFromHTTP(ctx, source)
		if err != nil {
			return nil, err
		}
		return normalizeXMLBytes(content), nil

	default:
		// Fallback to standard local filesystem
		content, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		return normalizeXMLBytes(content), nil
	}
}

func loadFromDatabase(ctx context.Context, uri string) ([]byte, error) {
	raw := strings.TrimPrefix(strings.TrimPrefix(uri, "sql://"), "db://")

	parts := strings.SplitN(raw, "#", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid DB URI syntax; missing SQL query fragment after '#': %s", uri)
	}

	connSpec := parts[0]
	query := parts[1]

	driverAndDSN := strings.SplitN(connSpec, "@", 2)
	if len(driverAndDSN) < 2 {
		return nil, fmt.Errorf("invalid DB connection spec; expected '<driver>@<dsn>': %s", connSpec)
	}

	driver := strings.ToLower(strings.TrimSpace(driverAndDSN[0]))
	dsn := strings.TrimSpace(driverAndDSN[1])

	// Standardize driver aliases
	switch driver {
	case "mssql":
		driver = "sqlserver"
	case "postgresql":
		driver = "postgres"
	case "sqlite3":
		driver = "sqlite"
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening database driver %q: %w", driver, err)
	}
	defer db.Close()

	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var content []byte
	err = db.QueryRowContext(execCtx, query).Scan(&content)
	if err != nil {
		return nil, fmt.Errorf("failed scanning XML content from database (%s): %w", driver, err)
	}

	if len(content) == 0 {
		return nil, fmt.Errorf("database query returned empty result set from %s", uri)
	}

	return content, nil
}

func loadFromHTTP(ctx context.Context, rawURL string) ([]byte, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	requestURL := parsedURL.String()
	requestAuthHeader := ""

	if parsedURL.User != nil {
		username := parsedURL.User.Username()
		if strings.HasPrefix(strings.ToLower(username), "basic ") {
			requestAuthHeader = username
			parsedURL.User = nil
			requestURL = parsedURL.String()
		} else if password, hasPassword := parsedURL.User.Password(); hasPassword {
			parsedURL.User = nil
			requestURL = parsedURL.String()
			// We delay setting the Basic auth header until after the request is created,
			// to avoid net/http re-encoding the URL userinfo and creating a duplicate header.
			requestAuthHeader = "__basic_user_pass__" + username + "\x00" + password
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	if requestAuthHeader != "" {
		if strings.HasPrefix(requestAuthHeader, "__basic_user_pass__") {
			parts := strings.SplitN(strings.TrimPrefix(requestAuthHeader, "__basic_user_pass__"), "\x00", 2)
			if len(parts) == 2 {
				req.SetBasicAuth(parts[0], parts[1])
			}
		} else {
			req.Header.Set("Authorization", requestAuthHeader)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP GET %s failed with status code %d", rawURL, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
