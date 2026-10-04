package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/etl-madness/go-flow/pkg/flowcrypto"
)

func resolveSecureKey(cliKey string) string {
	if strings.TrimSpace(cliKey) != "" {
		return strings.TrimSpace(cliKey)
	}
	if envKey := strings.TrimSpace(os.Getenv("FLOW_SECURE_KEY")); envKey != "" {
		return envKey
	}
	if envKey := strings.TrimSpace(os.Getenv("SECURE_KEY")); envKey != "" {
		return envKey
	}
	return ""
}

func readInput(inputPath string) ([]byte, error) {
	if inputPath == "" || inputPath == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(inputPath)
}

func writeOutput(outputPath string, data []byte) error {
	if outputPath == "" || outputPath == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

func main() {
	actionFlag := flag.String("action", "encrypt", "Action to perform: encrypt, decrypt, gen-key, sign, verify, gen-keypair")
	genKeyFlag := flag.Bool("gen-key", false, "Shortcut to generate a cryptographically secure key")
	inFileFlag := flag.String("in", "", "Input file path (or '-' / empty for stdin)")
	outFileFlag := flag.String("out", "", "Output file path (or '-' / empty for stdout)")
	keyFlag := flag.String("secure-key", "", "Key or passphrase for encryption/decryption (falls back to FLOW_SECURE_KEY)")
	keyAlias := flag.String("key", "", "Alias for -secure-key")
	binaryFlag := flag.Bool("binary", false, "Output raw binary ciphertext instead of armored text format")

	// Signature & Keypair flags
	sigTypeFlag := flag.String("sig-type", "openssl", "Signature type: openssl, pgp, or pkcs7")
	sigFileFlag := flag.String("signature", "", "Signature file path (or use alias -sig)")
	sigFileAlias := flag.String("sig", "", "Alias for -signature")
	pubKeyFlag := flag.String("public-key", "", "Public key file path (PEM, OpenPGP, or alias -pub)")
	pubKeyAlias := flag.String("pub", "", "Alias for -public-key")
	privKeyFlag := flag.String("private-key", "", "Private key file path (PEM, OpenPGP, or alias -priv)")
	privKeyAlias := flag.String("priv", "", "Alias for -private-key")
	certFlag := flag.String("cert", "", "X.509 Certificate file path (PEM or DER)")
	caCertFlag := flag.String("ca-cert", "", "Root CA certificate file path for chain validation")
	keyringFlag := flag.String("keyring", "", "OpenPGP public/private keyring file path")
	passphraseFlag := flag.String("passphrase", "", "Passphrase for encrypted private keys")
	wrapFlag := flag.Bool("wrap", false, "Wrap signed output into a unified FLOWSIGNED:v1: envelope with content")
	keypairType := flag.String("keypair-type", "rsa", "Keypair type for gen-keypair: rsa, ecdsa, ed25519, or pgp")
	keyBits := flag.Int("bits", 2048, "Bit length for RSA key generation")
	outPrivFlag := flag.String("out-priv", "private_key.pem", "Output file path for private key during gen-keypair")
	outPubFlag := flag.String("out-pub", "public_key.pem", "Output file path for public key during gen-keypair")
	outCertFlag := flag.String("out-cert", "", "Optional output path to generate a self-signed X.509 certificate during gen-keypair")
	cnFlag := flag.String("cn", "Flow Code Signer", "Common name for self-signed certificate or OpenPGP identity")

	flag.Parse()

	// Resolve aliases
	effectiveSigFile := *sigFileFlag
	if effectiveSigFile == "" {
		effectiveSigFile = *sigFileAlias
	}
	effectivePubKey := *pubKeyFlag
	if effectivePubKey == "" {
		effectivePubKey = *pubKeyAlias
	}
	effectivePrivKey := *privKeyFlag
	if effectivePrivKey == "" {
		effectivePrivKey = *privKeyAlias
	}

	action := strings.ToLower(strings.TrimSpace(*actionFlag))

	// 1. Handle Key Generation (Symmetric)
	if *genKeyFlag || action == "gen-key" {
		key, err := flowcrypto.GenerateSecureKey(32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating secure key: %v\n", err)
			os.Exit(1)
		}
		if err := writeOutput(*outFileFlag, []byte(key+"\n")); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing generated key: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 2. Handle Asymmetric Keypair Generation
	if action == "gen-keypair" {
		var privBytes, pubBytes, certBytes []byte
		var err error

		switch strings.ToLower(*keypairType) {
		case "ecdsa":
			privBytes, pubBytes, err = flowcrypto.GenerateECDSAKeyPair("P-256")
		case "ed25519":
			privBytes, pubBytes, err = flowcrypto.GenerateEd25519KeyPair()
		case "pgp", "gpg", "openpgp":
			privBytes, pubBytes, err = flowcrypto.GenerateOpenPGPKeyPair(*cnFlag, "Flow Operator", "signer@flow.local")
		case "rsa", "":
			privBytes, pubBytes, err = flowcrypto.GenerateRSAKeyPair(*keyBits)
		default:
			fmt.Fprintf(os.Stderr, "Error: unsupported keypair type %q (expected rsa, ecdsa, ed25519, or pgp)\n", *keypairType)
			os.Exit(1)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating keypair: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(*outPrivFlag, privBytes, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving private key to %s: %v\n", *outPrivFlag, err)
			os.Exit(1)
		}
		if err := os.WriteFile(*outPubFlag, pubBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving public key to %s: %v\n", *outPubFlag, err)
			os.Exit(1)
		}
		fmt.Printf("Successfully generated %s keypair:\n  Private key: %s\n  Public key:  %s\n", *keypairType, *outPrivFlag, *outPubFlag)

		if *outCertFlag != "" {
			privKey, err := flowcrypto.ParsePrivateKeyPEM(privBytes)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing private key for certificate: %v\n", err)
				os.Exit(1)
			}
			pubKey, _, err := flowcrypto.ParsePublicKeyOrCertPEM(pubBytes)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing public key for certificate: %v\n", err)
				os.Exit(1)
			}
			certBytes, err = flowcrypto.GenerateSelfSignedCertificate(privKey, pubKey, *cnFlag, 365)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating self-signed certificate: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(*outCertFlag, certBytes, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving certificate to %s: %v\n", *outCertFlag, err)
				os.Exit(1)
			}
			fmt.Printf("  Certificate: %s\n", *outCertFlag)
		}
		return
	}

	// 3. Read Input
	inputBytes, err := readInput(*inFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input from %s: %v\n", *inFileFlag, err)
		os.Exit(1)
	}
	if len(inputBytes) == 0 {
		fmt.Fprintf(os.Stderr, "Error: input is empty\n")
		os.Exit(1)
	}

	// 4. Handle Digital Signing
	if action == "sign" {
		if effectivePrivKey == "" {
			fmt.Fprintf(os.Stderr, "Error: missing private key for signing. Specify -private-key (or -priv).\n")
			os.Exit(1)
		}
		privKeyBytes, err := os.ReadFile(effectivePrivKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading private key %s: %v\n", effectivePrivKey, err)
			os.Exit(1)
		}

		var certBytes []byte
		if *certFlag != "" {
			certBytes, err = os.ReadFile(*certFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading certificate %s: %v\n", *certFlag, err)
				os.Exit(1)
			}
		}

		sigBytes, format, err := flowcrypto.SignData(inputBytes, *sigTypeFlag, privKeyBytes, certBytes, *passphraseFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Signing failed: %v\n", err)
			os.Exit(1)
		}

		var finalOutput []byte
		if *wrapFlag {
			armored := flowcrypto.WrapSignedPayload(format, sigBytes, inputBytes)
			finalOutput = []byte(armored)
		} else if *binaryFlag {
			finalOutput = sigBytes
		} else {
			armored := flowcrypto.WrapSignatureOnly(format, sigBytes)
			finalOutput = []byte(armored + "\n")
		}

		if err := writeOutput(*outFileFlag, finalOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing signature output to %s: %v\n", *outFileFlag, err)
			os.Exit(1)
		}
		if *outFileFlag != "" && *outFileFlag != "-" {
			fmt.Printf("Successfully signed %s -> %s (format: %s)\n", *inFileFlag, *outFileFlag, format)
		}
		return
	}

	// 5. Handle Digital Signature Verification
	if action == "verify" {
		var keyOrCertBytes []byte
		if effectivePubKey != "" {
			keyOrCertBytes, err = os.ReadFile(effectivePubKey)
		} else if *certFlag != "" {
			keyOrCertBytes, err = os.ReadFile(*certFlag)
		} else if *caCertFlag != "" {
			keyOrCertBytes, err = os.ReadFile(*caCertFlag)
		} else if *keyringFlag != "" {
			keyOrCertBytes, err = os.ReadFile(*keyringFlag)
		} else {
			fmt.Fprintf(os.Stderr, "Error: verification requires -public-key, -cert, -ca-cert, or -keyring\n")
			os.Exit(1)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading verification key/cert: %v\n", err)
			os.Exit(1)
		}

		// Check if input is a unified FLOWSIGNED envelope
		if flowcrypto.IsSignedPayload(inputBytes) {
			content, res, err := flowcrypto.VerifyPayloadAuto(inputBytes, keyOrCertBytes)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Verification FAILED: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Signature VALID (%s via %s, Signer: %s)\n", res.Format, res.Algorithm, res.SignerInfo)
			if *outFileFlag != "" {
				if err := writeOutput(*outFileFlag, content); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing verified content to %s: %v\n", *outFileFlag, err)
					os.Exit(1)
				}
				if *outFileFlag != "-" {
					fmt.Printf("Extracted verified content to %s\n", *outFileFlag)
				}
			}
			return
		}

		// Otherwise, detached signature is expected
		if effectiveSigFile == "" {
			// Auto-check companion signature files (<input>.sig, <input>.asc, <input>.p7s)
			if *inFileFlag != "" && *inFileFlag != "-" {
				for _, ext := range []string{".sig", ".asc", ".p7s"} {
					if _, err := os.Stat(*inFileFlag + ext); err == nil {
						effectiveSigFile = *inFileFlag + ext
						break
					}
				}
			}
		}

		if effectiveSigFile == "" {
			fmt.Fprintf(os.Stderr, "Error: detached signature file not specified. Provide -signature (or -sig).\n")
			os.Exit(1)
		}

		sigBytes, err := os.ReadFile(effectiveSigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading signature file %s: %v\n", effectiveSigFile, err)
			os.Exit(1)
		}

		res, err := flowcrypto.VerifyAuto(inputBytes, sigBytes, keyOrCertBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Verification FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Signature VALID (%s via %s, Signer: %s)\n", res.Format, res.Algorithm, res.SignerInfo)
		return
	}

	// 6. Resolve Encryption Key (for Encrypt/Decrypt)
	effectiveKey := *keyFlag
	if effectiveKey == "" {
		effectiveKey = *keyAlias
	}
	effectiveKey = resolveSecureKey(effectiveKey)

	switch action {
	case "encrypt":
		if effectiveKey == "" {
			fmt.Fprintf(os.Stderr, "Error: missing encryption key. Specify -secure-key or set FLOW_SECURE_KEY environment variable.\n")
			os.Exit(1)
		}

		var outputBytes []byte
		if *binaryFlag {
			outputBytes, err = flowcrypto.EncryptBytes(inputBytes, effectiveKey)
		} else {
			var armored string
			armored, err = flowcrypto.EncryptArmored(inputBytes, effectiveKey)
			outputBytes = []byte(armored)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Encryption error: %v\n", err)
			os.Exit(1)
		}

		if err := writeOutput(*outFileFlag, outputBytes); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output to %s: %v\n", *outFileFlag, err)
			os.Exit(1)
		}

		if *outFileFlag != "" && *outFileFlag != "-" {
			fmt.Printf("Successfully encrypted %s -> %s\n", *inFileFlag, *outFileFlag)
		}

	case "decrypt":
		if effectiveKey == "" {
			fmt.Fprintf(os.Stderr, "Error: missing decryption key. Specify -secure-key or set FLOW_SECURE_KEY environment variable.\n")
			os.Exit(1)
		}

		trimmed := bytes.TrimSpace(inputBytes)
		var decrypted []byte
		if flowcrypto.IsEncryptedPayload(trimmed) {
			decrypted, err = flowcrypto.DecryptArmored(string(trimmed), effectiveKey)
		} else {
			decrypted, err = flowcrypto.DecryptBytes(trimmed, effectiveKey)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Decryption error: %v\n", err)
			os.Exit(1)
		}

		if err := writeOutput(*outFileFlag, decrypted); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output to %s: %v\n", *outFileFlag, err)
			os.Exit(1)
		}

		if *outFileFlag != "" && *outFileFlag != "-" {
			fmt.Printf("Successfully decrypted %s -> %s\n", *inFileFlag, *outFileFlag)
		}

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown action %q (supported: encrypt, decrypt, gen-key, sign, verify, gen-keypair)\n", action)
		os.Exit(1)
	}
}
