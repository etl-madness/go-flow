package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/etl-madness/go-flow/pkg/flowcrypto"
)

func TestResolveSecureKeyPrecedence(t *testing.T) {
	os.Setenv("FLOW_SECURE_KEY", "EnvFlowKey")
	os.Setenv("SECURE_KEY", "EnvGenericKey")
	defer os.Unsetenv("FLOW_SECURE_KEY")
	defer os.Unsetenv("SECURE_KEY")

	// 1. Explicit CLI key wins
	if got := resolveSecureKey("ExplicitCLI"); got != "ExplicitCLI" {
		t.Fatalf("expected ExplicitCLI, got %s", got)
	}

	// 2. FLOW_SECURE_KEY takes precedence over SECURE_KEY
	if got := resolveSecureKey(""); got != "EnvFlowKey" {
		t.Fatalf("expected EnvFlowKey, got %s", got)
	}

	// 3. SECURE_KEY is fallback
	os.Unsetenv("FLOW_SECURE_KEY")
	if got := resolveSecureKey(""); got != "EnvGenericKey" {
		t.Fatalf("expected EnvGenericKey, got %s", got)
	}
}

func TestCryptoToolFileRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	plainFile := filepath.Join(tmpDir, "plain.xml")
	encFile := filepath.Join(tmpDir, "encrypted.enc")
	decFile := filepath.Join(tmpDir, "decrypted.xml")

	plainContent := "<pipeline><flow><log msg=\"roundtrip test\"/></flow></pipeline>"
	if err := os.WriteFile(plainFile, []byte(plainContent), 0644); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	key := "TestKeyXYZ_123"

	// 1. Encrypt
	plainBytes, err := readInput(plainFile)
	if err != nil {
		t.Fatalf("readInput failed: %v", err)
	}
	armored, err := flowcrypto.EncryptArmored(plainBytes, key)
	if err != nil {
		t.Fatalf("EncryptArmored failed: %v", err)
	}
	if err := writeOutput(encFile, []byte(armored)); err != nil {
		t.Fatalf("writeOutput failed: %v", err)
	}

	// Verify file is armored
	encBytes, err := os.ReadFile(encFile)
	if err != nil {
		t.Fatalf("failed reading encrypted file: %v", err)
	}
	if !strings.HasPrefix(string(encBytes), flowcrypto.ArmoredPrefixV1) {
		t.Fatalf("expected armored prefix, got: %s", string(encBytes))
	}

	// 2. Decrypt
	decryptedBytes, err := flowcrypto.DecryptAuto(encBytes, key, true)
	if err != nil {
		t.Fatalf("DecryptAuto failed: %v", err)
	}
	if err := writeOutput(decFile, decryptedBytes); err != nil {
		t.Fatalf("writeOutput failed: %v", err)
	}

	// 3. Verify exact match
	decContent, err := os.ReadFile(decFile)
	if err != nil {
		t.Fatalf("failed reading decrypted file: %v", err)
	}
	if string(decContent) != plainContent {
		t.Fatalf("expected %q, got %q", plainContent, string(decContent))
	}
}

func TestCryptoToolSignAndVerifyDetached(t *testing.T) {
	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "pipeline.xml")
	sigFile := filepath.Join(tmpDir, "pipeline.xml.sig")

	content := []byte("<pipeline id=\"signed\"><flow/></pipeline>")
	if err := os.WriteFile(dataFile, content, 0644); err != nil {
		t.Fatalf("failed writing data file: %v", err)
	}

	privPEM, pubPEM, err := flowcrypto.GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair failed: %v", err)
	}

	sigBytes, format, err := flowcrypto.SignData(content, "openssl", privPEM, nil, "")
	if err != nil {
		t.Fatalf("SignData failed: %v", err)
	}
	armoredSig := flowcrypto.WrapSignatureOnly(format, sigBytes)
	if err := os.WriteFile(sigFile, []byte(armoredSig), 0644); err != nil {
		t.Fatalf("failed writing signature file: %v", err)
	}

	// Verify
	loadedSig, err := os.ReadFile(sigFile)
	if err != nil {
		t.Fatalf("failed reading signature file: %v", err)
	}
	res, err := flowcrypto.VerifyAuto(content, loadedSig, pubPEM)
	if err != nil {
		t.Fatalf("VerifyAuto failed: %v", err)
	}
	if !res.Valid {
		t.Fatal("expected signature to be valid")
	}
}

func TestCryptoToolSignAndVerifyWrapped(t *testing.T) {
	content := []byte("<pipeline id=\"wrapped\"><flow/></pipeline>")
	privPEM, pubPEM, err := flowcrypto.GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair failed: %v", err)
	}

	sigBytes, format, err := flowcrypto.SignData(content, "openssl", privPEM, nil, "")
	if err != nil {
		t.Fatalf("SignData failed: %v", err)
	}

	wrapped := flowcrypto.WrapSignedPayload(format, sigBytes, content)
	unwrapped, res, err := flowcrypto.VerifyPayloadAuto([]byte(wrapped), pubPEM)
	if err != nil {
		t.Fatalf("VerifyPayloadAuto failed: %v", err)
	}
	if string(unwrapped) != string(content) {
		t.Fatalf("expected unwrapped %q, got %q", string(content), string(unwrapped))
	}
	if !res.Valid {
		t.Fatal("expected valid result")
	}
}

func TestBinaryCiphertextWithWhitespaceBytes(t *testing.T) {
	key := "RandomBinaryTestKey2026!"
	// Test with arbitrary binary data
	plain := []byte{0x20, 0x0A, 0x00, 0xFF, 0x3C, 0x78, 0x6D, 0x6C, 0x0D, 0x20}

	enc, err := flowcrypto.EncryptBytes(plain, key)
	if err != nil {
		t.Fatalf("EncryptBytes failed: %v", err)
	}

	// DecryptAuto in requireEncrypted=true mode
	dec, err := flowcrypto.DecryptAuto(enc, key, true)
	if err != nil {
		t.Fatalf("DecryptAuto failed: %v", err)
	}

	if !bytes.Equal(plain, dec) {
		t.Fatalf("expected %v, got %v", plain, dec)
	}
}

