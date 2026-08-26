package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/TheRealHZL/stumpfworks-identity/internal/badge"
	"github.com/TheRealHZL/stumpfworks-identity/internal/scanner"
	app "github.com/TheRealHZL/stumpfworks-identity/internal/server"
	"github.com/TheRealHZL/stumpfworks-identity/internal/version"
	"golang.org/x/term"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	url := flag.String("server", "http://127.0.0.1:8080", "server URL")
	client := flag.String("client-id", hostname(), "client ID")
	payload := flag.String("badge", "", "badge payload test mode")
	camera := flag.Bool("camera", false, "scan one QR code with the optional platform camera helper")
	cameraHelper := flag.String("camera-helper", "", "optional path to a platform camera scanner")
	caFile := flag.String("ca-file", "", "additional PEM CA certificate for HTTPS")
	show := flag.Bool("version", false, "show version")
	flag.Parse()
	if *show {
		fmt.Println("sw-badge-agent", version.Version)
		return
	}
	fmt.Printf("StumpfWorks Badge Agent\nServer: %s\nClient: %s\n\n", *url, *client)
	v := *payload
	if v == "" {
		fmt.Println("Waiting for badge...")
		var e error
		if *camera {
			helper := *cameraHelper
			if helper == "" {
				if runtime.GOOS == "darwin" {
					helper = filepath.Join(filepath.Dir(os.Args[0]), "sw-badge-camera-macos")
				} else {
					helper = "/usr/local/bin/sw-badge-camera-linux"
				}
			}
			v, e = (scanner.Command{Path: helper}).Scan(context.Background())
		} else {
			v, e = (scanner.Manual{In: os.Stdin, Out: os.Stdout}).Scan(context.Background())
		}
		if e != nil {
			fatal(e)
		}
	}
	code, token, e := badge.ParsePayload(v)
	if e != nil {
		fatal(e)
	}
	fmt.Printf("Badge detected\nBadge ID: %s\n\nContacting server...\n\n", code)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if *caFile != "" {
		pem, e := os.ReadFile(*caFile)
		if e != nil {
			fatal(e)
		}
		roots, e := x509.SystemCertPool()
		if e != nil {
			fatal(e)
		}
		if !roots.AppendCertsFromPEM(pem) {
			fatal(fmt.Errorf("CA file contains no certificates"))
		}
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	}
	hc := http.Client{Timeout: 10 * time.Second, Transport: transport}
	out, e := authorize(&hc, *url, app.AuthRequest{BadgeID: code, Token: token, ClientID: *client})
	if e != nil {
		fatal(e)
	}
	if !out.Valid && out.Reason == "pin_required" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fatal(fmt.Errorf("PIN required but no interactive terminal is available"))
		}
		fmt.Print("PIN: ")
		raw, e := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if e != nil {
			fatal(e)
		}
		pinValue := string(raw)
		out, e = authorize(&hc, *url, app.AuthRequest{BadgeID: code, Token: token, ClientID: *client, PIN: pinValue})
		for i := range raw {
			raw[i] = 0
		}
		pinValue = ""
		if e != nil {
			fatal(e)
		}
	}
	if out.Valid {
		fmt.Printf("AUTHORIZED\nUser: %s\nDisplay name: %s\n", out.Username, out.DisplayName)
	} else {
		fmt.Printf("DENIED\nReason: %s\n", out.Reason)
		os.Exit(2)
	}
}
func authorize(hc *http.Client, url string, in app.AuthRequest) (app.AuthResponse, error) {
	body, _ := json.Marshal(in)
	resp, e := hc.Post(url+"/api/v1/auth/badge", "application/json", bytes.NewReader(body))
	if e != nil {
		return app.AuthResponse{}, e
	}
	defer resp.Body.Close()
	var out app.AuthResponse
	e = json.NewDecoder(resp.Body).Decode(&out)
	return out, e
}
func hostname() string {
	h, e := os.Hostname()
	if e != nil || h == "" {
		return "unknown-client"
	}
	return h
}
func fatal(e error) { fmt.Fprintln(os.Stderr, "Error:", e); os.Exit(1) }
