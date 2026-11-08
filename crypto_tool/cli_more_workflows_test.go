package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCryptoCLIPKCS7AndKeyFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.xml")
	priv := filepath.Join(dir, "private.pem")
	pub := filepath.Join(dir, "public.pem")
	cert := filepath.Join(dir, "cert.pem")
	sig := filepath.Join(dir, "signature.p7s")
	keyFile := filepath.Join(dir, "secret.key")
	ciphertext := filepath.Join(dir, "encrypted.txt")
	output := filepath.Join(dir, "output.xml")
	content := []byte("<pipeline><flow/></pipeline>")
	if err := os.WriteFile(input, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte("passphrase\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runCryptoCLI(t, "-action", "gen-keypair", "-keypair-type", "rsa", "-out-priv", priv, "-out-pub", pub, "-out-cert", cert)
	runCryptoCLI(t, "-action", "sign", "-sig-type", "pkcs7", "-cert", cert, "-private-key", priv, "-in", input, "-out", sig)
	runCryptoCLI(t, "-action", "verify", "-signature", sig, "-cert", cert, "-ca-cert", cert, "-in", input)
	runCryptoCLI(t, "-action", "encrypt", "-key-file", keyFile, "-in", input, "-out", ciphertext)
	if !strings.HasPrefix(string(readCryptoFile(t, ciphertext)), "FLOWENC:v1:") {
		t.Fatal("expected armored encrypted output")
	}
	runCryptoCLI(t, "-action", "decrypt", "-key-file", keyFile, "-in", ciphertext, "-out", output)
	if got := readCryptoFile(t, output); !bytes.Equal(got, content) {
		t.Fatalf("decrypted = %q, want %q", got, content)
	}
}

func TestCryptoCLIECDSAKeyPair(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "private.pem")
	pub := filepath.Join(dir, "public.pem")
	runCryptoCLI(t, "-action", "gen-keypair", "-keypair-type", "ecdsa", "-out-priv", priv, "-out-pub", pub)
	if !strings.Contains(string(readCryptoFile(t, priv)), "EC PRIVATE KEY") || !strings.Contains(string(readCryptoFile(t, pub)), "PUBLIC KEY") {
		t.Fatal("expected PEM-encoded ECDSA key pair")
	}
}
