# `crypto_tool` CLI Reference

`crypto_tool` is a standalone cross-platform command-line utility for encryption, decryption, digital signature signing and verification, and cryptographic keypair generation for Flow pipelines, options, configs, and generic files.

It compiles as a single lightweight binary with zero external runtime dependencies on **Windows**, **Linux**, **macOS**, and **FreeBSD**.

---

## Installation & Build

Build the tool using Go:

```bash
# Windows
go build -o crypto_tool.exe ./crypto_tool

# Linux / macOS / FreeBSD
go build -o crypto_tool ./crypto_tool
```

---

## Command-Line Flags

### Encryption & Symmetric Keys
| Flag | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `-action` | `string` | `encrypt` | Action to execute: `encrypt`, `decrypt`, `gen-key`, `sign`, `verify`, or `gen-keypair`. |
| `-gen-key` | `bool` | `false` | Shortcut for `-action gen-key`. Generates a 256-bit cryptographically secure key. |
| `-in` | `string` | `""` | Input file path (or `-` / empty for standard input `stdin`). |
| `-out` | `string` | `""` | Output file path (or `-` / empty for standard output `stdout`). |
| `-secure-key` | `string` | `""` | Encryption or decryption key/passphrase. Falls back to `FLOW_SECURE_KEY` or `SECURE_KEY` env vars. |
| `-key` | `string` | `""` | Alias for `-secure-key`. |
| `-binary` | `bool` | `false` | Output raw binary ciphertext or signature instead of the default text-safe armored format. |

### Digital Signatures & Asymmetric Keypairs
| Flag | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `-sig-type` | `string` | `openssl` | Signature format for `-action sign`: `openssl`, `pgp`, or `pkcs7`. |
| `-signature`, `-sig` | `string` | `""` | Detached signature file path for `-action verify`. |
| `-public-key`, `-pub` | `string` | `""` | Public key file path (PEM or OpenPGP) for verification. |
| `-private-key`, `-priv` | `string` | `""` | Private key file path (PEM or OpenPGP) for signing. |
| `-cert` | `string` | `""` | X.509 certificate file path (PEM or DER) for signing (PKCS#7) or verification. |
| `-ca-cert` | `string` | `""` | Root CA certificate for certificate chain validation. |
| `-keyring` | `string` | `""` | OpenPGP armored public keyring file path for signature verification. |
| `-wrap` | `bool` | `false` | When signing, bundle both the signature and content into a single self-verifying `FLOWSIGNED:v1:...` envelope. |
| `-keypair-type` | `string` | `rsa` | Keypair type for `-action gen-keypair`: `rsa`, `ecdsa`, `ed25519`, or `pgp`. |
| `-bits` | `int` | `2048` | Key size in bits for RSA keypair generation. |
| `-out-priv` | `string` | `private_key.pem` | Output path for generated private key. |
| `-out-pub` | `string` | `public_key.pem` | Output path for generated public key. |
| `-out-cert` | `string` | `""` | Optional output path to generate a self-signed X.509 certificate during keypair generation. |
| `-cn` | `string` | `Flow Signer` | Common Name for self-signed X.509 certificate or OpenPGP identity. |

---

## Usage Examples

### 1. Symmetric Key & Encryption

#### Generate a 256-bit Secure Key
```bash
crypto_tool -gen-key
crypto_tool -gen-key -out flow_key.txt
```

#### Encrypt Files (Text-Armored Envelope)
```bash
crypto_tool -action encrypt -in scripts.xml -out scripts.xml.enc -secure-key "MySecretPassphrase123!"
```

#### Decrypt Files
```bash
crypto_tool -action decrypt -in scripts.xml.enc -out scripts.xml -secure-key "MySecretPassphrase123!"
```

---

### 2. Generating Digital Signature Keypairs

#### Generate RSA Keypair & Self-Signed X.509 Certificate
```bash
crypto_tool -action gen-keypair -keypair-type rsa -bits 2048 \
  -out-priv priv.pem -out-pub pub.pem -out-cert cert.pem -cn "Flow Code Signer"
```

#### Generate ECDSA (P-256) or Ed25519 Keypair
```bash
crypto_tool -action gen-keypair -keypair-type ecdsa -out-priv ec_priv.pem -out-pub ec_pub.pem
crypto_tool -action gen-keypair -keypair-type ed25519 -out-priv ed_priv.pem -out-pub ed_pub.pem
```

#### Generate OpenPGP / GPG Armored Keypair
```bash
crypto_tool -action gen-keypair -keypair-type pgp -out-priv gpg_priv.asc -out-pub gpg_pub.asc -cn "Release Manager"
```

---

### 3. Signing Files

#### OpenSSL Detached Signature
```bash
crypto_tool -action sign -in scripts.xml -out scripts.xml.sig -private-key priv.pem -sig-type openssl
```

#### PKCS#7 / CMS Detached Signature (`.p7s`)
```bash
crypto_tool -action sign -in scripts.xml -out scripts.xml.p7s -private-key priv.pem -cert cert.pem -sig-type pkcs7
```

#### OpenPGP / GPG Detached Signature (`.asc`)
```bash
crypto_tool -action sign -in scripts.xml -out scripts.xml.asc -private-key gpg_priv.asc -sig-type pgp
```

#### Self-Contained Signed Package (`-wrap`)
Embeds signature and file content into a single `FLOWSIGNED:v1:...` envelope:
```bash
crypto_tool -action sign -in scripts.xml -out scripts.xml.signed -private-key priv.pem -wrap
```

---

### 4. Verifying Signatures

#### Verify Detached Signature
```bash
crypto_tool -action verify -in scripts.xml -signature scripts.xml.sig -public-key pub.pem
```
*(If `-signature` is omitted, `crypto_tool` automatically checks for companion files `<in>.sig`, `<in>.asc`, or `<in>.p7s`)*.

#### Verify PKCS#7 / CMS Signature
```bash
crypto_tool -action verify -in scripts.xml -signature scripts.xml.p7s -cert cert.pem
```

#### Verify OpenPGP Armored Signature
```bash
crypto_tool -action verify -in scripts.xml -signature scripts.xml.asc -keyring gpg_pub.asc
```

#### Verify and Extract Content from Unified Envelope
```bash
crypto_tool -action verify -in scripts.xml.signed -public-key pub.pem -out verified_scripts.xml
```

---

## Key Resolution Precedence

When `-action encrypt` or `-action decrypt` is invoked, `crypto_tool` resolves the key in the following order:

1. `-secure-key` or `-key` CLI flag
2. `FLOW_SECURE_KEY` environment variable
3. `SECURE_KEY` environment variable

---

## Error Handling & Exit Codes

* **Exit Code `0`:** Operation completed successfully (verification PASSED, encryption/decryption succeeded).
* **Exit Code `1`:** Execution error (verification FAILED, missing key, invalid passphrase, or tampered file).
