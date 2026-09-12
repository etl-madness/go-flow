# Release Notes

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
