package clientupdate

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPackage(t *testing.T, payload []byte, mutate func(*Manifest, *[]byte), extra string) (string, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	manifest := Manifest{SchemaVersion: 1, ReleaseVersion: "1.2.1", Channel: "development", Architecture: "linux-amd64", MinimumVersion: "1.2.0-dev", CreatedAt: time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC), Files: []File{{Component: "sw-badge-client-status", Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}}}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(&manifest, &manifestBytes)
	}
	signature := ed25519.Sign(privateKey, manifestBytes)
	path := filepath.Join(t.TempDir(), "update.zip")
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(output)
	for name, data := range map[string][]byte{"manifest.json": manifestBytes, "manifest.sig": []byte(base64.StdEncoding.EncodeToString(signature)), "payload/sw-badge-client-status": payload} {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if extra != "" {
		writer, err := archive.Create(extra)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = writer.Write([]byte("unexpected"))
	}
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err = output.Close(); err != nil {
		t.Fatal(err)
	}
	return path, publicKey
}

func TestVerifyPackage(t *testing.T) {
	path, key := testPackage(t, []byte("verified binary"), nil, "")
	result, err := VerifyPackage(path, key, "linux-amd64", "1.2.0-dev")
	if err != nil || result.Manifest.ReleaseVersion != "1.2.1" || len(result.Files) != 1 {
		t.Fatalf("verification failed: %+v, %v", result, err)
	}
}

func TestVerifyRejectsWrongArchitecture(t *testing.T) {
	path, key := testPackage(t, []byte("binary"), nil, "")
	if _, err := VerifyPackage(path, key, "linux-arm64", "1.2.0-dev"); err == nil || !strings.Contains(err.Error(), "architecture") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	path, key := testPackage(t, []byte("tampered"), func(manifest *Manifest, raw *[]byte) {
		manifest.Files[0].SHA256 = strings.Repeat("0", 64)
		*raw, _ = json.Marshal(manifest)
	}, "")
	if _, err := VerifyPackage(path, key, "linux-amd64", "1.2.0-dev"); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRejectsWrongSignatureAndUnlistedEntry(t *testing.T) {
	path, _ := testPackage(t, []byte("binary"), nil, "")
	wrong, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := VerifyPackage(path, wrong, "linux-amd64", "1.2.0-dev"); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("unexpected signature error: %v", err)
	}
	path, key := testPackage(t, []byte("binary"), nil, "payload/../../escape")
	if _, err := VerifyPackage(path, key, "linux-amd64", "1.2.0-dev"); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unexpected path error: %v", err)
	}
}

func TestLoadPublicKey(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	path := filepath.Join(t.TempDir(), "release.pub")
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(publicKey)), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPublicKey(path)
	if err != nil || !publicKey.Equal(loaded) {
		t.Fatalf("public key load failed: %v", err)
	}
}

func TestVerifyRejectsIncompatibleClientVersion(t *testing.T) {
	path, key := testPackage(t, []byte("binary"), func(manifest *Manifest, raw *[]byte) {
		manifest.MinimumVersion = "1.3.0"
		manifest.ReleaseVersion = "1.4.0"
		*raw, _ = json.Marshal(manifest)
	}, "")
	if _, err := VerifyPackage(path, key, "linux-amd64", "1.2.0"); err == nil || !strings.Contains(err.Error(), "below minimum") {
		t.Fatalf("unexpected compatibility error: %v", err)
	}
}

func TestSemanticVersionValidation(t *testing.T) {
	for _, valid := range []string{"1.2.0", "1.2.0-dev", "v1.2.0-rc.10+build.4"} {
		if !validVersion(valid) {
			t.Errorf("valid version rejected: %s", valid)
		}
	}
	for _, invalid := range []string{"1.2", "01.2.0", "1.2.0-", "1.2.0+", "1.2.x"} {
		if validVersion(invalid) {
			t.Errorf("invalid version accepted: %s", invalid)
		}
	}
}
