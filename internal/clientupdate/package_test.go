package clientupdate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateKeyAndCreateSignedPackage(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "release.key"), filepath.Join(dir, "release.pub")
	if err := GenerateSigningKey(privatePath, publicPath); err != nil {
		t.Fatal(err)
	}
	if err := GenerateSigningKey(privatePath, publicPath); err == nil {
		t.Fatal("existing keys were overwritten")
	}
	privateKey, err := LoadPrivateKey(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := LoadPublicKey(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	component := filepath.Join(dir, "status")
	if err = os.WriteFile(component, []byte("status binary"), 0755); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(dir, "update.zip")
	options := PackageOptions{ReleaseVersion: "1.2.1", Channel: "development", Architecture: "linux-amd64", MinimumVersion: "1.2.0-dev", CreatedAt: time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC), Components: map[string]string{"sw-badge-client-status": component}}
	if err = CreateSignedPackage(packagePath, privateKey, options); err != nil {
		t.Fatal(err)
	}
	result, err := VerifyPackage(packagePath, publicKey, "linux-amd64", "1.2.0-dev")
	if err != nil || result.Manifest.ReleaseVersion != "1.2.1" {
		t.Fatalf("package verification failed: %+v, %v", result, err)
	}
	if err = CreateSignedPackage(packagePath, privateKey, options); err == nil {
		t.Fatal("existing package was overwritten")
	}
}
