package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/TheRealHZL/stumpfworks-identity/internal/clientdiagnostics"
	"github.com/TheRealHZL/stumpfworks-identity/internal/version"
)

func env(name string) string { return os.Getenv(name) }

func main() {
	clientID := flag.String("client-id", env("SWBADGE_CLIENT_ID"), "configured client ID")
	server := flag.String("server", env("SWBADGE_SERVER"), "configured LOGIN01 URL")
	caFile := flag.String("ca-file", env("SWBADGE_CA_FILE"), "configured CA file")
	tokenFile := flag.String("token-file", env("SWBADGE_CLIENT_TOKEN_FILE"), "configured status-token file")
	output := flag.String("output", "swbadge-diagnostics.zip", "output ZIP path")
	show := flag.Bool("version", false, "show version")
	flag.Parse()
	if *show {
		fmt.Println("sw-badge-client-diagnostics", version.Version)
		return
	}
	if *clientID == "" {
		fatal("client ID is required")
	}
	report := clientdiagnostics.Collect(context.Background(), clientdiagnostics.Options{ClientID: *clientID, Version: version.Version, ServerURL: *server, CAFile: *caFile, TokenFile: *tokenFile}, time.Now())
	if err := clientdiagnostics.WriteArchive(*output, report); err != nil {
		fatal(err.Error())
	}
	fmt.Println(*output)
}

func fatal(message string) { fmt.Fprintln(os.Stderr, "client diagnostics:", message); os.Exit(1) }
