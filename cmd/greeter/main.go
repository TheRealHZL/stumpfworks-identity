package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/TheRealHZL/stumpfworks-identity/internal/badge"
	"github.com/TheRealHZL/stumpfworks-identity/internal/scanner"
	app "github.com/TheRealHZL/stumpfworks-identity/internal/server"
)

type publicState struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	BadgeID     string `json:"badge_id,omitempty"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}
type greeter struct {
	mu                sync.Mutex
	state             publicState
	badgeID, token    string
	server, client    string
	http              *http.Client
	helper            string
	authorizedCommand string
}

func main() {
	server := flag.String("server", "https://login01.example.test:8080", "badge server URL")
	client := flag.String("client-id", "client01-greeter", "client ID")
	ca := flag.String("ca-file", "/etc/stumpfworks-badge/stumpfworks-homelab-ca.crt", "PEM CA file")
	helper := flag.String("camera-helper", "/usr/local/bin/sw-badge-camera-linux", "camera helper")
	listen := flag.String("listen", "127.0.0.1:18081", "local UI listener")
	authorizedCommand := flag.String("authorized-command", "", "optional executable invoked with the authorized username")
	flag.Parse()
	hc, err := httpClient(*ca)
	if err != nil {
		log.Fatal(err)
	}
	g := &greeter{state: publicState{Status: "idle", Message: "Ready to scan"}, server: strings.TrimRight(*server, "/"), client: *client, http: hc, helper: *helper, authorizedCommand: *authorizedCommand}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", g.page)
	mux.HandleFunc("GET /api/state", g.getState)
	mux.HandleFunc("POST /api/scan", g.startScan)
	mux.HandleFunc("POST /api/pin", g.submitPIN)
	mux.HandleFunc("POST /api/reset", g.reset)
	srv := http.Server{Addr: *listen, Handler: security(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}
	log.Printf("StumpfWorks Greeter ready at http://%s", *listen)
	log.Fatal(srv.ListenAndServe())
}
func httpClient(path string) (*http.Client, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("CA file contains no certificates")
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	return &http.Client{Timeout: 10 * time.Second, Transport: tr}, nil
}
func (g *greeter) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, page)
}
func (g *greeter) getState(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	s := g.state
	g.mu.Unlock()
	writeJSON(w, s)
}
func (g *greeter) startScan(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	if g.state.Status == "scanning" {
		g.mu.Unlock()
		writeJSON(w, map[string]string{"status": "already_scanning"})
		return
	}
	g.clearLocked()
	g.state = publicState{Status: "scanning", Message: "Hold your QR badge in front of the camera"}
	g.mu.Unlock()
	go g.scan()
	writeJSON(w, map[string]string{"status": "started"})
}
func (g *greeter) scan() {
	value, err := (scanner.Command{Path: g.helper}).Scan(context.Background())
	if err != nil {
		g.fail("Camera scan failed")
		return
	}
	code, token, err := badge.ParsePayload(value)
	if err != nil {
		g.fail("Invalid StumpfWorks badge")
		return
	}
	g.mu.Lock()
	g.badgeID, g.token = code, token
	g.state = publicState{Status: "checking", Message: "Checking badge…", BadgeID: code}
	g.mu.Unlock()
	out, err := g.authorize("")
	if err != nil {
		g.fail("Badge server unavailable")
		return
	}
	g.apply(out)
}
func (g *greeter) submitPIN(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	pin := r.FormValue("pin")
	g.mu.Lock()
	ready := g.state.Status == "pin" && g.token != ""
	g.mu.Unlock()
	if !ready {
		http.Error(w, "no PIN requested", 409)
		return
	}
	out, err := g.authorize(pin)
	pin = ""
	if err != nil {
		g.fail("Badge server unavailable")
		http.Error(w, "server unavailable", 502)
		return
	}
	g.apply(out)
	writeJSON(w, map[string]string{"status": "checked"})
}
func (g *greeter) authorize(pin string) (app.AuthResponse, error) {
	g.mu.Lock()
	in := app.AuthRequest{BadgeID: g.badgeID, Token: g.token, ClientID: g.client, PIN: pin}
	g.mu.Unlock()
	raw, _ := json.Marshal(in)
	resp, err := g.http.Post(g.server+"/api/v1/auth/badge", "application/json", bytes.NewReader(raw))
	if err != nil {
		return app.AuthResponse{}, err
	}
	defer resp.Body.Close()
	var out app.AuthResponse
	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
	return out, err
}
func (g *greeter) apply(out app.AuthResponse) {
	g.mu.Lock()
	handoff := ""
	grant := ""
	switch {
	case out.Valid:
		g.state = publicState{Status: "authorized", Message: "Authentication successful", BadgeID: out.BadgeID, Username: out.Username, DisplayName: out.DisplayName}
		g.clearSecretsLocked()
		if validUsername(out.Username) && out.LoginGrant != "" {
			handoff = out.Username
			grant = out.LoginGrant
		}
	case out.Reason == "pin_required":
		g.state.Status = "pin"
		g.state.Message = "Enter your personal PIN"
	case out.Reason == "invalid_pin":
		g.state.Status = "pin"
		g.state.Message = "Incorrect PIN — try again"
	case out.Reason == "pin_rate_limited":
		g.state = publicState{Status: "denied", Message: "Too many incorrect PIN attempts"}
		g.clearSecretsLocked()
	default:
		g.state = publicState{Status: "denied", Message: "Access denied: " + out.Reason}
		g.clearSecretsLocked()
	}
	g.mu.Unlock()
	if handoff != "" && g.authorizedCommand != "" {
		cmd := exec.Command(g.authorizedCommand, handoff)
		cmd.Stdin = strings.NewReader(grant + "\n")
		if err := cmd.Start(); err != nil {
			g.fail("LightDM handoff failed")
		}
	}
}

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func validUsername(v string) bool { return usernamePattern.MatchString(v) }
func (g *greeter) reset(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	g.clearLocked()
	g.state = publicState{Status: "idle", Message: "Ready to scan"}
	g.mu.Unlock()
	writeJSON(w, map[string]string{"status": "reset"})
}
func (g *greeter) fail(message string) {
	g.mu.Lock()
	g.state = publicState{Status: "error", Message: message}
	g.clearSecretsLocked()
	g.mu.Unlock()
}
func (g *greeter) clearLocked()        { g.clearSecretsLocked() }
func (g *greeter) clearSecretsLocked() { g.badgeID = ""; g.token = "" }
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'")
		if r.Method == "POST" && !strings.HasPrefix(r.Header.Get("Origin"), "http://127.0.0.1:18081") {
			http.Error(w, "invalid origin", 403)
			return
		}
		next.ServeHTTP(w, r)
	})
}

const page = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>StumpfWorks Login</title><style>:root{font:18px system-ui;color-scheme:dark;--green:#4ee1a0;--blue:#4e8cff}*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;background:radial-gradient(circle at top,#19283e,#080d15 65%);color:#f2f7ff}.card{width:min(620px,92vw);padding:46px;border:1px solid #2a3c55;border-radius:24px;background:#111a27;box-shadow:0 24px 80px #0008;text-align:center}.brand{letter-spacing:3px;color:var(--green);font-size:13px}h1{font-size:38px;margin:12px 0}.icon{font-size:80px;margin:25px}.message{color:#a9b8cc;min-height:30px}.badge{font-family:monospace;color:var(--green)}button,input{font:inherit;padding:14px 18px;border-radius:11px;border:1px solid #344861}button{background:var(--blue);color:white;border:0;cursor:pointer}input{background:#09111d;color:white;width:220px;text-align:center;letter-spacing:8px}.hidden{display:none}.authorized h1{color:var(--green)}.denied h1,.error h1{color:#ff6f88}</style></head><body><main class="card" id="card"><div class="brand">STUMPFWORKS / IDENTITY</div><div class="icon" id="icon">▣</div><h1 id="title">Badge Login</h1><p class="message" id="message">Ready to scan</p><p class="badge" id="badge"></p><button id="scan">Scan badge</button><form id="pinForm" class="hidden"><input id="pin" name="pin" type="password" inputmode="numeric" minlength="4" maxlength="32" autocomplete="off" placeholder="PIN"><button>Continue</button></form><button id="reset" class="hidden">Start over</button></main><script>const $=id=>document.getElementById(id);let last='';async function post(url,body){await fetch(url,{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:body||''})}async function poll(){let s=await fetch('/api/state',{cache:'no-store'}).then(r=>r.json());$('card').className='card '+s.status;$('message').textContent=s.message;$('badge').textContent=s.badgeID||'';$('scan').classList.toggle('hidden',s.status!=='idle');$('pinForm').classList.toggle('hidden',s.status!=='pin');$('reset').classList.toggle('hidden',!['authorized','denied','error'].includes(s.status));if(s.status==='pin'&&last!=='pin')$('pin').focus();if(s.status==='authorized'){$('title').textContent='Welcome, '+(s.displayName||s.username);$('icon').textContent='✓'}else if(s.status==='denied'||s.status==='error'){$('title').textContent='Access denied';$('icon').textContent='×'}else{$('title').textContent='Badge Login';$('icon').textContent='▣'}last=s.status}setInterval(poll,500);poll();$('scan').onclick=()=>post('/api/scan');$('reset').onclick=async()=>{await post('/api/reset');last='';poll()};$('pinForm').onsubmit=async e=>{e.preventDefault();let p=$('pin').value;$('pin').value='';await post('/api/pin','pin='+encodeURIComponent(p));p=''};</script></body></html>`
