package builder

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestServer(t *testing.T) (*Server, *Storage) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_auth_builder.db")

	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	server, err := NewServer(storage, 0)
	if err != nil {
		storage.Close()
		t.Fatalf("failed to create test server: %v", err)
	}

	return server, storage
}

func TestBuilderAuth_RejectUnauthorized(t *testing.T) {
	server, storage := setupTestServer(t)
	defer storage.Close()

	handler := server.Handler()

	// 1. Root route without auth
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for unauthenticated root request, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Access Restricted") {
		t.Fatalf("expected 401 HTML page to contain 'Access Restricted', got: %s", rec.Body.String())
	}

	// 2. API route without auth
	apiReq := httptest.NewRequest(http.MethodGet, "/api/canvas", nil)
	apiRec := httptest.NewRecorder()
	handler.ServeHTTP(apiRec, apiReq)

	if apiRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for unauthenticated API request, got %d", apiRec.Code)
	}
	var errResp map[string]string
	if err := json.Unmarshal(apiRec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode JSON 401 error response: %v", err)
	}
	if !strings.Contains(errResp["error"], "Unauthorized") {
		t.Fatalf("unexpected error message in 401 JSON: %s", errResp["error"])
	}
}

func TestBuilderAuth_QueryTokenAndSessionCookie(t *testing.T) {
	server, storage := setupTestServer(t)
	defer storage.Close()

	handler := server.Handler()
	token := server.GetAuthToken()
	if token == "" {
		t.Fatal("expected non-empty auth token generated for server")
	}

	// 1. Visit root with ?token=...
	req := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should redirect to clean "/" and set session cookie
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303 upon initial token auth, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie to be set", sessionCookieName)
	}
	if !sessionCookie.HttpOnly {
		t.Errorf("expected session cookie to be HttpOnly")
	}
	if sessionCookie.Secure {
		t.Errorf("expected session cookie Secure attribute to be false on plain HTTP")
	}

	// 1b. Visit root with ?token=... and X-Forwarded-Proto: https
	httpsReq := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
	httpsReq.Header.Set("X-Forwarded-Proto", "https")
	httpsRec := httptest.NewRecorder()
	handler.ServeHTTP(httpsRec, httpsReq)

	var httpsCookie *http.Cookie
	for _, c := range httpsRec.Result().Cookies() {
		if c.Name == sessionCookieName {
			httpsCookie = c
			break
		}
	}
	if httpsCookie == nil {
		t.Fatalf("expected %q cookie to be set for HTTPS request", sessionCookieName)
	}
	if !httpsCookie.Secure {
		t.Errorf("expected session cookie Secure attribute to be true when X-Forwarded-Proto is https")
	}

	// 2. Follow redirect to "/" using the session cookie
	followReq := httptest.NewRequest(http.MethodGet, "/", nil)
	followReq.AddCookie(sessionCookie)
	followRec := httptest.NewRecorder()
	handler.ServeHTTP(followRec, followReq)

	if followRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 using session cookie, got %d", followRec.Code)
	}

	// 3. Access API endpoint using session cookie
	canvasReq := httptest.NewRequest(http.MethodGet, "/api/canvas", nil)
	canvasReq.AddCookie(sessionCookie)
	canvasRec := httptest.NewRecorder()
	handler.ServeHTTP(canvasRec, canvasReq)

	if canvasRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for API call using session cookie, got %d", canvasRec.Code)
	}
}

func TestBuilderAuth_BearerToken(t *testing.T) {
	server, storage := setupTestServer(t)
	defer storage.Close()

	handler := server.Handler()
	token := server.GetAuthToken()

	req := httptest.NewRequest(http.MethodGet, "/api/canvas", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for Bearer token auth, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBuilderAuth_InvalidTokenAndFakeCookie(t *testing.T) {
	server, storage := setupTestServer(t)
	defer storage.Close()

	handler := server.Handler()

	// 1. Invalid query token
	badTokenReq := httptest.NewRequest(http.MethodGet, "/?token=invalid_token_123", nil)
	badTokenRec := httptest.NewRecorder()
	handler.ServeHTTP(badTokenRec, badTokenReq)

	if badTokenRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for bad token, got %d", badTokenRec.Code)
	}

	// 2. Fake session cookie
	fakeCookieReq := httptest.NewRequest(http.MethodGet, "/", nil)
	fakeCookieReq.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "fake_session_id_456",
	})
	fakeCookieRec := httptest.NewRecorder()
	handler.ServeHTTP(fakeCookieRec, fakeCookieReq)

	if fakeCookieRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for fake session cookie, got %d", fakeCookieRec.Code)
	}
}

func TestBuilderAuth_PortOverrideAndDynamicPort(t *testing.T) {
	server, storage := setupTestServer(t)
	defer storage.Close()

	// Bind to an ephemeral port (port 0) on loopback
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on loopback port 0: %v", err)
	}
	defer listener.Close()

	actualPort := listener.Addr().(*net.TCPAddr).Port
	if actualPort <= 0 {
		t.Fatalf("expected positive port number, got %d", actualPort)
	}

	// Start server on the listener in background
	go func() {
		_ = server.Serve(listener)
	}()
	time.Sleep(50 * time.Millisecond)

	// Verify server port was updated
	if server.Port() != actualPort {
		t.Fatalf("expected server.Port() to be %d, got %d", actualPort, server.Port())
	}
}
