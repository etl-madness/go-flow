package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/etl-madness/go-flow/pkg/flowcrypto"
)

func TestLoadResourceSecureWithEncryptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	encFile := filepath.Join(tmpDir, "pipeline.xml.enc")
	rawXML := "<pipeline id=\"encrypted-test\"><flow><log msg=\"secret content\"/></flow></pipeline>"
	key := "SecureKey-12345"

	armored, err := flowcrypto.EncryptArmored([]byte(rawXML), key)
	if err != nil {
		t.Fatalf("EncryptArmored failed: %v", err)
	}

	if err := os.WriteFile(encFile, []byte(armored), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ctx := context.Background()

	// 1. Successful decryption with key
	loaded, err := LoadResourceSecure(ctx, encFile, true, key)
	if err != nil {
		t.Fatalf("LoadResourceSecure failed: %v", err)
	}
	if string(loaded) != rawXML {
		t.Fatalf("expected %q, got %q", rawXML, string(loaded))
	}

	// 2. Auto-detection of encryption when encrypted=false
	loadedAuto, err := LoadResourceSecure(ctx, encFile, false, key)
	if err != nil {
		t.Fatalf("LoadResourceSecure auto-decrypt failed: %v", err)
	}
	if string(loadedAuto) != rawXML {
		t.Fatalf("expected %q, got %q", rawXML, string(loadedAuto))
	}

	// 3. Decryption fails with wrong key
	_, err = LoadResourceSecure(ctx, encFile, true, "WrongKey")
	if err == nil {
		t.Fatal("expected error with wrong key, got nil")
	}

	// 4. Missing key error
	_, err = LoadResourceSecure(ctx, encFile, true, "")
	if err == nil {
		t.Fatal("expected error with empty key, got nil")
	}
	if !strings.Contains(err.Error(), "no secure key was provided") {
		t.Fatalf("expected missing key message, got: %v", err)
	}
}

