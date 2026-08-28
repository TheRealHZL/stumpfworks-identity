package clientupdate

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls  []string
	failAt string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if call == r.failAt {
		return errors.New("injected command failure")
	}
	return nil
}

func TestPostInstallHealthCheckSelectsRequiredCommands(t *testing.T) {
	plan := StagePlan{Components: []StagedComponent{{Component: "sw-badge-client-status"}, {Component: "swbadge-client-status.timer"}}}
	runner := &fakeRunner{}
	if err := PostInstallHealthCheck(context.Background(), plan, runner)(); err != nil {
		t.Fatal(err)
	}
	want := []string{"systemctl daemon-reload", "systemctl start swbadge-client-status.service", "systemctl is-active swbadge-client-status.timer", "systemctl is-active lightdm.service"}
	if strings.Join(runner.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected commands: %v", runner.calls)
	}
}

func TestPostInstallHealthCheckPropagatesFailure(t *testing.T) {
	runner := &fakeRunner{failAt: "systemctl is-active swbadge-client-status.timer"}
	plan := StagePlan{Components: []StagedComponent{{Component: "sw-badge-client-status"}}}
	err := PostInstallHealthCheck(context.Background(), plan, runner)()
	if err == nil || !strings.Contains(err.Error(), "timer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGreeterPlanRequiresMaintenanceWindow(t *testing.T) {
	plan := StagePlan{Components: []StagedComponent{{Component: "sw-badge-native-greeter", RestartLightDM: true}}}
	if !RequiresLightDMMaintenance(plan) {
		t.Fatal("greeter update did not require maintenance")
	}
}

func TestGreeterHealthRestartsLightDM(t *testing.T) {
	plan := StagePlan{Components: []StagedComponent{{Component: "sw-badge-native-greeter", RestartLightDM: true}}}
	runner := &fakeRunner{}
	if err := PostInstallHealthCheck(context.Background(), plan, runner)(); err != nil {
		t.Fatal(err)
	}
	want := []string{"systemctl restart lightdm.service", "systemctl try-restart swbadge-vnc.service", "systemctl is-active lightdm.service"}
	if strings.Join(runner.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected commands: %v", runner.calls)
	}
}
