# Release Notes (v1.1.15) - Flow Visual Builder Security & Port Management

## Overview

This release introduces critical single-user security hardening, dynamic port management, and test automation for the Flow Visual Builder (`-builder`). It prevents unauthorized local or network access to the host machine's filesystem and pipeline execution APIs, and allows flexible port allocation with dynamic ephemeral ports by default.

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

## 3. Automated Testing & Documentation

- **Test Suite (`builder/auth_test.go`):**
  - `TestBuilderAuth_RejectUnauthorized`: Asserts 401 Unauthorized for unauthenticated browser visits (HTML error card) and API calls (JSON error).
  - `TestBuilderAuth_QueryTokenAndSessionCookie`: Tests the `?token=` parameter exchange, verifies `HttpOnly` session cookie issuance, tests clean 303 redirection, and validates that cookie-authenticated subsequent requests succeed.
  - `TestBuilderAuth_BearerToken`: Validates `Authorization: Bearer <token>` header on protected API endpoints.
  - `TestBuilderAuth_InvalidTokenAndFakeCookie`: Tests rejection of invalid query tokens and forged session cookies.
  - `TestBuilderAuth_PortOverrideAndDynamicPort`: Validates that binding to port `0` dynamically assigns a valid ephemeral port and updates the server instance state.

- **Documentation Updates:**
  - [`README.md`](README.md): Updated builder launch command, feature summary, and CLI reference table with default port `0`.
  - [`docs/AUTOMATION_ETL_SUMMARY.md`](docs/AUTOMATION_ETL_SUMMARY.md): Updated visual builder CLI flag table.
  - [`docs/BUSINESS_SUMMARY.md`](docs/BUSINESS_SUMMARY.md): Updated architecture summary to reflect dynamic ephemeral port allocation and single-user security model.
