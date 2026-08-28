package clientupdate

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = nil
	command.Stderr = nil
	return command.Run()
}

func PostInstallHealthCheck(parent context.Context, plan StagePlan, runner CommandRunner) func() error {
	return serviceHealthCheck(parent, plan, runner, RequiresLightDMMaintenance(plan))
}

func PostRollbackRecovery(parent context.Context, plan StagePlan, runner CommandRunner) func() error {
	return serviceHealthCheck(parent, plan, runner, RequiresLightDMMaintenance(plan))
}

func serviceHealthCheck(parent context.Context, plan StagePlan, runner CommandRunner, restartLightDM bool) func() error {
	return func() error {
		if runner == nil {
			return errors.New("health-check command runner is required")
		}
		ctx, cancel := context.WithTimeout(parent, 30*time.Second)
		defer cancel()
		hasSystemdUnit, hasStatusComponent := false, false
		for _, component := range plan.Components {
			switch component.Component {
			case "swbadge-client-status.service", "swbadge-client-status.timer":
				hasSystemdUnit = true
			case "sw-badge-client-status":
				hasStatusComponent = true
			}
		}
		if hasSystemdUnit {
			if err := runner.Run(ctx, "systemctl", "daemon-reload"); err != nil {
				return fmt.Errorf("systemd daemon-reload: %w", err)
			}
		}
		if hasSystemdUnit || hasStatusComponent {
			if err := runner.Run(ctx, "systemctl", "start", "swbadge-client-status.service"); err != nil {
				return fmt.Errorf("client status report: %w", err)
			}
			if err := runner.Run(ctx, "systemctl", "is-active", "swbadge-client-status.timer"); err != nil {
				return fmt.Errorf("client status timer: %w", err)
			}
		}
		if restartLightDM {
			if err := runner.Run(ctx, "systemctl", "restart", "lightdm.service"); err != nil {
				return fmt.Errorf("LightDM restart: %w", err)
			}
			if err := runner.Run(ctx, "systemctl", "try-restart", "swbadge-vnc.service"); err != nil {
				return fmt.Errorf("VNC rebind: %w", err)
			}
		}
		if err := runner.Run(ctx, "systemctl", "is-active", "lightdm.service"); err != nil {
			return fmt.Errorf("LightDM health: %w", err)
		}
		return nil
	}
}
