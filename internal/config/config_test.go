package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClientTargetVersionConfigAndEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("updates:\n  client_target_version: 1.2.1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.ClientTargetVersion != "1.2.1" {
		t.Fatalf("unexpected config: %+v, %v", loaded, err)
	}
	t.Setenv("SWBADGE_CLIENT_TARGET_VERSION", "1.2.2")
	loaded, err = Load(path)
	if err != nil || loaded.ClientTargetVersion != "1.2.2" {
		t.Fatalf("environment did not override config: %+v, %v", loaded, err)
	}
}
