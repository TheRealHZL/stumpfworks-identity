package clientupdate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func rollbackFixture(t *testing.T) (string, string, StagePlan) {
	t.Helper()
	root := t.TempDir()
	statusTarget := rootedTarget(root, componentSpecs["sw-badge-client-status"].Target)
	if err := os.MkdirAll(filepath.Dir(statusTarget), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statusTarget, []byte("old status"), 0755); err != nil {
		t.Fatal(err)
	}
	plan := StagePlan{ReleaseVersion: "1.2.1", Components: []StagedComponent{{Component: "sw-badge-client-status", Target: componentSpecs["sw-badge-client-status"].Target, Mode: 0755}, {Component: "sw-badge-client-diagnostics", Target: componentSpecs["sw-badge-client-diagnostics"].Target, Mode: 0755}}}
	rollbackDir := filepath.Join(t.TempDir(), "rollback")
	if _, err := CreateRollback(root, plan, rollbackDir, time.Now()); err != nil {
		t.Fatal(err)
	}
	return root, rollbackDir, plan
}

func TestRestoreRollbackRestoresAndRemoves(t *testing.T) {
	root, rollbackDir, _ := rollbackFixture(t)
	statusTarget := rootedTarget(root, componentSpecs["sw-badge-client-status"].Target)
	diagnosticsTarget := rootedTarget(root, componentSpecs["sw-badge-client-diagnostics"].Target)
	if err := os.WriteFile(statusTarget, []byte("new status"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(diagnosticsTarget), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diagnosticsTarget, []byte("new diagnostics"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := RestoreRollback(root, rollbackDir); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(statusTarget)
	if err != nil || string(restored) != "old status" {
		t.Fatalf("existing file not restored: %q, %v", restored, err)
	}
	if _, err = os.Stat(diagnosticsTarget); !os.IsNotExist(err) {
		t.Fatalf("new file was not removed: %v", err)
	}
}

func TestRestoreVerifiesEverythingBeforeChangingTargets(t *testing.T) {
	root, rollbackDir, _ := rollbackFixture(t)
	statusTarget := rootedTarget(root, componentSpecs["sw-badge-client-status"].Target)
	if err := os.WriteFile(statusTarget, []byte("installed status"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rollbackDir, "files", "sw-badge-client-status"), []byte("tampered backup"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RestoreRollback(root, rollbackDir); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
	current, _ := os.ReadFile(statusTarget)
	if string(current) != "installed status" {
		t.Fatalf("target changed before full validation: %q", current)
	}
}

func TestRestoreCanResumeAfterInjectedFailure(t *testing.T) {
	root, rollbackDir, _ := rollbackFixture(t)
	statusTarget := rootedTarget(root, componentSpecs["sw-badge-client-status"].Target)
	diagnosticsTarget := rootedTarget(root, componentSpecs["sw-badge-client-diagnostics"].Target)
	_ = os.WriteFile(statusTarget, []byte("new status"), 0755)
	_ = os.MkdirAll(filepath.Dir(diagnosticsTarget), 0755)
	_ = os.WriteFile(diagnosticsTarget, []byte("new diagnostics"), 0755)
	err := restoreRollback(root, rollbackDir, func(index int, _ RollbackFile) error {
		if index == 1 {
			return errors.New("injected failure")
		}
		return nil
	})
	if err == nil {
		t.Fatal("injected failure was ignored")
	}
	if err = RestoreRollback(root, rollbackDir); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(statusTarget)
	if string(restored) != "old status" {
		t.Fatal("resume did not restore existing file")
	}
	if _, err = os.Stat(diagnosticsTarget); !os.IsNotExist(err) {
		t.Fatal("resume did not remove new file")
	}
}
