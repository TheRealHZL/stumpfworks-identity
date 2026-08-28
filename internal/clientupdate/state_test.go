package clientupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestWriteAndReadUpdateState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "update.json")
	want := UpdateState{Version: "1.2.1", Status: "success", UpdatedAt: time.Unix(100, 0).UTC(), RollbackAvailable: true}
	if err := WriteUpdateState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadUpdateState(path)
	if err != nil || got == nil || got.Version != want.Version || got.Status != want.Status || !got.UpdatedAt.Equal(want.UpdatedAt) || !got.RollbackAvailable {
		t.Fatalf("unexpected state: %+v, %v", got, err)
	}
	if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0644 {
		t.Fatalf("unexpected state mode: %v, %v", info, err)
	}
}
