package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func runCryptoCLI(t *testing.T, args ...string) {
	t.Helper()
	oldFlags, oldArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })
	flag.CommandLine = flag.NewFlagSet("crypto-tool", flag.ContinueOnError)
	os.Args = append([]string{"crypto-tool"}, args...)
	main()
}

func readCryptoFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCryptoCLIEncryptionRoundTrips(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.xml")
	plain := []byte("<pipeline><flow/></pipeline>")
	if err := os.WriteFile(input, plain, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLOW_SECURE_KEY", "test-passphrase")
	for _, binary := range []bool{false, true} {
		t.Run(map[bool]string{false: "armored", true: "binary"}[binary], func(t *testing.T) {
			encrypted := filepath.Join(dir, filepath.Base(t.Name())+".enc")
			decrypted := filepath.Join(dir, filepath.Base(t.Name())+".xml")
			args := []string{"-action", "encrypt", "-in", input, "-out", encrypted}
			if binary {
				args = append(args, "-binary")
			}
			runCryptoCLI(t, args...)
			args = []string{"-action", "decrypt", "-in", encrypted, "-out", decrypted}
			if binary {
				args = append(args, "-binary")
			}
			runCryptoCLI(t, args...)
			if got := readCryptoFile(t, decrypted); !bytes.Equal(got, plain) {
				t.Fatalf("decrypted = %q, want %q", got, plain)
			}
		})
	}
}

func TestCryptoCLISigningWorkflows(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.xml")
	priv, pub, cert := filepath.Join(dir, "private.pem"), filepath.Join(dir, "public.pem"), filepath.Join(dir, "cert.pem")
	content := []byte("<pipeline><flow/></pipeline>")
	if err := os.WriteFile(input, content, 0600); err != nil {
		t.Fatal(err)
	}
	runCryptoCLI(t, "-action", "gen-keypair", "-keypair-type", "ed25519", "-out-priv", priv, "-out-pub", pub, "-out-cert", cert)
	if len(readCryptoFile(t, cert)) == 0 {
		t.Fatal("generated certificate is empty")
	}
	sig := input + ".sig"
	runCryptoCLI(t, "-action", "sign", "-in", input, "-out", sig, "-priv", priv)
	runCryptoCLI(t, "-action", "verify", "-in", input, "-pub", pub)
	wrapped := filepath.Join(dir, "signed.txt")
	extracted := filepath.Join(dir, "extracted.xml")
	runCryptoCLI(t, "-action", "sign", "-in", input, "-out", wrapped, "-private-key", priv, "-wrap")
	runCryptoCLI(t, "-action", "verify", "-in", wrapped, "-public-key", pub, "-out", extracted)
	if got := readCryptoFile(t, extracted); !bytes.Equal(got, content) {
		t.Fatalf("extracted = %q, want %q", got, content)
	}
	key := filepath.Join(dir, "secret.key")
	runCryptoCLI(t, "-gen-key", "-out", key)
	if len(bytes.TrimSpace(readCryptoFile(t, key))) == 0 {
		t.Fatal("generated symmetric key is empty")
	}
}
