package clientstatus

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheRealHZL/stumpfworks-identity/internal/clientupdate"
)

func TestSendRejectsPlainHTTP(t *testing.T) {
	err := Send(context.Background(), http.DefaultClient, "http://login01.test:8080", "secret", Report{})
	if err == nil {
		t.Fatal("plain HTTP was accepted")
	}
}

func TestCollectIncludesValidatedUpdateState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	want := clientupdate.UpdateState{Version: "1.2.0", Status: "success", UpdatedAt: time.Now().UTC(), RollbackAvailable: true}
	if err := clientupdate.WriteUpdateState(path, want); err != nil {
		t.Fatal(err)
	}
	report, err := CollectWithUpdateState(context.Background(), "client01", "1.2.0", path)
	if err != nil || report.Update == nil || report.Update.Version != want.Version || !report.Update.RollbackAvailable {
		t.Fatalf("unexpected report: %+v, %v", report, err)
	}
}

func TestReportContainsOnlyBoundedFields(t *testing.T) {
	r := Collect(context.Background(), "client01", "1.2.0-dev")
	for name, state := range map[string]string{"network": r.NetworkStatus, "ad": r.ADStatus, "camera": r.CameraStatus, "kerberos": r.KerberosStatus} {
		if state != "unknown" && state != "ok" && state != "degraded" && state != "unavailable" {
			t.Fatalf("%s returned invalid state %q", name, state)
		}
	}
}
