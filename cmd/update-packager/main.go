package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/TheRealHZL/stumpfworks-identity/internal/clientupdate"
)

type components map[string]string

func (c components) String() string { return "name=path" }
func (c components) Set(value string) error {
	name, path, ok := strings.Cut(value, "=")
	if !ok || name == "" || path == "" {
		return fmt.Errorf("component must use name=path")
	}
	if _, exists := c[name]; exists {
		return fmt.Errorf("duplicate component %q", name)
	}
	c[name] = path
	return nil
}

func main() {
	generateKey := flag.Bool("generate-key", false, "generate a new Ed25519 signing key pair")
	privateKeyPath := flag.String("private-key", "", "private signing key path")
	publicKeyPath := flag.String("public-key", "", "public verification key path for key generation")
	output := flag.String("output", "", "new signed update ZIP")
	release := flag.String("release", "", "release version")
	channel := flag.String("channel", "development", "release channel")
	architecture := flag.String("architecture", "linux-amd64", "target OS and architecture")
	minimum := flag.String("minimum-version", "", "minimum installed client version")
	files := components{}
	flag.Var(files, "component", "component name and source path as name=path; repeatable")
	flag.Parse()
	if *generateKey {
		if *privateKeyPath == "" || *publicKeyPath == "" {
			fatal("--generate-key requires --private-key and --public-key")
		}
		if *output != "" || *release != "" || *minimum != "" || len(files) != 0 {
			fatal("key generation and package creation cannot be combined")
		}
		if err := clientupdate.GenerateSigningKey(*privateKeyPath, *publicKeyPath); err != nil {
			fatal(err.Error())
		}
		fmt.Printf("generated public_key=%s private_key=%s\n", *publicKeyPath, *privateKeyPath)
		return
	}
	if *privateKeyPath == "" || *output == "" || *release == "" || *minimum == "" || len(files) == 0 {
		fatal("package creation requires --private-key, --output, --release, --minimum-version and at least one --component")
	}
	privateKey, err := clientupdate.LoadPrivateKey(*privateKeyPath)
	if err != nil {
		fatal(err.Error())
	}
	options := clientupdate.PackageOptions{ReleaseVersion: *release, Channel: *channel, Architecture: *architecture, MinimumVersion: *minimum, Components: files}
	if err = clientupdate.CreateSignedPackage(*output, privateKey, options); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("created package=%s release=%s components=%d\n", *output, *release, len(files))
}

func fatal(message string) { fmt.Fprintln(os.Stderr, "update packager:", message); os.Exit(1) }
