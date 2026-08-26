package clientstatus

import (
	"context"
	"net/http"
	"testing"
)

func TestSendRejectsPlainHTTP(t *testing.T) {
	err := Send(context.Background(), http.DefaultClient, "http://login01.test:8080", "secret", Report{})
	if err == nil {
		t.Fatal("plain HTTP was accepted")
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
