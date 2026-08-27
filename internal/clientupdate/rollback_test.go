package clientupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateRollbackRecordsExistingAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	existingTarget := filepath.Join(root, "usr", "local", "libexec", "sw-badge-client-status")
	if err := os.MkdirAll(filepath.Dir(existingTarget), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingTarget, []byte("old status binary"), 0755); err != nil {
		t.Fatal(err)
	}
	plan := StagePlan{ReleaseVersion: "1.2.1", Components: []StagedComponent{{Component: "sw-badge-client-status", Target: componentSpecs["sw-badge-client-status"].Target, Mode: 0755}, {Component: "sw-badge-client-diagnostics", Target: componentSpecs["sw-badge-client-diagnostics"].Target, Mode: 0755}}}
	rollbackDir := filepath.Join(t.TempDir(), "rollback")
	manifest, err := CreateRollback(root, plan, rollbackDir, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 2 || !manifest.Files[0].Existed || manifest.Files[1].Existed {
		t.Fatalf("unexpected rollback manifest: %+v", manifest)
	}
	backup, err := os.ReadFile(filepath.Join(rollbackDir, "files", "sw-badge-client-status"))
	if err != nil || string(backup) != "old status binary" {
		t.Fatalf("unexpected backup: %q, %v", backup, err)
	}
}

func TestCreateRollbackRejectsSymlinkAndInvalidPlan(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "usr", "local", "libexec", "sw-badge-client-status")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(root, "real")
	if err := os.WriteFile(real, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, target); err == nil {
		plan := StagePlan{ReleaseVersion: "1.2.1", Components: []StagedComponent{{Component: "sw-badge-client-status", Target: componentSpecs["sw-badge-client-status"].Target, Mode: 0755}}}
		if _, err = CreateRollback(root, plan, filepath.Join(t.TempDir(), "rollback"), time.Now()); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("unexpected symlink error: %v", err)
		}
	}
	bad := StagePlan{Components: []StagedComponent{{Component: "sw-badge-client-status", Target: "/tmp/escape", Mode: 0755}}}
	if _, err := CreateRollback(root, bad, filepath.Join(t.TempDir(), "bad"), time.Now()); err == nil || !strings.Contains(err.Error(), "invalid staged") {
		t.Fatalf("unexpected plan error: %v", err)
	}
}
