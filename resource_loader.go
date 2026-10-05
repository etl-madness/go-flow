package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
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

	"github.com/etl-madness/go-flow/pkg/flowcrypto"
)

func resourceSourceLabel(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}

	if strings.HasPrefix(source, "sql://") || strings.HasPrefix(source, "db://") ||
		strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return maskSensitiveSourceForDB(source, false)
	}

	if absPath, err := filepath.Abs(source); err == nil {
		return absPath
	}
	return source
}

// SecurityOptions configures decryption and digital signature verification for resource loading.
type SecurityOptions struct {
	Encrypted       bool
	SecureKey       string
	VerifySignature bool
	PublicKeyPath   string
	CertPath        string
	CACertPath      string
	KeyringPath     string
	SignaturePath   string
}

// LoadResource resolves paths from filesystem, SQL databases, or HTTP endpoints.
// Format for SQL URIs: sql://<driver>@<dsn>#<SQL_QUERY>
func LoadResource(ctx context.Context, source string) ([]byte, error) {
	return LoadResourceSecure(ctx, source, false, "")
}

// LoadResourceSecure resolves paths from filesystem, SQL databases, or HTTP endpoints,
// and decrypts the content if encrypted is true or if the payload contains an encrypted envelope.
func LoadResourceSecure(ctx context.Context, source string, encrypted bool, secureKey string) ([]byte, error) {
	data, _, err := LoadResourceVerified(ctx, source, SecurityOptions{
		Encrypted: encrypted,
		SecureKey: secureKey,
	})
	return data, err
}

// LoadResourceVerified resolves paths, verifies digital signatures (OpenSSL, OpenPGP, PKCS#7 / Windows),
// and decrypts content (AES-256-GCM) before returning normalized bytes.
func LoadResourceVerified(ctx context.Context, source string, opts SecurityOptions) ([]byte, *flowcrypto.VerificationResult, error) {
	var content []byte
	var err error

	switch {
	case strings.HasPrefix(source, "sql://") || strings.HasPrefix(source, "db://"):
		content, err = loadFromDatabase(ctx, source)
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		content, err = loadFromHTTP(ctx, source)
	default:
		// Fallback to standard local filesystem
		content, err = os.ReadFile(source)
	}

	if err != nil {
		return nil, nil, err
	}

	// 1. Check if verification key or certificate is provided
	var keyOrCertBytes []byte
	if opts.PublicKeyPath != "" {
		keyOrCertBytes, err = os.ReadFile(opts.PublicKeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed reading public key %s: %w", opts.PublicKeyPath, err)
		}
	} else if opts.CertPath != "" {
		keyOrCertBytes, err = os.ReadFile(opts.CertPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed reading certificate %s: %w", opts.CertPath, err)
		}
	} else if opts.KeyringPath != "" {
		keyOrCertBytes, err = os.ReadFile(opts.KeyringPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed reading keyring %s: %w", opts.KeyringPath, err)
		}
	}

	var caCertBytes []byte
	if opts.CACertPath != "" {
		caCertBytes, err = os.ReadFile(opts.CACertPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed reading CA certificate %s: %w", opts.CACertPath, err)
		}
	}

	// 1. Detect if an outer digital signature is present (unified envelope or detached companion)
	isOuterSignedEnvelope := flowcrypto.IsSignedPayload(content)
	effectiveSig := opts.SignaturePath
	shouldCheckCompanion := opts.SignaturePath != "" || opts.VerifySignature || len(keyOrCertBytes) > 0 || len(caCertBytes) > 0
	if effectiveSig == "" && shouldCheckCompanion && !strings.Contains(source, "://") {
		for _, ext := range []string{".sig", ".asc", ".p7s"} {
			candidate := source + ext
			if _, err := os.Stat(candidate); err == nil {
				effectiveSig = candidate
				break
			}
		}
	}
	hasOuterSignature := isOuterSignedEnvelope || effectiveSig != ""

	var verResult *flowcrypto.VerificationResult

	if hasOuterSignature {
		if len(keyOrCertBytes) == 0 && len(caCertBytes) == 0 {
			return nil, nil, fmt.Errorf("resource %q has a digital signature, but no public key (-public-key) or certificate (-cert) was provided for verification", resourceSourceLabel(source))
		}

		if isOuterSignedEnvelope {
			// Unified signed envelope
			unwrapped, res, err := flowcrypto.VerifyPayloadAutoWithCA(content, keyOrCertBytes, caCertBytes)
			if err != nil {
				return nil, nil, fmt.Errorf("digital signature verification failed for resource %q: %w", resourceSourceLabel(source), err)
			}
			content = unwrapped
			verResult = res
		} else {
			// Detached signature
			sigBytes, err := os.ReadFile(effectiveSig)
			if err != nil {
				return nil, nil, fmt.Errorf("failed reading signature file %s: %w", effectiveSig, err)
			}

			res, err := flowcrypto.VerifyAutoWithCA(content, sigBytes, keyOrCertBytes, caCertBytes)
			if err != nil {
				return nil, nil, fmt.Errorf("digital signature verification failed for resource %q: %w", resourceSourceLabel(source), err)
			}
			verResult = res
		}
	}

	// 2. Decrypt if explicitly requested or if auto-detected as encrypted
	if opts.Encrypted || flowcrypto.IsEncryptedPayload(content) {
		key := strings.TrimSpace(opts.SecureKey)
		if key == "" {
			key = strings.TrimSpace(os.Getenv("FLOW_SECURE_KEY"))
		}
		if key == "" {
			key = strings.TrimSpace(os.Getenv("SECURE_KEY"))
		}

		if key == "" {
			return nil, nil, fmt.Errorf("resource %q is encrypted but no secure key was provided (specify -secure-key or set FLOW_SECURE_KEY)", resourceSourceLabel(source))
		}

		decrypted, err := flowcrypto.DecryptAuto(content, key, opts.Encrypted)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decrypt resource %q: %w", resourceSourceLabel(source), err)
		}
		content = decrypted

		// In case of Sign-then-Encrypt, check if decrypted payload was itself signed
		if flowcrypto.IsSignedPayload(content) {
			if len(keyOrCertBytes) == 0 && len(caCertBytes) == 0 {
				return nil, nil, fmt.Errorf("decrypted payload in %q is digitally signed, but no public key (-public-key) or certificate (-cert) was provided", resourceSourceLabel(source))
			}
			unwrapped, res, err := flowcrypto.VerifyPayloadAutoWithCA(content, keyOrCertBytes, caCertBytes)
			if err != nil {
				return nil, nil, fmt.Errorf("digital signature verification failed for decrypted payload in %q: %w", resourceSourceLabel(source), err)
			}
			content = unwrapped
			verResult = res
		}
	}

	// 3. Enforce signature verification if -verify-signature was explicitly requested
	if opts.VerifySignature && (verResult == nil || !verResult.Valid) {
		return nil, nil, fmt.Errorf("digital signature verification enforced (-verify-signature), but resource %q is not signed", resourceSourceLabel(source))
	}

	return normalizeXMLBytes(content), verResult, nil
}

