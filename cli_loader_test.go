package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/etl-madness/go-flow/pkg/flowcrypto"
)

func TestApplyXMLOptionsSecureEncrypted(t *testing.T) {
	tmpDir := t.TempDir()
	optFile := filepath.Join(tmpDir, "options.xml.enc")
	key := "OptKey-789"

	optionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<flow_cli_options>
    <options>
        <format>jsonpretty</format>
        <validate>true</validate>
    </options>
</flow_cli_options>`

	armored, err := flowcrypto.EncryptArmored([]byte(optionsXML), key)
	if err != nil {
		t.Fatalf("EncryptArmored failed: %v", err)
	}

	if err := os.WriteFile(optFile, []byte(armored), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Make sure target flags exist in flag.CommandLine
	if flag.Lookup("format") == nil {
		flag.String("format", "csv", "format")
	}
	if flag.Lookup("validate") == nil {
		flag.Bool("validate", false, "validate")
	}

	err = applyXMLOptionsSecure(optFile, true, key)
	if err != nil {
		t.Fatalf("applyXMLOptionsSecure failed: %v", err)
	}

	if flag.Lookup("format").Value.String() != "jsonpretty" {
		t.Fatalf("expected format=jsonpretty, got: %s", flag.Lookup("format").Value.String())
	}
	if flag.Lookup("validate").Value.String() != "true" {
		t.Fatalf("expected validate=true, got: %s", flag.Lookup("validate").Value.String())
	}
}

func TestApplyXMLOptionsVerifiedSigned(t *testing.T) {
	tmpDir := t.TempDir()
	optFile := filepath.Join(tmpDir, "options.xml.signed")
	pubFile := filepath.Join(tmpDir, "public.pem")

	optionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<flow_cli_options>
    <options>
        <debug>true</debug>
    </options>
</flow_cli_options>`

	privPEM, pubPEM, err := flowcrypto.GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKeyPair failed: %v", err)
	}
	if err := os.WriteFile(pubFile, pubPEM, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	sigBytes, format, err := flowcrypto.SignData([]byte(optionsXML), "openssl", privPEM, nil, "")
	if err != nil {
		t.Fatalf("SignData failed: %v", err)
	}

	wrapped := flowcrypto.WrapSignedPayload(format, sigBytes, []byte(optionsXML))
	if err := os.WriteFile(optFile, []byte(wrapped), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if flag.Lookup("debug") == nil {
		flag.Bool("debug", false, "debug")
	}

	err = applyXMLOptionsVerified(optFile, SecurityOptions{
		PublicKeyPath: pubFile,
	})
	if err != nil {
		t.Fatalf("applyXMLOptionsVerified failed: %v", err)
	}

	if flag.Lookup("debug").Value.String() != "true" {
		t.Fatalf("expected debug=true, got %s", flag.Lookup("debug").Value.String())
	}
}

