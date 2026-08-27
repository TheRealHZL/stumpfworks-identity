package clientdiagnostics

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestArchiveExcludesConfiguredSecretsAndAddresses(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	caPath := filepath.Join(dir, "ca.crt")
	secret := "top-secret-client-token"
	if err := os.WriteFile(tokenPath, []byte(secret), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caPath, []byte("certificate-placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	report := Collect(context.Background(), Options{ClientID: "client01", Version: "1.2.0-dev", ServerURL: "https://192.0.2.15:8080", CAFile: caPath, TokenFile: tokenPath}, time.Unix(100, 0))
	output := filepath.Join(dir, "diagnostics.zip")
	if err := WriteArchive(output, report); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(output)
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("archive permissions: %v, %v", info, err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var contents strings.Builder
	for _, file := range reader.File {
		opened, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(opened)
		opened.Close()
		if err != nil {
			t.Fatal(err)
		}
		contents.Write(data)
	}
	for _, forbidden := range []string{secret, "192.0.2.15", tokenPath, caPath} {
		if strings.Contains(contents.String(), forbidden) {
			t.Fatalf("archive leaked %q", forbidden)
		}
	}
	if !strings.Contains(contents.String(), `"server_https": true`) || !strings.Contains(contents.String(), `"token_file_secured":`) {
		t.Fatal("bounded configuration checks missing")
	}
}
