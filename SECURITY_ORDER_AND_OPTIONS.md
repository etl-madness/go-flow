# Security Architecture: Cryptographic Order of Operations & Options Guide

This document details the exact **order of operations** for combining digital signatures and encryption in `go-flow`, explains the cryptographic design rationale, and provides a complete reference for all security options and CLI flags across `flow.exe`, `crypto_tool`, `db_importer`, and XML configuration files.

---

## 1. Quick Reference: Which Order Should You Use?

| Security Pattern | Creation Order | Runtime Execution Order | Primary Advantage | Best Used When |
| :--- | :--- | :--- | :--- | :--- |
| **Encrypt-then-Sign**<br>*(Recommended Standard)* | 1. Encrypt XML<br>2. Sign Ciphertext | 1. Verify Signature<br>2. Decrypt Ciphertext<br>3. Parse & Run | **Zero Attack Surface:** Untrusted/tampered inputs are rejected before decryption keys or memory are touched. | Standard production workflows, public/untrusted transport, CI/CD distribution, and cloud deployments. |
| **Sign-then-Encrypt**<br>*(Confidential Signer)* | 1. Sign XML<br>2. Encrypt Package | 1. Decrypt Ciphertext<br>2. Verify Signature<br>3. Parse & Run | **Signer Privacy:** The digital signature, certificate, and signer identity are encrypted and hidden from external observers. | Sensitive corporate environments where the identity of the signer or build server must remain confidential. |
| **Detached Signature** | Sign file to `.sig` / `.p7s` / `.asc` | 1. Verify Detached Sig<br>2. Decrypt (if needed)<br>3. Parse & Run | **Non-Invasive:** Original XML or encrypted file remains completely untouched. | Auditing, compliance regimes, and external repository verification without wrapper envelopes. |

### Architectural Comparison

```mermaid
flowchart TD
    subgraph EncryptThenSign["Workflow A: Encrypt-then-Sign (Recommended Standard)"]
        direction TB
        A1["Plaintext XML"] -->|"1. AES-256-GCM"| A2["FLOWENC Ciphertext"]
        A2 -->|"2. Sign Outer"| A3["FLOWSIGNED Package"]
        A3 -->|"3. flow.exe: Verify Sig First"| A4["Verified Ciphertext"]
        A4 -->|"4. flow.exe: Decrypt Second"| A5["Verified AST Execution"]
    end

    subgraph SignThenEncrypt["Workflow B: Sign-then-Encrypt (Confidential Signer)"]
        direction TB
        B1["Plaintext XML"] -->|"1. Sign Plaintext"| B2["FLOWSIGNED Envelope"]
        B2 -->|"2. AES-256-GCM"| B3["FLOWENC Package"]
        B3 -->|"3. flow.exe: Decrypt First"| B4["Decrypted FLOWSIGNED"]
        B4 -->|"4. flow.exe: Verify Sig Second"| B5["Verified AST Execution"]
    end
```

---

## 2. Order of Operations Deep Dive

### Pattern A: Encrypt-then-Sign (Recommended Standard)

In modern cryptography (TLS 1.3, IPSec, SSH, Signal), **Encrypt-then-Authenticate/Sign** is the industry standard. The ciphertext is signed so that the receiving engine can verify data authenticity and author identity **before** attempting decryption.

#### Creation Flow (`crypto_tool`):

```mermaid
flowchart LR
    XML["Plaintext XML<br>(pipeline.xml)"] -->|"Step 1: AES-256-GCM<br>(-action encrypt)"| ENC["Ciphertext<br>FLOWENC:v1:..."]
    ENC -->|"Step 2: Sign Ciphertext<br>(-action sign -wrap)"| SIGNED["Signed Package<br>FLOWSIGNED:v1:..."]
```

1. **Step 1: Encrypt Plaintext XML**
   ```bash
   crypto_tool -action encrypt -in pipeline.xml -out pipeline.xml.enc -secure-key "SecretPassphrase"
   ```
   *Result:* Creates a `FLOWENC:v1:...` text-armored ciphertext.

