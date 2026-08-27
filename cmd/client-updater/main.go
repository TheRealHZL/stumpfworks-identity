package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/TheRealHZL/stumpfworks-identity/internal/clientupdate"
	"github.com/TheRealHZL/stumpfworks-identity/internal/version"
)

func main() {
	packageFile := flag.String("package", "", "local signed update ZIP")
	publicKeyFile := flag.String("public-key", "", "trusted Ed25519 public-key file")
	architecture := flag.String("architecture", "", "expected OS-architecture (defaults to current platform)")
	stageDir := flag.String("stage-dir", "", "optional new directory for verified payload staging")
	dryRun := flag.Bool("dry-run", false, "verify without installing")
	show := flag.Bool("version", false, "show version")
	flag.Parse()
	if *show {
		fmt.Println("sw-badge-client-updater", version.Version)
		return
	}
	if !*dryRun {
		fatal("installation is not implemented; --dry-run is required")
	}
	if *packageFile == "" || *publicKeyFile == "" {
		fatal("package and public key are required")
	}
	publicKey, err := clientupdate.LoadPublicKey(*publicKeyFile)
	if err != nil {
		fatal(err.Error())
	}
	result, err := clientupdate.VerifyPackage(*packageFile, publicKey, *architecture, version.Version)
	if err != nil {
		fatal(err.Error())
	}
	if *stageDir != "" {
		plan, err := clientupdate.VerifyAndStage(*packageFile, publicKey, *architecture, version.Version, *stageDir)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("staged directory=%s components=%d restart_lightdm=%t\n", *stageDir, len(plan.Components), requiresLightDM(plan))
	}
	fmt.Printf("verified release=%s channel=%s architecture=%s files=%d\n", result.Manifest.ReleaseVersion, result.Manifest.Channel, result.Manifest.Architecture, len(result.Files))
}

func requiresLightDM(plan clientupdate.StagePlan) bool {
	for _, component := range plan.Components {
		if component.RestartLightDM {
			return true
		}
	}
	return false
}

func fatal(message string) { fmt.Fprintln(os.Stderr, "client updater:", message); os.Exit(1) }
