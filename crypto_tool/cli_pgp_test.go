package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCryptoCLIOpenPGPWrapped(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "private.asc")
	pub := filepath.Join(dir, "public.asc")
	input := filepath.Join(dir, "input.xml")
	wrapped := filepath.Join(dir, "signed.txt")
	output := filepath.Join(dir, "verified.xml")
	content := []byte("<pipeline><flow/></pipeline>")
	if err := os.WriteFile(input, content, 0600); err != nil {
		t.Fatal(err)
	}
	runCryptoCLI(t, "-action", "gen-keypair", "-keypair-type", "pgp", "-cn", "Test signer", "-out-priv", priv, "-out-pub", pub)
	runCryptoCLI(t, "-action", "sign", "-sig-type", "pgp", "-priv", priv, "-wrap", "-in", input, "-out", wrapped)
	runCryptoCLI(t, "-action", "verify", "-keyring", pub, "-in", wrapped, "-out", output)
	if got := readCryptoFile(t, output); !bytes.Equal(got, content) {
		t.Fatalf("verified = %q, want %q", got, content)
	}
}

func TestCryptoCLIStandardOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("example"), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := os.CreateTemp(dir, "stdout")
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = output
	t.Cleanup(func() { os.Stdout = previous; output.Close() })
	runCryptoCLI(t, "-action", "encrypt", "-key", "test-key", "-in", input, "-out", "-")
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = previous
	data := readCryptoFile(t, output.Name())
	if !bytes.HasPrefix(data, []byte("FLOWENC:v1:")) {
		t.Fatalf("unexpected stdout %q", data)
	}
}
