package clientdiagnostics

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/TheRealHZL/stumpfworks-identity/internal/clientstatus"
)

type Options struct {
	ClientID, Version, ServerURL, CAFile, TokenFile string
}

type Report struct {
	SchemaVersion int                 `json:"schema_version"`
	GeneratedAt   time.Time           `json:"generated_at"`
	ClientID      string              `json:"client_id"`
	Version       string              `json:"version"`
	OS            string              `json:"os"`
	Architecture  string              `json:"architecture"`
	ServerHTTPS   bool                `json:"server_https"`
	CAReadable    bool                `json:"ca_readable"`
	TokenSecured  bool                `json:"token_file_secured"`
	Checks        clientstatus.Report `json:"checks"`
	Services      map[string]string   `json:"services"`
}

func Collect(ctx context.Context, options Options, now time.Time) Report {
	parsed, err := url.Parse(options.ServerURL)
	return Report{SchemaVersion: 1, GeneratedAt: now.UTC(), ClientID: options.ClientID, Version: options.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH, ServerHTTPS: err == nil && parsed.Scheme == "https" && parsed.Host != "", CAReadable: readableRegular(options.CAFile), TokenSecured: securedTokenFile(options.TokenFile), Checks: clientstatus.Collect(ctx, options.ClientID, options.Version), Services: map[string]string{"lightdm": serviceState(ctx, "lightdm.service"), "client_status_timer": serviceState(ctx, "swbadge-client-status.timer")}}
}

func readableRegular(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
func securedTokenFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm() == 0600
}

func serviceState(parent context.Context, unit string) string {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return "unavailable"
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "is-active", unit).Output()
	state := strings.TrimSpace(string(out))
	for _, allowed := range []string{"active", "inactive", "activating", "deactivating", "failed"} {
		if state == allowed {
			return state
		}
	}
	if err != nil {
		return "unknown"
	}
	return "unknown"
}

func WriteArchive(path string, report Report) error {
	if path == "" {
		return errors.New("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	cleanup := func() { file.Close(); os.Remove(temporary) }
	archive := zip.NewWriter(file)
	header := &zip.FileHeader{Name: "diagnostics.json", Method: zip.Deflate}
	header.SetMode(0600)
	entry, err := archive.CreateHeader(header)
	if err != nil {
		cleanup()
		return err
	}
	encoder := json.NewEncoder(entry)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(report); err != nil {
		cleanup()
		return err
	}
	readme, err := archive.Create("README.txt")
	if err != nil {
		cleanup()
		return err
	}
	_, err = readme.Write([]byte("This archive contains bounded StumpfWorks Identity readiness states. It excludes IP addresses, SSIDs, usernames, logs, credentials, tokens, certificate contents, and Kerberos principals.\n"))
	if err != nil {
		cleanup()
		return err
	}
	if err = archive.Close(); err != nil {
		cleanup()
		return err
	}
	if err = file.Close(); err != nil {
		os.Remove(temporary)
		return err
	}
	if err = os.Chmod(temporary, 0600); err != nil {
		os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}
