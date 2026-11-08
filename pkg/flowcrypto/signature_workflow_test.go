package flowcrypto

import (
	"bytes"
	"encoding/pem"
	"testing"
)

func TestSignDataFormatsAndVerification(t *testing.T) {
	content := []byte("a signed payload")
	priv, pub, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	key, err := ParsePrivateKeyPEM(priv)
	if err != nil {
		t.Fatal(err)
	}
	public, _, err := ParsePublicKeyOrCertPEM(pub)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := GenerateSelfSignedCertificate(key, public, "test signer", 1)
	if err != nil {
		t.Fatal(err)
	}
	sig, format, err := SignData(content, "openssl", priv, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := VerifyAuto(content, []byte(WrapSignatureOnly(format, sig)), pub)
	if err != nil || !result.Valid {
		t.Fatalf("detached signature: %+v, %v", result, err)
	}
	if _, err := VerifyAuto([]byte("tampered"), []byte(WrapSignatureOnly(format, sig)), pub); err == nil {
		t.Fatal("tampered payload was accepted")
	}
	payload := []byte(WrapSignedPayload(format, sig, content))
	actual, result, err := VerifyPayloadAuto(payload, cert)
	if err != nil || !result.Valid || !bytes.Equal(actual, content) {
		t.Fatalf("signed envelope: %q %+v %v", actual, result, err)
	}
	if !IsSignaturePayload([]byte(WrapSignatureOnly(format, sig))) {
		t.Fatal("signature envelope not recognized")
	}
	for _, invalid := range []struct {
		kind string
		key  []byte
		cert []byte
	}{
		{"unknown", priv, nil}, {"openssl", []byte("invalid"), nil},
		{"pgp", []byte("invalid"), nil}, {"pkcs7", priv, nil}, {"pkcs7", priv, []byte("invalid")},
	} {
		if _, _, err := SignData(content, invalid.kind, invalid.key, invalid.cert, ""); err == nil {
			t.Errorf("SignData(%s) accepted invalid input", invalid.kind)
		}
	}
}

func TestSignatureAndKeyParsingErrors(t *testing.T) {
	for _, payload := range []string{
		"not signed", "FLOWSIGNED:v1:", "FLOWSIGNED:v1:OPENSSL:!:YQ==", "FLOWSIGNED:v1:OPENSSL:YQ==:!",
	} {
		if _, _, _, err := ParseSignedPayload([]byte(payload)); err == nil {
			t.Errorf("accepted malformed envelope %q", payload)
		}

	}
	for _, block := range []string{"CERTIFICATE", "PUBLIC KEY", "RSA PUBLIC KEY", "UNKNOWN"} {
		data := pem.EncodeToMemory(&pem.Block{Type: block, Bytes: []byte("invalid")})
		if _, _, err := ParsePublicKeyOrCertPEM(data); err == nil {
			t.Errorf("accepted invalid %s", block)
		}
		if _, err := ParsePrivateKeyPEM(data); err == nil {
			t.Errorf("accepted invalid private key %s", block)
		}
	}
	if _, err := DecryptArmored(ArmoredPrefixV1+"!", "key"); err == nil {
		t.Fatal("accepted invalid armored ciphertext")
	}
	if _, err := DecryptAuto([]byte(ArmoredPrefixV1+"AAAA"), "", false); err == nil {
		t.Fatal("accepted encrypted payload without key")
	}
}

func TestSignDataPGPAndPKCS7(t *testing.T) {
	content := []byte("signed data")
	pgpPriv, pgpPub, err := GenerateOpenPGPKeyPair("Test signer", "", "signer@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	pgpSig, format, err := SignData(content, "pgp", pgpPriv, nil, "")
	if err != nil || format != "PGP" {
		t.Fatalf("PGP signing: format=%q err=%v", format, err)
	}
	pgpResult, err := VerifyAuto(content, pgpSig, pgpPub)
	if err != nil || !pgpResult.Valid {
		t.Fatalf("PGP verification: %+v %v", pgpResult, err)
	}
	priv, pub, err := GenerateRSAKeyPair(2048)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := ParsePrivateKeyPEM(priv)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, _, err := ParsePublicKeyOrCertPEM(pub)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := GenerateSelfSignedCertificate(privateKey, publicKey, "test signer", 1)
	if err != nil {
		t.Fatal(err)
	}
	sig, format, err := SignData(content, "pkcs7", priv, cert, "")
	if err != nil || format != "PKCS7" {
		t.Fatalf("PKCS7 signing: format=%q err=%v", format, err)
	}
	result, err := VerifyAutoWithCA(content, []byte(WrapSignatureOnly(format, sig)), cert, cert)
	if err != nil || !result.Valid {
		t.Fatalf("PKCS7 verification: %+v %v", result, err)
	}
}