const maxHTTPResourceBytes = 50 * 1024 * 1024 // 50MB maximum allowable resource size

func loadFromDatabase(ctx context.Context, uri string) ([]byte, error) {
	raw := strings.TrimPrefix(strings.TrimPrefix(uri, "sql://"), "db://")

	parts := strings.SplitN(raw, "#", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid DB URI syntax; missing SQL query fragment after '#': %s", maskSensitiveSourceForDB(uri, false))
	}

	connSpec := parts[0]
	query := parts[1]

	driverAndDSN := strings.SplitN(connSpec, "@", 2)
	if len(driverAndDSN) < 2 {
		return nil, fmt.Errorf("invalid DB connection spec; expected '<driver>@<dsn>': %s", maskSensitiveSourceForDB("sql://"+connSpec, false))
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
		return nil, fmt.Errorf("database query returned empty result set from %s", maskSensitiveSourceForDB(uri, false))
	}

	return content, nil
}

func isCloudMetadataOrLinkLocal(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "169.254.169.254" || host == "metadata.google.internal" || host == "instance-data" {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
		if ipv4 := ip.To4(); ipv4 != nil && ipv4[0] == 169 && ipv4[1] == 254 {
			return true
		}
	}
	return false
}

func loadFromHTTP(ctx context.Context, rawURL string) ([]byte, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if isCloudMetadataOrLinkLocal(parsedURL.Hostname()) {
		return nil, fmt.Errorf("HTTP GET %s blocked: access to cloud metadata and link-local addresses is prohibited", maskSensitiveSourceForDB(rawURL, false))
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
		return nil, fmt.Errorf("HTTP GET %s failed with status code %d", maskSensitiveSourceForDB(rawURL, false), resp.StatusCode)
	}

	lr := io.LimitReader(resp.Body, maxHTTPResourceBytes+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxHTTPResourceBytes {
		return nil, fmt.Errorf("HTTP response from %s exceeded maximum allowed size (%d bytes)", maskSensitiveSourceForDB(rawURL, false), maxHTTPResourceBytes)
	}

	return body, nil
}
