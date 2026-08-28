package clientupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVerifyAndStage(t *testing.T) {
	payload := []byte("verified staged binary")
	packagePath, key := testPackage(t, payload, nil, "")
	stageDir := filepath.Join(t.TempDir(), "stage")
	plan, err := VerifyAndStage(packagePath, key, "linux-amd64", "1.2.0-dev", stageDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Components) != 1 || plan.Components[0].Target != "/usr/local/libexec/sw-badge-client-status" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	staged, err := os.ReadFile(filepath.Join(stageDir, "payload", "sw-badge-client-status"))
	if err != nil {
		t.Fatal(err)
	}
	if string(staged) != string(payload) {
		t.Fatal("staged payload differs")
	}
	info, err := os.Stat(filepath.Join(stageDir, "stage-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("plan mode is %o", info.Mode().Perm())
	}
}

func TestVerifyAndStageRefusesExistingDirectory(t *testing.T) {
	packagePath, key := testPackage(t, []byte("binary"), nil, "")
	stageDir := filepath.Join(t.TempDir(), "stage")
	if err := os.Mkdir(stageDir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyAndStage(packagePath, key, "linux-amd64", "1.2.0-dev", stageDir); err == nil {
		t.Fatal("existing stage directory was overwritten")
	}
}