2. **Step 2: Sign Ciphertext into Unified Package**
   ```bash
   crypto_tool -action sign -in pipeline.xml.enc -out pipeline.xml.signed -private-key priv.pem -wrap
   ```
   *Result:* Wraps the ciphertext into `FLOWSIGNED:v1:OPENSSL-RSA-SHA256:<base64-signature>:<base64-FLOWENC-ciphertext>`.

#### Runtime Execution Flow (`flow.exe`):

```mermaid
flowchart TD
    INPUT["Input Package<br>(pipeline.xml.signed)"] --> V_SIG{"1. Verify Digital Signature<br>(Public Key / Certificate)"}
    
    V_SIG -->|"Signature Valid"| DEC{"2. Decrypt AES-256-GCM<br>(PBKDF2 Key Derivation)"}
    V_SIG -->|"Signature Invalid / Untrusted"| ABORT1["Immediate Abort (Exit 1)<br>Decryption key never touched"]
    
    DEC -->|"Key Valid & Authenticated"| PARSE["3. Parse XML Schema & AST"]
    DEC -->|"Wrong Key / Tampered Tag"| ABORT2["Immediate Abort (Exit 1)<br>Decryption Failed"]
    
    PARSE --> EXEC["Execute Flow Workflows"]

    classDef pass fill:#e6fffa,stroke:#047857,stroke-width:2px;
    classDef fail fill:#ffebe9,stroke:#cf222e,stroke-width:2px;
    classDef stage fill:#f0f9ff,stroke:#0284c7,stroke-width:2px;
    class V_SIG,DEC,PARSE stage;
    class EXEC pass;
    class ABORT1,ABORT2 fail;
```

*Command to execute:*
```bash
flow.exe -file pipeline.xml.signed -public-key pub.pem -secure-key "SecretPassphrase"
```

---

### Pattern B: Sign-then-Encrypt (Confidential Signatures)

In this pattern, the author signs the plaintext XML first, and then the entire signed package is encrypted. Only entities possessing the decryption key can view the pipeline **and** verify who signed it.

#### Creation Flow (`crypto_tool`):

```mermaid
flowchart LR
    XML["Plaintext XML<br>(pipeline.xml)"] -->|"Step 1: Sign XML<br>(-action sign -wrap)"| SIGNED["Signed Envelope<br>FLOWSIGNED:v1:..."]
    SIGNED -->|"Step 2: AES-256-GCM<br>(-action encrypt)"| ENC["Encrypted Package<br>FLOWENC:v1:..."]
```

1. **Step 1: Sign Plaintext XML into Unified Package**
   ```bash
   crypto_tool -action sign -in pipeline.xml -out pipeline.xml.signed -private-key priv.pem -wrap
   ```

2. **Step 2: Encrypt the Signed Package**
   ```bash
   crypto_tool -action encrypt -in pipeline.xml.signed -out pipeline.xml.enc -secure-key "SecretPassphrase"
   ```

#### Runtime Execution Flow (`flow.exe`):

```mermaid
flowchart TD
    INPUT["Encrypted Resource<br>(FLOWENC:v1:...)"] --> DEC{"1. Decrypt Outer Envelope<br>(AES-256-GCM with -secure-key)"}
    
    DEC -->|"Decryption Successful"| DETECT{"Detect Inner Content"}
    DEC -->|"Wrong Key / Tampered Tag"| ABORT1["Immediate Abort (Exit 1)<br>Decryption Failed"]
    
    DETECT -->|"Inner Content is FLOWSIGNED:v1"| V_SIG{"2. Verify Digital Signature<br>(Public Key / Certificate)"}
    DETECT -->|"Inner Content is Plaintext XML"| PARSE["3. Parse XML Schema & AST"]
    
    V_SIG -->|"Signature Valid"| PARSE
    V_SIG -->|"Signature Invalid / Untrusted"| ABORT2["Immediate Abort (Exit 1)<br>Signature Verification Failed"]
    
    PARSE --> EXEC["Execute Flow Workflows"]

    classDef pass fill:#e6fffa,stroke:#047857,stroke-width:2px;
    classDef fail fill:#ffebe9,stroke:#cf222e,stroke-width:2px;
    classDef stage fill:#f0f9ff,stroke:#0284c7,stroke-width:2px;
    class DEC,DETECT,V_SIG,PARSE stage;
    class EXEC pass;
    class ABORT1,ABORT2 fail;
```

