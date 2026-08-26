package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/TheRealHZL/stumpfworks-identity/internal/clientstatus"
	"github.com/TheRealHZL/stumpfworks-identity/internal/version"
)

func value(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func main() {
	server := flag.String("server", value("SWBADGE_SERVER", ""), "LOGIN01 HTTPS URL")
	clientID := flag.String("client-id", value("SWBADGE_CLIENT_ID", ""), "registered client ID")
	caFile := flag.String("ca-file", value("SWBADGE_CA_FILE", ""), "additional PEM CA certificate")
	tokenFile := flag.String("token-file", value("SWBADGE_CLIENT_TOKEN_FILE", "/etc/stumpfworks-badge/client-status-token"), "root-readable client token file")
	show := flag.Bool("version", false, "show version")
	flag.Parse()
	if *show {
		fmt.Println("sw-badge-client-status", version.Version)
		return
	}
	if *server == "" || *clientID == "" {
		fatal("server and client ID are required")
	}
	raw, err := os.ReadFile(*tokenFile)
	if err != nil {
		fatal(err.Error())
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		fatal("client token file is empty")
	}
	hc, err := clientstatus.HTTPClient(*caFile)
	if err != nil {
		fatal(err.Error())
	}
	report := clientstatus.Collect(context.Background(), *clientID, version.Version)
	if err = clientstatus.Send(context.Background(), hc, *server, token, report); err != nil {
		fatal(err.Error())
	}
}

func fatal(message string) { fmt.Fprintln(os.Stderr, "client status:", message); os.Exit(1) }