func TestLoadResourceSecureWithEnvKey(t *testing.T) {
	tmpDir := t.TempDir()
	encFile := filepath.Join(tmpDir, "config.xml.enc")
	rawXML := "<config><variables><variable name=\"env\" value=\"prod\"/></variables></config>"
	key := "EnvKeyABC-999"

	armored, err := flowcrypto.EncryptArmored([]byte(rawXML), key)
	if err != nil {
		t.Fatalf("EncryptArmored failed: %v", err)
	}
	if err := os.WriteFile(encFile, []byte(armored), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	os.Setenv("FLOW_SECURE_KEY", key)
	defer os.Unsetenv("FLOW_SECURE_KEY")

	ctx := context.Background()
	loaded, err := LoadResourceSecure(ctx, encFile, false, "")
	if err != nil {
		t.Fatalf("LoadResourceSecure with FLOW_SECURE_KEY failed: %v", err)
	}
	if string(loaded) != rawXML {
		t.Fatalf("expected %q, got %q", rawXML, string(loaded))
	}
}

func TestLoadResourceVerifiedWithDetachedSignature(t *testing.T) {
	tmpDir := t.TempDir()
	xmlFile := filepath.Join(tmpDir, "pipeline.xml")
	sigFile := filepath.Join(tmpDir, "pipeline.xml.sig")
	pubFile := filepath.Join(tmpDir, "public.pem")

	rawXML := "<pipeline id=\"signed-pipeline\"><flow><log msg=\"verified!\"/></flow></pipeline>"
	if err := os.WriteFile(xmlFile, []byte(rawXML), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	privPEM, pubPEM, err := flowcrypto.GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair failed: %v", err)
	}
	if err := os.WriteFile(pubFile, pubPEM, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	sigBytes, format, err := flowcrypto.SignData([]byte(rawXML), "openssl", privPEM, nil, "")
	if err != nil {
		t.Fatalf("SignData failed: %v", err)
	}
	armoredSig := flowcrypto.WrapSignatureOnly(format, sigBytes)
	if err := os.WriteFile(sigFile, []byte(armoredSig), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ctx := context.Background()

	// 1. Verify with explicit signature path
	loaded, verRes, err := LoadResourceVerified(ctx, xmlFile, SecurityOptions{
		VerifySignature: true,
		PublicKeyPath:   pubFile,
		SignaturePath:   sigFile,
	})
	if err != nil {
		t.Fatalf("LoadResourceVerified failed: %v", err)
	}
	if string(loaded) != rawXML {
		t.Fatalf("expected %q, got %q", rawXML, string(loaded))
	}
	if verRes == nil || !verRes.Valid {
		t.Fatal("expected valid verification result")
	}

	// 2. Auto-detect companion .sig file
	loadedAuto, verResAuto, err := LoadResourceVerified(ctx, xmlFile, SecurityOptions{
		PublicKeyPath: pubFile,
	})
	if err != nil {
		t.Fatalf("LoadResourceVerified auto-sig failed: %v", err)
	}
	if string(loadedAuto) != rawXML {
		t.Fatalf("expected %q, got %q", rawXML, string(loadedAuto))
	}
	if verResAuto == nil || !verResAuto.Valid {
		t.Fatal("expected valid auto-detected result")
	}

	// 3. Tampered data fails verification
	tamperedXML := "<pipeline id=\"hacked\"><flow/></pipeline>"
	if err := os.WriteFile(xmlFile, []byte(tamperedXML), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	_, _, err = LoadResourceVerified(ctx, xmlFile, SecurityOptions{
		PublicKeyPath: pubFile,
	})
	if err == nil {
		t.Fatal("expected verification failure for tampered file, got nil")
	}
}

func TestLoadResourceVerifiedWithUnifiedEnvelope(t *testing.T) {
	tmpDir := t.TempDir()
	envelopeFile := filepath.Join(tmpDir, "pipeline.xml.signed")
	pubFile := filepath.Join(tmpDir, "public.pem")

	rawXML := "<pipeline id=\"unified-envelope\"><flow><log msg=\"inside envelope\"/></flow></pipeline>"
	privPEM, pubPEM, err := flowcrypto.GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair failed: %v", err)
	}
	if err := os.WriteFile(pubFile, pubPEM, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	sigBytes, format, err := flowcrypto.SignData([]byte(rawXML), "openssl", privPEM, nil, "")
	if err != nil {
		t.Fatalf("SignData failed: %v", err)
	}

	wrapped := flowcrypto.WrapSignedPayload(format, sigBytes, []byte(rawXML))
	if err := os.WriteFile(envelopeFile, []byte(wrapped), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ctx := context.Background()
	loaded, verRes, err := LoadResourceVerified(ctx, envelopeFile, SecurityOptions{
		PublicKeyPath: pubFile,
	})
	if err != nil {
		t.Fatalf("LoadResourceVerified with unified envelope failed: %v", err)
	}
	if string(loaded) != rawXML {
		t.Fatalf("expected %q, got %q", rawXML, string(loaded))
	}
	if verRes == nil || !verRes.Valid {
		t.Fatal("expected valid verification result")
	}
}

func TestLoadResourceCombinedSignedAndEncrypted(t *testing.T) {
	tmpDir := t.TempDir()
	combinedFile := filepath.Join(tmpDir, "pipeline.xml.sec")
	pubFile := filepath.Join(tmpDir, "public.pem")
	key := "AESPassphraseCombined123"

	rawXML := "<pipeline id=\"both\"><flow><log msg=\"signed and encrypted\"/></flow></pipeline>"
	privPEM, pubPEM, err := flowcrypto.GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair failed: %v", err)
	}
	if err := os.WriteFile(pubFile, pubPEM, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 1. Encrypt first
	armored, err := flowcrypto.EncryptArmored([]byte(rawXML), key)
	if err != nil {
		t.Fatalf("EncryptArmored failed: %v", err)
	}

	// 2. Sign ciphertext
	sigBytes, format, err := flowcrypto.SignData([]byte(armored), "openssl", privPEM, nil, "")
	if err != nil {
		t.Fatalf("SignData failed: %v", err)
	}

	// 3. Wrap in unified signed envelope
	wrapped := flowcrypto.WrapSignedPayload(format, sigBytes, []byte(armored))
	if err := os.WriteFile(combinedFile, []byte(wrapped), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ctx := context.Background()
	loaded, verRes, err := LoadResourceVerified(ctx, combinedFile, SecurityOptions{
		Encrypted:     true,
		SecureKey:     key,
		PublicKeyPath: pubFile,
	})
	if err != nil {
		t.Fatalf("LoadResourceVerified combined failed: %v", err)
	}
	if string(loaded) != rawXML {
		t.Fatalf("expected %q, got %q", rawXML, string(loaded))
	}
	if verRes == nil || !verRes.Valid {
		t.Fatal("expected valid verification result")
	}
}

func TestLoadResourceVerifiedSignThenEncrypt(t *testing.T) {
	tmpDir := t.TempDir()
	combinedFile := filepath.Join(tmpDir, "pipeline.xml.enc")
	pubFile := filepath.Join(tmpDir, "public.pem")

	rawXML := `<pipeline><title>Confidential Signer</title></pipeline>`
	key := "SignThenEncryptKey2026!"

	privPEM, pubPEM, err := flowcrypto.GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair failed: %v", err)
	}
	if err := os.WriteFile(pubFile, pubPEM, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 1. Sign plaintext first
	sigBytes, format, err := flowcrypto.SignData([]byte(rawXML), "openssl", privPEM, nil, "")
	if err != nil {
		t.Fatalf("SignData failed: %v", err)
	}
	signedEnvelope := flowcrypto.WrapSignedPayload(format, sigBytes, []byte(rawXML))

	// 2. Encrypt signed envelope
	armored, err := flowcrypto.EncryptArmored([]byte(signedEnvelope), key)
	if err != nil {
		t.Fatalf("EncryptArmored failed: %v", err)
	}
	if err := os.WriteFile(combinedFile, []byte(armored), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ctx := context.Background()
	loaded, verRes, err := LoadResourceVerified(ctx, combinedFile, SecurityOptions{
		Encrypted:       true,
		SecureKey:       key,
		PublicKeyPath:   pubFile,
		VerifySignature: true,
	})
	if err != nil {
		t.Fatalf("LoadResourceVerified Sign-then-Encrypt failed: %v", err)
	}
	if string(loaded) != rawXML {
		t.Fatalf("expected %q, got %q", rawXML, string(loaded))
	}
	if verRes == nil || !verRes.Valid {
		t.Fatal("expected valid verification result")
	}
}