*Command to execute:*
```bash
flow.exe -file pipeline.xml.enc -secure-key "SecretPassphrase" -public-key pub.pem
```

---

### Pattern C: Detached Signatures (Unwrapped Files)

In this pattern, signatures are kept as standalone companion files (`.sig`, `.asc`, or `.p7s`) alongside the target file.

```mermaid
flowchart TD
    TARGET["Pipeline Resource<br>(pipeline.xml)"] --> CHECK{"Check Companion Files"}
    CHECK -->|"Found pipeline.xml.sig"| VER_SIG["Verify via OpenSSL / Raw Sig"]
    CHECK -->|"Found pipeline.xml.p7s"| VER_P7S["Verify via Windows PKCS#7"]
    CHECK -->|"Found pipeline.xml.asc"| VER_PGP["Verify via OpenPGP Keyring"]
    CHECK -->|"No Companion File Found"| EXPLICIT{"-signature Flag Specified?"}
    
    EXPLICIT -->|"Yes"| VER_EXP["Verify Explicit Signature File"]
    EXPLICIT -->|"No"| RUN_DIRECT["Execute Unsigned (if allowed)"]
    
    VER_SIG & VER_P7S & VER_PGP & VER_EXP --> DEC_CHECK{"Is Content Encrypted?"}
    DEC_CHECK -->|"Yes (FLOWENC:v1)"| DEC["Decrypt AES-256-GCM"]
    DEC_CHECK -->|"No (Plaintext)"| PARSE["Parse Schema & AST"]
    DEC --> PARSE
    PARSE --> EXEC["Execute Flow Workflows"]
```

* **Create Detached Signature:**
  ```bash
  # Sign plaintext or encrypted file
  crypto_tool -action sign -in pipeline.xml -out pipeline.xml.sig -private-key priv.pem
  ```

* **Execute with Companion Auto-Detection:**
  `flow.exe` automatically looks for `pipeline.xml.sig`, `pipeline.xml.asc`, or `pipeline.xml.p7s` when `-public-key` or `-cert` is specified:
  ```bash
  flow.exe -file pipeline.xml -public-key pub.pem
  ```

* **Execute with Explicit Signature Path:**
  ```bash
  flow.exe -file pipeline.xml -signature /path/to/custom_signature.sig -public-key pub.pem
  ```

---

## 3. Comprehensive Options & Flags Matrix

### Runtime Engine: `flow.exe`

| CLI Flag | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `-file` | `string` | `scripts.xml` | Path to pipeline XML or `sql://...` database URI. |
| `-encrypted` | `bool` | `false` | Explicitly declares that resource is encrypted. *(Note: Auto-detected if payload starts with `FLOWENC:v1:`)*. |
| `-secure-key` | `string` | `""` | Passphrase or key for AES-256 decryption. Falls back to `FLOW_SECURE_KEY` or `SECURE_KEY`. |
| `-verify-signature` | `bool` | `false` | Enforce signature verification before decrypting or running pipeline. |
| `-public-key` | `string` | `""` | Public key file path (RSA, ECDSA, Ed25519 PEM or OpenPGP keyring) for verification. |
| `-cert` | `string` | `""` | X.509 certificate file path (PEM or DER) for digital signature verification. |
| `-ca-cert` | `string` | `""` | Root CA certificate for certificate chain verification. |
| `-keyring` | `string` | `""` | OpenPGP armored public keyring file path for signature verification. |
| `-signature` | `string` | `""` | Explicit detached digital signature file path (auto-checks `<in>.sig`, `<in>.asc`, `<in>.p7s` if omitted). |
| `-options` | `string` | `""` | Path to XML options file (can be encrypted, signed, or plain). |
| `-config` | `string` | `""` | Path to config override XML file (can be encrypted, signed, or plain). |
| `-import-file` | `string` | `""` | Import file into database storage (auto-verifies signatures and decrypts on import). |
| `-export-file` | `string` | `""` | Export pipeline/options/config from database storage (supports `-encrypted`). |
| `-dsn` | `string` | `""` | External database DSN for import/export directly to external DB. |

