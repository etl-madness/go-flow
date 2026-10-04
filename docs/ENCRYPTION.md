# Cross-Platform Encryption Guide for Flow

Flow provides native, cross-platform AES-256-GCM encryption and decryption across **Windows**, **Linux**, **macOS**, and **FreeBSD** with zero external runtime dependencies.

This system allows you to protect sensitive pipeline configurations, connection strings, credentials, and XML option files both at rest in the filesystem and inside database repositories (`dbo.flow_pipeline_content`, `dbo.flow_options_content`, `dbo.flow_config_content`, or SQLite builder databases).

---

## 1. Cryptographic Architecture

* **Cipher:** AES-256 in Galois/Counter Mode (**AES-256-GCM**), providing authenticated encryption with associated data (AEAD).
* **Key Derivation:** **PBKDF2-HMAC-SHA256** with **100,000 iterations** and a cryptographically secure 16-byte random salt generated via `crypto/rand`.
* **Tamper Proofing:** 128-bit (16-byte) authentication tag; any modification, bit flip, or truncated payload is automatically rejected with an authentication error before any processing occurs.
* **Pure Go Implementation:** Implemented with Go's standard library (`crypto/aes`, `crypto/cipher`, `crypto/rand`, `crypto/sha256`) and pure-Go PBKDF2 (`golang.org/x/crypto/pbkdf2`). No CGO, Windows DPAPI, or macOS Keychain dependencies required.

---

## 2. Text-Safe Envelope Specification

To ensure encrypted data can be safely stored in database string columns (such as SQL Server `NVARCHAR(MAX)` or SQLite `TEXT`) without Unicode or byte-encoding corruption, Flow formats ciphertexts into an armored text envelope:

```text
FLOWENC:v1:<base64(salt || nonce || ciphertext || auth_tag)>
```

* `FLOWENC:v1:` Header prefix identifying the envelope version.
* `salt`: 16-byte random salt used during PBKDF2 derivation.
* `nonce`: 12-byte standard GCM initialization vector / nonce.
* `ciphertext`: AES-256-GCM encrypted payload.
* `auth_tag`: 16-byte authentication tag verifying payload integrity.

The envelope prefix enables `flow.exe`, `db_importer`, and `crypto_tool` to automatically detect encrypted files and provide helpful error messages if a key is missing.

---

## 3. Key Management & Precedence

Encryption and decryption keys can be passed through CLI flags or environment variables:

| Priority | Method | Description |
| :---: | :--- | :--- |
| **1 (Highest)** | `-secure-key <key>` | Command-line parameter supplied directly to `flow.exe`, `db_importer`, or `crypto_tool`. |
| **2** | `FLOW_SECURE_KEY` | Environment variable (recommended for containerized and automated production runs). |
| **3** | `SECURE_KEY` | Generic fallback environment variable. |

---

## 4. Standalone Tool: `crypto_tool`

Flow includes a dedicated CLI utility located in `crypto_tool/` to generate keys and perform offline encryption/decryption.

### Generate a Cryptographically Secure Key
```bash
# Generate a new 256-bit base64-encoded key
crypto_tool -gen-key
```

### Encrypt a File
```bash
# Encrypt an XML pipeline to text-armored format
crypto_tool -action encrypt -in pipeline.xml -out pipeline.xml.enc -secure-key "MySecretPassphrase123!"

# Encrypt using an environment variable
export FLOW_SECURE_KEY="MySecretPassphrase123!"
crypto_tool -action encrypt -in config.xml -out config.xml.enc
```

### Decrypt a File
```bash
crypto_tool -action decrypt -in pipeline.xml.enc -out pipeline.xml -secure-key "MySecretPassphrase123!"
```

### Raw Binary Mode (Optional)
If you specifically need a raw binary file instead of the armored text envelope:
```bash
crypto_tool -action encrypt -in secret.bin -out secret.enc -binary -secure-key "Key123"
```

---

## 5. Database Importer: `db_importer`

The `db_importer` tool imports and exports pipelines, options, and configs to/from SQL Server repositories, with support for encryption at rest:

### Import and Encrypt into SQL Server
```bash
# Encrypts the local file before inserting/merging into dbo.flow_pipeline_content
db_importer -dsn "sqlserver://sa:Password123!@localhost:1433?database=master" \
            -table pipeline \
            -name "daily_customer_sync" \
            -file "pipelines/sync.xml" \
            -encrypted \
            -secure-key "MySecretPassphrase123!"
```

### Export from SQL Server and Decrypt
```bash
# Fetches from dbo.flow_pipeline_content, decrypts in memory, and writes plain XML to disk
db_importer -action export \
            -dsn "sqlserver://sa:Password123!@localhost:1433?database=master" \
            -table pipeline \
            -name "daily_customer_sync" \
            -file "sync_decrypted.xml" \
            -encrypted \
            -secure-key "MySecretPassphrase123!"
```

---

## 6. Running Encrypted Pipelines with `flow.exe`

`flow.exe` can execute encrypted pipelines, options, and configuration overrides whether they reside on disk, an HTTP endpoint, or inside a SQL database.

### Running Encrypted Files from Disk
```bash
flow.exe -file pipeline.xml.enc -encrypted -secure-key "MySecretPassphrase123!"
```

### Running Encrypted Pipelines from a Database
```bash
flow.exe -encrypted -secure-key "MySecretPassphrase123!" \
         -file "sql://sqlserver@localhost:1433?database=master#SELECT PipelineXML FROM dbo.flow_pipeline_content WHERE Name = 'daily_customer_sync'"
```

### Running Encrypted Options from a Database
```bash
flow.exe -encrypted -secure-key "MySecretPassphrase123!" \
         -options "sql://sqlserver@localhost:1433?database=master#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name = 'PROD_OPTIONS'"
```

### Auto-Detection
If `flow.exe` loads a file or database record beginning with `FLOWENC:v1:`, it automatically detects that the resource is encrypted and uses the configured key (`-secure-key` or `FLOW_SECURE_KEY`) without requiring the `-encrypted` flag.

---

## 7. `flow.exe` Draft Import & Export

`flow.exe` can also export and import drafts directly from the local SQLite builder database (`flow_builder.db`) or external databases:

### Exporting an Encrypted Pipeline Draft
```bash
flow.exe -export-file pipeline_backup.xml.enc \
         -export-name "daily_etl" \
         -export-type pipeline \
         -encrypted \
         -secure-key "MySecretPassphrase123!"
```

### Importing an Encrypted Draft into Builder Storage
```bash
# Decrypts the draft on import so it can be visually inspected and edited in the builder
flow.exe -import-file pipeline_backup.xml.enc \
         -import-name "daily_etl" \
         -import-type pipeline \
         -secure-key "MySecretPassphrase123!"
```

---

## 8. Digital Signatures & Security Architecture

Flow includes comprehensive digital signature creation and verification supporting OpenSSL (RSA, ECDSA, Ed25519), OpenPGP / GPG, and Windows Authenticode / PKCS#7 in pure Go with zero OS dependencies.

* **Decoupled Security Pipeline:** Signature verification occurs prior to decryption (**Verify -> Decrypt -> Parse -> Execute**), preventing tampered or unauthorized payloads from being processed.
* **Full Documentation:** See [**`SIGNATURES.md`**](SIGNATURES.md) for complete instructions, key generation guides, and engine flags (`-verify-signature`, `-public-key`, `-cert`, `-ca-cert`, `-keyring`, `-signature`).

