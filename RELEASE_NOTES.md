# Release Notes

## Release Notes (v1.1.23) - Database Session Auditing, Combined Signatures & Encryption, CLI Reference & Builder UX Enhancements

### Overview

Version 1.1.23 delivers key observability, cryptographic, operational, and user experience enhancements across Flow:
1. **Database Session Auditing & Identity Tracking**: Pipeline events (`PipelineEventRecord`) and run summaries (`PipelineRunRecord`, `RunSummaryRecord`) now capture and persist the database process / session ID (`SPID` or engine equivalent) and authenticated `database_user_name` alongside the operating system user (`os_user_name`) across Microsoft SQL Server, PostgreSQL, MySQL, Oracle, and SQLite.
2. **Combined Encryption & Digital Signatures Architecture**: Comprehensive architecture and reference workflows for combining AES-256-GCM authenticated encryption with multi-standard digital signatures into single files via Sign-then-Encrypt, Encrypt-then-Sign, and unified envelopes, featuring Windows Digital Certificate (PowerShell / PKCS#7 / X.509) examples in `docs/COMBINED_ENCRYPTION_AND_SIGNATURES.md`.
3. **Comprehensive CLI Reference Guide**: Detailed documentation published in `docs/cli_arguments.md` covering all command-line arguments, environment variables, exit codes, and examples for `flow.exe`, `crypto_tool`, and `db_importer`.
4. **Builder Execution Output Format Selector**: Added an on-demand format selector (`text`, `json`, `csv`, `xml`, `mermaid`) to the Builder pipeline execution modal when running without a pre-loaded options file.
5. **Builder File Browser Error Recovery**: Hardened directory navigation in the Builder file browser to detect inaccessible/permission-denied folders and automatically revert to the last known good directory instead of failing into an empty or broken state.
6. **Builder Database Connection Test Highlights**: Updated the database settings tab in the Builder UI so that database driver badges turn emerald green on successful connection test and persist status across page reloads via `localStorage`.
7. **Builder Telemetry & Step Event Modal Upgrades**: Enhanced the Step Event Detail modal in the Builder UI to surface OS User, Authenticated Database User, and SPID / Session ID, backed by resilient dynamic column discovery in telemetry queries.
8. **Multi-Dialect DDL Schemas & Documentation**: Updated table creation scripts in `docs/db_logging_setup.md` for PostgreSQL, MySQL, SQLite, Microsoft SQL Server, and Oracle to include `os_user_name`, `db_user_name`, and `spid`, with transparent backward compatibility for legacy schemas.

---

### Included Changes

#### 1. Database Session Auditing & Identity Tracking (`spid`, `db_user_name`, `os_user_name`)
- **Telemetry Record Expansion**:
  - Added `SPID int64`, `DatabaseUserName string`, `DBUserName string`, and `OSUserName string` to `PipelineEventRecord` and `PipelineRunRecord` in `builder/telemetry.go`.
  - Added matching fields to `RunSummaryRecord` in `formating.go`.
  - Maintained backward compatibility by preserving `user_name` in all models and database outputs.
- **Dialect Session Identity Resolution (`FetchDatabaseSessionIdentity`)**:
  - Dynamically inspects the connection and resolves session metadata upon initialization:
    - **Microsoft SQL Server (`sqlserver`)**: `SELECT @@SPID, SYSTEM_USER`
    - **PostgreSQL (`postgres`)**: `SELECT pg_backend_pid(), CURRENT_USER`
    - **MySQL (`mysql`)**: `SELECT CONNECTION_ID(), CURRENT_USER()`
    - **Oracle (`oracle`)**: `SELECT TO_NUMBER(SYS_CONTEXT('USERENV', 'SID')), SYS_CONTEXT('USERENV', 'SESSION_USER') FROM DUAL`
    - **SQLite (`sqlite`)**: Host process PID `int64(os.Getpid())` and OS username (or `"sqlite"`).
- **Dynamic Schema Discovery & Backward Compatibility**:
  - `DatabaseSink` and `LogRunSummaryToDB` execute column detection probes (`SELECT * FROM <table> WHERE 1=0`) to identify table columns at runtime.
  - If a table was created with an older schema missing `spid`, `db_user_name`, or `os_user_name`, queries insert cleanly without errors. When the columns exist, they are automatically populated.
- **Dynamic Column Telemetry Queries**:
  - Refactored `QueryPipelineEvents` and `QueryPipelineRuns` in `builder/telemetry.go` to use `rows.Columns()` for dynamic scanning, eliminating scan errors or panics regardless of whether legacy or upgraded schemas are in use.
- **Runtime Engine Integration**:
  - In `main.go`, attached session identity logging under `-debug` and wired `dbSPID` and `dbUser` into both streaming execution and standard batch execution flows.

#### 2. Combined Encryption & Digital Signatures (`docs/COMBINED_ENCRYPTION_AND_SIGNATURES.md`)
- **Multi-Layer Cryptographic Architectural Guidance**:
  - Documented workflows for bundling AES-256-GCM symmetric encryption with asymmetric digital signatures into a single deployable artifact:
    - **Sign-then-Encrypt (Confidential Signature)**: Payload is signed first into a `FLOWSIGNED:v1:...` envelope, then encrypted into `FLOWENC:v1:...`. Guarantees payload confidentiality, sender authenticity, and hides signer identity from external observers.
    - **Encrypt-then-Sign (Publicly Verifiable Ciphertext)**: Payload is encrypted first into `FLOWENC:v1:...`, then signed into `FLOWSIGNED:v1:...`. Enables intermediate routers, CI/CD runners, or gateways to verify cryptographic integrity without possessing the decryption key.
    - **Unified Self-Contained Envelopes**: Combines payload and signature into a single file with zero secondary `.sig` / `.asc` / `.p7s` companion file dependencies.
- **Windows Digital Certificate Integration**:
  - Documented end-to-end Windows PowerShell workflows for exporting certificates from the Windows Certificate Store (`cert:\LocalMachine\My` or `cert:\CurrentUser\My`), creating test self-signed certificates with `New-SelfSignedCertificate`, exporting public keys with `Export-Certificate`, and signing/verifying with `crypto_tool` using sanitized, generic placeholders (no real passwords, paths, or keys).

#### 3. Comprehensive CLI Reference Guide (`docs/cli_arguments.md`)
- Published complete, structured CLI documentation covering all utilities:
  - **`flow.exe`**: Core pipeline execution flags (`-source`, `-options`, `-config`), execution variables (`-var key=value`), output formats (`-format`), debug/dry-run options, secure encryption flags (`-secure-key`, `-key-file`), digital signature verification flags (`-verify-signature`, `-cert`, `-ca-cert`, `-keyring`), draft repository import/export flags (`-import-file`, `-export-file`, `-dsn`), and Flow Builder server mode (`-builder`, `-port`).
  - **`crypto_tool`**: Complete flag reference for all actions (`encrypt`, `decrypt`, `gen-key`, `gen-keypair`, `sign`, `verify`), supported algorithms (AES-256-GCM, RSA, ECDSA, Ed25519, OpenPGP, PKCS#7), key file handling, and armor formatting (`-wrap`, `-binary`).
  - **`db_importer`**: SQL Server repository importer/exporter flags (`-server`, `-database`, `-table`, `-pipeline-name`, `-encrypted`, `-export`, `-key-file`).
  - Includes standard exit codes, environment variable precedence rules, and process table secret protection best practices.

#### 4. Flow Builder UI Enhancements & Fixes
- **On-Demand Execution Format Selection**:
  - Added an output format dropdown (`text`, `json`, `csv`, `xml`, `mermaid`) directly within the Builder pipeline execution modal.
  - Enabled when no options file is loaded, allowing developers to execute pipelines and observe immediate formatted results without authoring an XML options draft.
  - Passes format via SSE query parameters to `handleExecuteStream`.
- **File Browser Resilience & Last Known Good Directory Recovery**:
  - Fixed a UI issue where clicking the parent directory button (`..`) or navigating into an inaccessible/restricted directory caused a `failed to read directory` error that cleared the directory listing and stranded the user.
  - Implemented automatic state retention and error recovery: on navigation failure, the UI surfaces an alert banner and immediately restores the last known good directory path and listing.
- **Database Connection Test Status Indicator**:
  - Fixed the database settings tab in the Builder UI so that testing a database connection updates the driver badge highlight to emerald green (`border-emerald-700/60 bg-emerald-950/80 text-emerald-300`) upon success, or red upon failure.
  - Saved test statuses in `localStorage` under `flow_builder_db_test_statuses` so connection health state persists across tab switches and browser refreshes.
- **Step Event Detail Modal Upgrades**:
  - Updated the Builder Event Detail modal HTML in both `tmpl/index.html` and embedded `html_content.go` to display **OS User / Host**, **Database User**, and **SPID / Session**.
  - Updated `openEventDetailModal()` to populate `evt.os_user_name || evt.user_name`, `evt.db_user_name || evt.database_user_name`, and `evt.spid`.
- **Execution Logs SPID Field & Run Summary Integration**:
  - Surfaced the **SPID** column directly in the Builder Execution Logs **Pipeline Runs table** (`#log-runs-tbody`), rendering each run's database process ID in amber monospace text.
  - Added real-time SPID search filtering to `#log-runs-search`, allowing developers to search runs by database SPID/PID alongside run ID, script, host, and user.
  - Integrated **SPID** and **DB User** into the **Selected Active Run Summary Bar** (`#log-active-run-summary`), displaying session connection parameters alongside duration, status, and total row counts.
  - Added dedicated **SPID** column into the **Step Events table** (`#log-events-container`), showing session identity for individual pipeline execution steps.

#### 5. Multi-Database Logging DDL Updates (`docs/db_logging_setup.md`)
- Updated canonical DDL schemas for both `pipeline_runs` and `pipeline_events` across:
  - **PostgreSQL**: Added `os_user_name VARCHAR(256)`, `db_user_name VARCHAR(256)`, `spid BIGINT`.
  - **MySQL / MariaDB**: Added `os_user_name VARCHAR(256)`, `db_user_name VARCHAR(256)`, `spid BIGINT`.
  - **SQLite**: Added `os_user_name TEXT`, `db_user_name TEXT`, `spid INTEGER`.
  - **Microsoft SQL Server**: Added `os_user_name VARCHAR(256)`, `db_user_name VARCHAR(256)`, `spid BIGINT`.
  - **Oracle**: Added `os_user_name VARCHAR2(256)`, `db_user_name VARCHAR2(256)`, `spid NUMBER(19)`.
- Added an explanatory session auditing reference table detailing dialect resolution functions.

#### 6. Automated Testing & Verification
- Added unit tests in `formating_test.go`:
  - `TestFetchDatabaseSessionIdentity_SQLite`: Validates host PID and user retrieval.
  - `TestDatabaseSink_EmitsSPIDAndDBUser`: Verifies SPID, DB user, and OS user persistence.
  - `TestLogRunSummaryToDB_IncludesSPIDAndDBUser`: Validates run summary auditing fields.
  - `TestDatabaseSink_BackwardCompatibilityWithoutSPID`: Verifies flawless execution against legacy 15-column tables.
- Added tests in `builder/builder_test.go`:
  - `TestPipelineEventRecord_SPIDAndDBUser`: Verifies dynamic column scanning in `QueryPipelineEvents` and `QueryPipelineRuns`.
  - `TestPipelineEventModal_ContainsSPIDAndDBUser`: Verifies event detail modal field bindings.
  - `TestExecutionLogs_ContainsSPID`: Verifies SPID column headers, search filter, summary bar, and event table row bindings across both `tmpl/index.html` and embedded `html_content.go`.
- Ran full test suite across all packages (`github.com/etl-madness/go-flow`, `builder`, `crypto_tool`, `db_importer`, `pkg/flowcrypto`); all 100% passing.
- Rebuilt `flow.exe` binary.

---

## Release Notes (v1.1.22) - Cross-Platform Encryption, Digital Signatures & Crypto Tooling

### Overview

Version 1.1.22 introduces enterprise-grade security capabilities to Flow in pure Go with zero external runtime dependencies across **Windows**, **Linux**, **macOS**, and **FreeBSD**:
1. **Cross-Platform AES-256-GCM Encryption**: Authenticated symmetric encryption with PBKDF2 key derivation and text-safe armored envelopes (`FLOWENC:v1:...`) for protecting pipelines, options, and configs at rest and in database repositories.
2. **Multi-Standard Digital Signatures**: Complete cryptographic signing and verification supporting OpenSSL/PKI (RSA, ECDSA, Ed25519), PKCS#7 / CMS (`.p7s`), and OpenPGP/GPG (`.asc`, `.sig`) with a decoupled **Verify -> Decrypt -> Execute** security pipeline.
3. **Dedicated CLI Utility (`crypto_tool`)**: Standalone binary for encryption, decryption, key generation, keypair generation, self-signed X.509 certificates, and signature verification.
4. **Database & CLI Enhancements**: Native encrypted and signed pipeline loading from SQL URIs (`sql://...`), draft import/export (`-import-file`, `-export-file`, `-dsn`), and encrypted database importer (`db_importer`).

---

### Included Changes

#### 1. Native AES-256-GCM Encryption & Armored Envelope
- **Cryptographic Primitives**: AES-256 in Galois/Counter Mode (AEAD) with PBKDF2-HMAC-SHA256 (100,000 iterations) and 16-byte cryptographically secure random salts.
- **Text-Safe Armored Envelope**: `FLOWENC:v1:<base64(salt || nonce || ciphertext || auth_tag)>` prevents byte-encoding and Unicode corruption when storing encrypted content in SQL Server `NVARCHAR(MAX)` or SQLite `TEXT` columns.
- **Transparent Runtime Decryption**: `flow.exe` automatically detects `FLOWENC:v1:` payloads or decrypts on the fly using `-secure-key` or `FLOW_SECURE_KEY` / `SECURE_KEY` environment variables.
- **Tamper Proofing**: 128-bit authentication tag verification ensures any bit-flip or corrupted data is rejected before parsing or execution.

#### 2. Cross-Platform Digital Signatures
- **Multi-Standard Signature Support**:
  - **OpenSSL / PKI X.509**: RSA (PKCS#1v1.5 & PSS), ECDSA (P-256, P-384, P-521), Ed25519 PEM keys and X.509 certificates.
  - **PKCS#7 / CMS**: Detached signatures (`.p7s`) parsed and verified via pure Go ASN.1 and X.509 code signing verification.
  - **OpenPGP / GPG**: RFC 4880 ASCII-armored detached signatures (`.asc`, `.sig`) verified against public keyrings.
- **Unified Signature Envelopes**:
  - `FLOWSIG:v1:<format>:<base64-signature>`: Text-safe detached signature armor.
  - `FLOWSIGNED:v1:<format>:<base64-signature>:<base64-content>`: Self-contained signed package bundling payload and cryptographic proof.
- **Decoupled Security Pipeline**: Strict **Verify -> Decrypt -> Parse -> Execute** pipeline supports Sign-then-Encrypt, Encrypt-then-Sign, and Sign-Only workflows.
- **Companion Signature Auto-Detection**: Local file execution automatically checks for companion signature files (`<file>.sig`, `<file>.asc`, `<file>.p7s`).
- **Engine CLI Flags**: Added `-verify-signature`, `-public-key`, `-cert`, `-ca-cert`, `-keyring`, `-signature`.
- **Options Schema**: Added signature elements to `xsd/options.xsd` (`<verify-signature>`, `<public-key>`, `<cert>`, `<ca-cert>`, `<keyring>`, `<signature>`).

#### 3. Standalone CLI Utility (`crypto_tool`)
- Added standalone utility under `crypto_tool/` supporting actions:
  - `-action encrypt`: Encrypt files or stdin into `FLOWENC:v1:` armored format or raw binary (`-binary`).
  - `-action decrypt`: Decrypt armored or binary ciphertexts back to plaintext.
  - `-action gen-key`: Generate cryptographically secure 256-bit symmetric keys.
  - `-action gen-keypair`: Generate RSA (2048/4096-bit), ECDSA, Ed25519, or OpenPGP keypairs and optional self-signed X.509 code-signing certificates (`-out-cert`, `-cn`).
  - `-action sign`: Create OpenSSL, OpenPGP, or Windows PKCS#7 detached signatures or self-contained signed packages (`-wrap`).
  - `-action verify`: Verify detached signatures or extract content from `FLOWSIGNED:v1:` envelopes.

#### 4. Engine & Database Importer Enhancements
- **Draft Import & Export**: Added `-import-file`, `-import-name`, `-import-type`, `-export-file`, `-export-name`, `-export-type`, and optional `-dsn` flags to `flow.exe` for managing pipeline, options, and config items in SQLite builder storage or external databases.
- **Enhanced `db_importer`**: Added `-encrypted`, `-secure-key`, and `-export` options to import and export encrypted content directly to/from SQL Server repositories (`dbo.flow_pipeline_content`, `dbo.flow_options_content`, `dbo.flow_config_content`).

#### 5. OpenPGP Modernization (`gopenpgp/v2`)
- **Migrated to `github.com/ProtonMail/gopenpgp/v2`**: Replaced the deprecated `golang.org/x/crypto/openpgp` package across `pkg/flowcrypto` with Proton's actively maintained OpenPGP v2 library.
- **Enhanced OpenPGP Tests**: Added test coverage verifying passphrase-locked OpenPGP private key unlocking, wrong passphrase rejection, and detached signature tamper rejection.

#### 6. Comprehensive Security Hardening & Vulnerability Remediation
- **Process Table Secret Protection**: Added `-key-file` flag across `flow.exe` and `crypto_tool` to read passphrases directly from disk without exposing secrets in OS process tables (`ps aux`, Windows Task Manager). A security warning is emitted to `stderr` whenever plaintext keys are supplied via CLI flags.
- **In-Memory Key Zeroization**: Added `ZeroBytes` in `pkg/flowcrypto` and `defer ZeroBytes(derivedKey)` in `EncryptBytes` / `DecryptBytes` to immediately scrub derived AES keys from memory upon cipher initialization.
- **PKCS#7 Trust Anchor Enforcement**: Required explicit trust anchors (`-cert` or `-ca-cert`) when verifying PKCS#7 / CMS signatures, rejecting unauthenticated self-signed embedded certificates.
- **Constant-Time Verification**: Replaced variable-time digest comparison in PKCS#7 with `crypto/subtle.ConstantTimeCompare` to eliminate timing side-channel attacks.
- **Cloud Metadata SSRF Protection**: Hardened `resource_loader.go` by blocking cloud instance metadata endpoints (`169.254.169.254`, `metadata.google.internal`, `instance-data`) and IPv4/IPv6 link-local addresses.
- **Bounded Resource Ingestion**: Capped HTTP resource fetches at 50MB using `io.LimitReader` to prevent denial-of-service memory exhaustion.
- **Credential Masking in Errors**: Masked passwords and authentication credentials across ODBC, ADO.NET, and SQL Server key-value DSNs in database connection error reports.
- **XSLT SSRF & Resource DoS Prevention (SEC-13)**: Hardened custom URI resolver in `xslt_processing.go` to block requests to cloud instance metadata and link-local addresses, and capped HTTP payloads at 50MB with `io.LimitReader`.
- **XSLT Local Filesystem Sandboxing (SEC-14)**: Restricted `file://` and local path resolution within XSLT stylesheets strictly to the working directory, rejecting directory traversal attempts (`../`).
- **Builder Import Prefix Traversal Fix (SEC-15)**: Replaced naive string prefix matching with strict canonical boundary validation (`s.sanitizePath`) in `handleImportScript`.
- **Builder SSE Referer Validation & Execution Sandboxing (SEC-16)**: Added case-insensitive `Referer` origin validation when `Origin` is omitted on SSE execution streams, and enforced strict path sandboxing (`s.sanitizeExecutionPath`) for `file`, `config`, and `options` parameters.
- **Database Importer `-key-file` Support (SEC-17)**: Added `-key-file` flag to `db_importer/mssql_importer.go` along with process table exposure warnings on `-secure-key`.
- **Memory Scrubbing Compiler Optimization Guard (SEC-18)**: Added `runtime.KeepAlive(b)` to `flowcrypto.ZeroBytes` to prevent dead store elimination (DSE) by Go SSA compiler optimizations.
- **Export File Permissions Hardening (SEC-19)**: Enforced `0600` permissions (owner read/write only) on exported configuration and pipeline files in `main.go` and `db_importer/mssql_importer.go`.
- **Builder Static Asset Handler Sandboxing (SEC-20)**: Replaced unscoped directory file server `http.FileServer(http.Dir("."))` on `/flow-mascot.jpg` with a targeted `http.ServeFile` handler.

#### 7. Documentation
- Created [`docs/ENCRYPTION.md`](docs/ENCRYPTION.md): Architecture guide for AES-256-GCM encryption, envelope formats, database integration, and key resolution.
- Created [`docs/SIGNATURES.md`](docs/SIGNATURES.md): Comprehensive guide for cross-platform digital signatures, standards, key generation, and usage.
- Created [`docs/security_order_and_options.md`](docs/security_order_and_options.md) and [`SECURITY_ORDER_AND_OPTIONS.md`](SECURITY_ORDER_AND_OPTIONS.md): Exhaustive order of operations and options reference guide.
- Created [`crypto_tool/README.md`](crypto_tool/README.md): Complete CLI reference for `crypto_tool`.
- Updated [`README.md`](README.md), [`docs/database.md`](docs/database.md), and [`docs/README_DATABASE_XML.md`](docs/README_DATABASE_XML.md).

---

## Release Notes (v1.1.20) - DSN and URI source tracking for observability password masking

### Overview

Masks passwords in DSN and URI sources for improved observability and security.

---
## Release Notes (v1.1.19) - DSN and URI source tracking for observability

### Overview

This release fixes `options_path` and related source metadata when the pipeline is configured with remote or database-backed sources instead of a local filesystem path.

### Included changes

- Preserved the original DSN/URI string when `-options`, `-config`, and pipeline source paths are supplied as SQL or HTTP references.
- Centralized source detection in the shared resource loader so file paths, database URIs, and HTTP URLs are handled consistently.
- Updated run summary and event metadata to store the actual source label used at runtime, which keeps observability accurate for DB-backed option files.
- Added regression coverage validating DSN-style option sources populate the execution metadata correctly.

---

## Release Notes (v1.1.18) - Initial Overview

### Overview

This release focuses on storing XML content in the database efficiently.
Include in this release:

- Ability to import XML files into the database using the new DB importer tool.
- Support for importing pipeline, options, and config XML files separately into the database.
- Allows for Options files to be stored in the database and reference both flat-file and database-stored XML content.
- Maintains ability to store and retrieve XML content in flat files.

Import updates include the new DB importer tool and enhanced support for XML content management in the database, and is located under db_importer. This tool allows users to seamlessly import XML files into the database, manage their content efficiently, and maintain compatibility with existing flat-file storage. It also handles quote issues by importing files directly as raw content into the database.

See db_importer/README_DB_IMPORTER.md for detailed usage instructions.

## Release Notes (v1.1.17) - Resizable Live XML Preview & Offline XSLT Schema Resolution

### Overview

This release introduces an adjustable, resizable Live XML Preview panel in the Flow Visual Builder with layout persistence, and hardens the XSLT 3.0 transformation engine with automatic in-memory resolution for pipeline schemas (`pipeline.xsd`) and configurable remote URI resolution.

---

### 1. Resizable Live XML Preview Panel

- **Draggable Splitter Handle:**
  - Added a dedicated splitter divider (`#xml-preview-resizer`) between the workflow canvas and the Live XML Preview panel.
  - Interactive styling with `cursor: col-resize`, cyan accent highlighting on hover/active states, and a centered visual grab pill indicator.
  - Seamless mouse and touch drag listeners with dynamic boundaries (`min-width: 200px` and responsive max-width preserving palette and canvas space).
  - Displays a live pixel width badge during active resizing.

- **Persistent Layout Memory (`localStorage`):**
  - Saves user-adjusted panel width in `localStorage` under `flow_builder_preview_width`.
  - Automatically restores custom width on page refresh or tab switching.

- **One-Click Reset & Quick Collapse:**
  - **Double-Click or Reset Button (`⟲`):** Instantly resets the panel to standard default width (`384px`).
  - **Quick Collapse / Expand (`▶` / `◀`):** Minimizes the preview panel into a compact 42px vertical bar to maximize canvas space, saving collapse state in `localStorage` (`flow_builder_preview_collapsed`).

- **Template & Runtime Synchronization:**
  - Synchronized across the HTML template ([`builder/tmpl/index.html`](builder/tmpl/index.html)) and the embedded Go string constant ([`builder/html_content.go`](builder/html_content.go)).

- **Automated Test Coverage:**
  - Added unit test `TestXMLPreviewAdjustablePanel` in [`builder/builder_test.go`](builder/builder_test.go) verifying splitter markup, reset/collapse controls, localStorage keys, and rendered HTML responses.

---

### 2. XSLT 3.0 Processing & Embedded Schema Resolution

- **Embedded Schema Interception (`pipeline.xsd`):**
  - Resolved transformation error (`nested-schema load denied by default-deny policy: no HTTPClient or URIResolver configured`) when processing XML pipelines containing `xsi:noNamespaceSchemaLocation` or `xsi:schemaLocation` pointing to `https://raw.githubusercontent.com/etl-madness/flow/main/xsd/pipeline.xsd`.
  - Configured custom `defaultURIResolver` conforming to both `xslt3.URIResolver` and `xpath3.URIResolver`.
  - Automatically intercepts references to `pipeline.xsd` and resolves them immediately in-memory via `flow.GetSchemaXSD()`, providing zero-latency, offline documentation generation (`-xslt`).

- **Remote & Local URI Fallback:**
  - Safely falls back to remote HTTP/HTTPS queries via a configured HTTP client with timeout.
  - Resolves local `file://` URIs and filesystem paths.

- **Automated Test Coverage:**
  - Added unit test `TestProcessXSLTWithSchemaLocation` in [`mermaid_test.go`](mermaid_test.go) verifying end-to-end transformation of pipeline XML referencing remote schemas.

---

## Release Notes (v1.1.17) - Flow Visual Builder Security & Port Management

### Overview

This release introduces critical single-user security hardening, dynamic port management, CodeQL security remediations (Reflected XSS), and automated test suites for the Flow Visual Builder (`-builder`). It prevents unauthorized local or network access to the host machine's filesystem and pipeline execution APIs, protects against cross-site scripting and MIME-confusion attacks, and enables flexible port allocation with dynamic ephemeral ports by default.

---

## 1. Flow Builder Single-User Security Hardening

- **Strict Loopback Binding (`127.0.0.1`):**
  - Updated the builder web server to bind exclusively to `127.0.0.1` (loopback only) instead of `0.0.0.0`.
  - Blocks connections originating across local area networks, Wi-Fi networks, or external network adapters.

- **Ephemeral Cryptographic Auth Token:**
  - A cryptographically secure 256-bit (32-byte) random hexadecimal token (`crypto/rand`) is automatically generated at server startup.
  - The local browser is opened directly to `http://127.0.0.1:<port>/?token=<token>`.

- **Session Cookie Authentication & URL Sanitization:**
  - On the first request with `?token=...`, the server verifies the token in constant-time (`crypto/subtle.ConstantTimeCompare`) to defend against timing attacks.
  - Generates a secure session identifier stored in an in-memory session registry and sets an `HttpOnly`, `SameSite=Strict` cookie (`flow_builder_session`).
  - Automatically detects HTTPS / reverse proxies via `r.TLS != nil` or `X-Forwarded-Proto: https` to set `Secure: true` dynamically, while safely leaving it `false` on standard `http://127.0.0.1` to avoid browser cookie rejection.
  - Automatically redirects to clean `/` to strip the token from the browser's address bar and history.
  - All subsequent page loads, HTMX AJAX calls, and EventSource SSE execution streams authenticate transparently via the session cookie.

- **Programmatic API Authentication:**
  - Supports `Authorization: Bearer <token>` headers for automated tools or headless API clients.

- **Unauthorized Access Defense:**
  - Unauthenticated requests lacking a valid session cookie or token are rejected with `401 Unauthorized`.
  - Returns structured JSON for API endpoints (`/api/*` or `Accept: application/json`) and a styled authentication entry page for web browsers.

---

## 2. Port Override & Dynamic Ephemeral Port Allocation

- **Default Dynamic Ephemeral Port (`port 0`):**
  - Changed the default value of `-builder-port` from `8080` to `0`.
  - When port `0` is passed (or no port flag is specified), the operating system dynamically allocates an available loopback port via `net.Listen("tcp", "127.0.0.1:0")`.
  - Completely eliminates port collision issues when multiple instances run or when port `8080` is in use by another service.

- **Dynamic Listener Port Resolution:**
  - Server extracts the actual allocated port from the TCP listener (`listener.Addr().(*net.TCPAddr).Port`).
  - Logs and launches the browser to the exact assigned port (e.g., `http://127.0.0.1:52134/?token=...`).

- **Multi-Tier Port Configuration Hierarchy:**
  1. **CLI Flag (`-builder-port <port>`):** Highest precedence, explicitly overrides all other settings.
  2. **Options XML File (`-options <file>`):** Moved `applyXMLOptions` execution prior to builder startup in `main.go`, allowing `<builder-port value="..."/>` to configure the builder port.
  3. **Environment Variables (`FLOW_BUILDER_PORT` / `PORT`):** Evaluated when `-builder-port` is not explicitly set on the CLI.
  4. **Default (`0`):** Automatically selects an available ephemeral loopback port.

---

## 3. CodeQL Reflected XSS Remediation & Defensive Security Headers

- **Reflected Cross-Site Scripting (XSS) Remediation:**
  - Resolved CodeQL alert for Reflected Cross-Site Scripting (`go/reflected-xss`).
  - In `handleSaveFile` (`/api/save_file`), eliminated raw string byte writing (`w.Write([]byte(fmt.Sprintf(...)))`) which echoed unescaped user-supplied filenames back into HTTP responses.
  - Replaced with explicit `Content-Type: application/json` headers and structured JSON serialization via `json.NewEncoder(w).Encode(...)`.
  - Neutralized internal file write errors returned to the client to avoid reflecting raw filesystem paths.

- **Defensive Error Handling in Path Resolution (`sanitizePath`):**
  - Updated `sanitizePath` to return generic access-denial errors (`"access denied: requested path is outside the working directory"`) rather than interpolating untrusted input paths into error strings.

- **Global HTTP Security Headers Middleware:**
  - Wrapped `Server.Handler()` with a defensive security middleware that applies essential security headers to every response:
    - `X-Content-Type-Options: nosniff`: Enforces strict MIME typing and prevents browsers from sniffing non-HTML API responses as executable HTML.
    - `X-Frame-Options: DENY`: Defends against clickjacking by disallowing framing of the application.

- **Frontend Client Synchronization:**
  - Updated `saveToFileOnDisk()` in both [`builder/tmpl/index.html`](builder/tmpl/index.html) and embedded [`builder/html_content.go`](builder/html_content.go) to parse the structured JSON payload (`r.json()`) and display status messages.

---

## 4. Automated Testing & Documentation

- **Test Suite (`builder/auth_test.go`):**
  - `TestBuilderAuth_RejectUnauthorized`: Asserts 401 Unauthorized for unauthenticated browser visits (HTML error card) and API calls (JSON error).
  - `TestBuilderAuth_QueryTokenAndSessionCookie`: Tests the `?token=` parameter exchange, verifies `HttpOnly` session cookie issuance, asserts `Secure: false` on plain HTTP and `Secure: true` when `X-Forwarded-Proto: https`, tests clean 303 redirection, and validates that cookie-authenticated subsequent requests succeed.
  - `TestBuilderAuth_BearerToken`: Validates `Authorization: Bearer <token>` header on protected API endpoints.
  - `TestBuilderAuth_InvalidTokenAndFakeCookie`: Tests rejection of invalid query tokens and forged session cookies.
  - `TestBuilderAuth_PortOverrideAndDynamicPort`: Validates that binding to port `0` dynamically assigns a valid ephemeral port and updates the server instance state.
  - `TestBuilderSecurity_HeadersAndSaveFileJSON`: Asserts that global security headers (`nosniff`, `DENY`) are emitted on responses, `/api/save_file` returns structured JSON rather than echoing raw input, and directory path errors do not reflect unsanitized traversal strings.

- **Documentation Updates:**
  - [`README.md`](README.md): Updated builder launch command, feature summary, and CLI reference table with default port `0`.
  - [`docs/AUTOMATION_ETL_SUMMARY.md`](docs/AUTOMATION_ETL_SUMMARY.md): Updated visual builder CLI flag table.
  - [`docs/BUSINESS_SUMMARY.md`](docs/BUSINESS_SUMMARY.md): Updated architecture summary to reflect dynamic ephemeral port allocation and single-user security model.