---

### Security Utility: `crypto_tool`

| CLI Flag | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `-action` | `string` | `encrypt` | Action to execute: `encrypt`, `decrypt`, `gen-key`, `sign`, `verify`, or `gen-keypair`. |
| `-in` | `string` | `""` | Input file path (or `-` / empty for `stdin`). |
| `-out` | `string` | `""` | Output file path (or `-` / empty for `stdout`). |
| `-secure-key`, `-key` | `string` | `""` | Key or passphrase for encryption/decryption. |
| `-binary` | `bool` | `false` | Emit/read raw binary ciphertext instead of armored text (`FLOWENC:v1:`). |
| `-sig-type` | `string` | `openssl` | Signature format for `-action sign`: `openssl`, `pgp`, or `pkcs7`. |
| `-signature`, `-sig` | `string` | `""` | Detached signature file path for `-action verify`. |
| `-public-key`, `-pub` | `string` | `""` | Public key file path for verification. |
| `-private-key`, `-priv` | `string` | `""` | Private key file path for signing. |
| `-cert` | `string` | `""` | X.509 certificate file path (PEM or DER) for signing (PKCS#7) or verification. |
| `-ca-cert` | `string` | `""` | Root CA certificate for chain validation. |
| `-keyring` | `string` | `""` | OpenPGP armored public keyring file path. |
| `-wrap` | `bool` | `false` | When signing, bundle signature and content into unified `FLOWSIGNED:v1:...` envelope. |
| `-keypair-type` | `string` | `rsa` | Key type for `-action gen-keypair`: `rsa`, `ecdsa`, `ed25519`, or `pgp`. |
| `-bits` | `int` | `2048` | RSA key size in bits (`2048` or `4096`). |
| `-out-priv` | `string` | `private_key.pem` | Generated private key output file path. |
| `-out-pub` | `string` | `public_key.pem` | Generated public key output file path. |
| `-out-cert` | `string` | `""` | Optional output file path for self-signed X.509 certificate. |
| `-cn` | `string` | `Flow Signer` | Common Name for self-signed certificate or OpenPGP identity. |

---

### Database Importer: `db_importer`

| CLI Flag | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `-action` | `string` | `import` | Action to perform: `import` or `export`. |
| `-dsn` | `string` | *(default)* | SQL Server connection string / DSN. |
| `-table` | `string` | `pipeline` | Target table type: `pipeline`, `options`, or `config`. |
| `-name` | `string` | `""` | Repository key name for the imported/exported item. |
| `-file` | `string` | `""` | Local file to import, or destination path when exporting. |
| `-desc` | `string` | *(default)* | Description of the XML content (import only). |
| `-encrypted` | `bool` | `false` | Encrypts content before importing, or decrypts content after exporting. |
| `-secure-key` | `string` | `""` | Passphrase/key for AES-256-GCM encryption/decryption. |
| `-export` | `bool` | `false` | Shortcut flag to export record from database to file. |

---

### XML Options Configuration (`options.xml`)

You can define security options inside XML options profiles conforming to [`xsd/options.xsd`](xsd/options.xsd):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<flow_cli_options>
    <options>
        <!-- Signature Verification Options -->
        <verify-signature>true</verify-signature>
        <public-key>/etc/flow/keys/production_pub.pem</public-key>
        <cert>/etc/flow/certs/code_signing.crt</cert>
        <ca-cert>/etc/flow/certs/root_ca.crt</ca-cert>
        <keyring>/etc/flow/keys/pgp_keyring.asc</keyring>
        <signature>/var/signatures/pipeline.sig</signature>

        <!-- Symmetric Encryption Options -->
        <encrypted>true</encrypted>
        <secure-key>ProductionSecretKey2026</secure-key>
    </options>
</flow_cli_options>
```

---

## 4. Key & Secret Precedence Hierarchy

When multiple sources provide keys, `flow.exe` and `crypto_tool` resolve them strictly in the following order:

```mermaid
flowchart TD
    P1["1. Explicit CLI Flag<br>(-secure-key / -key)"]
    P2["2. XML Options Profile<br>(&lt;secure-key&gt; in -options)"]
    P3["3. FLOW_SECURE_KEY<br>(Environment Variable)"]
    P4["4. SECURE_KEY<br>(Generic Fallback Env Var)"]

    P1 -->|"If not provided"| P2
    P2 -->|"If not provided"| P3
    P3 -->|"If not provided"| P4

    classDef high fill:#e6fffa,stroke:#047857,stroke-width:2px;
    classDef item fill:#f0f9ff,stroke:#0284c7,stroke-width:2px;
    class P1 high;
    class P2,P3,P4 item;
```

---

## 5. End-to-End Practical Recipes

### Recipe 1: Production CI/CD Release (Encrypt-then-Sign)

```bash
# 1. Generate release keypair and certificate (one time setup)
crypto_tool -action gen-keypair -keypair-type rsa -bits 2048 \
  -out-priv release_priv.pem -out-pub release_pub.pem \
  -out-cert release_cert.pem -cn "Release Build Agent"

# 2. Encrypt the production pipeline with a random 256-bit AES key
KEY=$(crypto_tool -gen-key)
crypto_tool -action encrypt -in pipeline.xml -out pipeline.xml.enc -secure-key "$KEY"

# 3. Sign the encrypted pipeline using the private key into a single package
crypto_tool -action sign -in pipeline.xml.enc -out pipeline.xml.signed -private-key release_priv.pem -wrap

# 4. Deploy pipeline.xml.signed to production servers. Run with:
export FLOW_SECURE_KEY="$KEY"
flow.exe -file pipeline.xml.signed -public-key release_pub.pem
```

---

### Recipe 2: Windows Authenticode / PKCS#7 CMS Workflow (`.p7s`)

```bash
# 1. Sign using an X.509 certificate and private key (creates standard PKCS#7 detached signature)
crypto_tool -action sign \
  -in scripts.xml \
  -out scripts.xml.p7s \
  -private-key signer_key.pem \
  -cert signer_cert.pem \
  -sig-type pkcs7

# 2. Execute on Windows, Linux, macOS, or FreeBSD (pure Go parses PKCS#7 ASN.1 structure)
flow.exe -file scripts.xml -cert signer_cert.pem
```

---

### Recipe 3: Storing Signed & Encrypted Pipelines in SQL Server

```bash
# 1. Import signed and encrypted pipeline package into SQL Server repository
flow.exe -dsn "sqlserver://sa:Password@localhost:1433?database=ETL" \
         -import-file pipeline.xml.signed \
         -import-name "NightlyBillingJob" \
         -import-type pipeline

# 2. Execute directly from SQL Server URI:
flow.exe \
  -file "sql://sqlserver@localhost:1433?database=ETL#SELECT PipelineXML FROM dbo.flow_pipeline_content WHERE Name='NightlyBillingJob'" \
  -public-key release_pub.pem \
  -secure-key "$KEY"
```

---

## 6. Security Guarantees & Failure Handling

* **Tampered Signature or Payload:** If any byte of the signature, ciphertext, or plaintext is altered, verification fails immediately with `exit code 1`. Zero child processes or database connections are spawned.
* **Missing or Invalid Key:** If an encrypted file is provided without a key, execution halts with `exit code 1` with a descriptive message prompting for `-secure-key` or `FLOW_SECURE_KEY`.
* **Zero Binary Dependencies:** No system installations of `openssl`, `gpg`, or `signtool.exe` are invoked at runtime. The execution behavior and security boundaries are 100% identical on Windows, Linux, macOS, and FreeBSD.
