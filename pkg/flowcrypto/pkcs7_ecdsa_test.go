package flowcrypto

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestPKCS7ECDSASignatureAndTrust(t *testing.T) {
	privatePEM, publicPEM, err := GenerateECDSAKeyPair("P-256")
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := ParsePrivateKeyPEM(privatePEM)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, _, err := ParsePublicKeyOrCertPEM(publicPEM)
	if err != nil {
		t.Fatal(err)
	}
	certPEM, err := GenerateSelfSignedCertificate(privateKey, publicKey, "ECDSA signer", 1)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("certificate PEM missing")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("signed ECDSA content")
	signature, err := SignPKCS7(data, cert, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyPKCS7(data, signature, cert)
	if err != nil || !cert.Equal(verified) {
		t.Fatalf("verification: certificate=%v error=%v", verified, err)
	}
	if _, err := VerifyPKCS7([]byte("tampered"), signature, cert); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("tampered data: %v", err)
	}
	if _, err := VerifyPKCS7WithCA(data, signature, nil, nil); err == nil {
		t.Fatal("signature accepted without a trust anchor")
	}
	if _, err := SignPKCS7(data, nil, privateKey); err == nil {
		t.Fatal("signing accepted a missing certificate")
	}
	if _, err := SignPKCS7(data, cert, nil); err == nil {
		t.Fatal("signing accepted a missing private key")
	}
	if _, err := VerifyPKCS7(data, []byte("not ASN.1"), cert); err == nil {
		t.Fatal("malformed signature was accepted")
	}
}
