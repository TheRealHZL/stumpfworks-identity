package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	app "github.com/TheRealHZL/stumpfworks-identity/internal/server"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func main() {
	server := flag.String("server", "https://login01.example.test:8080", "badge server")
	client := flag.String("client-id", "client01-greeter", "client ID")
	caFile := flag.String("ca-file", "/etc/stumpfworks-badge/stumpfworks-homelab-ca.crt", "HTTPS CA")
	grantFile := flag.String("grant-file", "/run/swbadge/login-grant", "one-time grant file")
	realm := flag.String("realm", "EXAMPLE.TEST", "Kerberos realm")
	flag.Parse()
	if err := run(*server, *client, *caFile, *grantFile, *realm); err != nil {
		fmt.Fprintln(os.Stderr, "swbadge PAM:", err)
		os.Exit(1)
	}
}

func run(server, client, caFile, grantFile, realm string) error {
	pamUser := strings.TrimSpace(os.Getenv("PAM_USER"))
	if !usernamePattern.MatchString(pamUser) {
		return errors.New("invalid PAM user")
	}
	raw, err := os.ReadFile(grantFile)
	if err != nil {
		return errors.New("no badge grant")
	}
	_ = os.Remove(grantFile)
	parts := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(parts) != 2 || parts[0] != pamUser || len(parts[1]) < 32 {
		return errors.New("badge grant user mismatch")
	}

	uidText, gidText, err := lookupIDs(pamUser)
	if err != nil {
		return err
	}
	uid64, err := strconv.ParseUint(uidText, 10, 32)
	if err != nil {
		return err
	}
	gid64, err := strconv.ParseUint(gidText, 10, 32)
	if err != nil {
		return err
	}

	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return err
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"StumpfWorks"}, OrganizationalUnit: []string{"Badge Login"}, CommonName: pamUser}}, key)
	if err != nil {
		return err
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	body, _ := json.Marshal(app.PKINITRequest{Grant: parts[1], ClientID: client, CSR: string(csrPEM)})
	hc, err := tlsClient(caFile)
	if err != nil {
		return err
	}
	resp, err := hc.Post(strings.TrimRight(server, "/")+"/api/v1/auth/pkinit", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("certificate request denied: %s", strings.TrimSpace(string(b)))
	}
	var issued app.PKINITResponse
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&issued); err != nil {
		return err
	}
	if issued.Username != pamUser {
		return errors.New("issued username mismatch")
	}

	tmp, err := os.MkdirTemp("/tmp", ".swbadge-pkinit-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	keyPath, certPath, anchorPath := filepath.Join(tmp, "user.key"), filepath.Join(tmp, "user.crt"), filepath.Join(tmp, "ca.crt")
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		return err
	}
	if err = os.WriteFile(certPath, []byte(issued.Certificate), 0600); err != nil {
		return err
	}
	if err = os.WriteFile(anchorPath, []byte(issued.CA), 0600); err != nil {
		return err
	}

	tmpCache := filepath.Join(tmp, "ccache")
	cmd := exec.Command("/usr/bin/kinit", "-C", "-c", "FILE:"+tmpCache,
		"-X", "X509_user_identity=FILE:"+certPath+","+keyPath,
		"-X", "X509_anchors=FILE:"+anchorPath, pamUser+"@"+strings.ToUpper(realm))
	if out, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("kinit failed: %w: %s", e, strings.TrimSpace(string(out)))
	}
	cache := "/tmp/krb5cc_" + uidText
	if err = os.Chown(tmpCache, int(uid64), int(gid64)); err != nil {
		return err
	}
	if err = os.Chmod(tmpCache, 0600); err != nil {
		return err
	}
	if err = os.Rename(tmpCache, cache); err != nil {
		return err
	}
	return nil
}

func lookupIDs(username string) (string, string, error) {
	out, err := exec.Command("/usr/bin/getent", "passwd", username).Output()
	if err != nil {
		return "", "", fmt.Errorf("domain user lookup failed: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(parts) < 4 || parts[0] != username {
		return "", "", errors.New("invalid domain user record")
	}
	return parts[2], parts[3], nil
}

func tlsClient(caFile string) (*http.Client, error) {
	pemCA, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if !roots.AppendCertsFromPEM(pemCA) {
		return nil, errors.New("HTTPS CA file contains no certificate")
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	return &http.Client{Timeout: 12 * time.Second, Transport: tr}, nil
}
