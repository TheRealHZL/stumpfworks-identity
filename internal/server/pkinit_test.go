package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDynamicPKINITCertificate(t *testing.T) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test PKINIT CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	userKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "alice"}}, userKey)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		t.Fatal(err)
	}
	issuer := &PKINITIssuer{cert: ca, key: caKey, caPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})), realm: "EXAMPLE.TEST", life: 10 * time.Minute}
	certPEM, _, err := issuer.sign("alice", csr)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, caPath := filepath.Join(dir, "user.crt"), filepath.Join(dir, "ca.crt")
	if err = os.WriteFile(certPath, []byte(certPEM), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(caPath, []byte(issuer.caPEM), 0600); err != nil {
		t.Fatal(err)
	}
	if out, e := exec.Command("openssl", "verify", "-CAfile", caPath, certPath).CombinedOutput(); e != nil {
		t.Fatalf("openssl verify: %v: %s", e, out)
	}
	out, err := exec.Command("openssl", "x509", "-in", certPath, "-noout", "-ext", "subjectAltName").CombinedOutput()
	if err != nil {
		t.Fatalf("openssl x509: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "alice@example.test") {
		t.Fatalf("UPN SAN missing: %s", out)
	}
}
