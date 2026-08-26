package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/TheRealHZL/stumpfworks-identity/internal/database"
	"github.com/TheRealHZL/stumpfworks-identity/internal/version"
)

type ClientStatusRequest struct {
	ClientID       string `json:"client_id"`
	Version        string `json:"version"`
	NetworkStatus  string `json:"network_status"`
	ADStatus       string `json:"ad_status"`
	CameraStatus   string `json:"camera_status"`
	KerberosStatus string `json:"kerberos_status"`
}

func HashClientToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validClientID(v string) bool {
	if len(v) < 1 || len(v) > 64 {
		return false
	}
	for _, r := range v {
		if !(r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func validState(v string) bool {
	return v == "unknown" || v == "ok" || v == "degraded" || v == "unavailable"
}

func (s *Server) clientStatus(w http.ResponseWriter, r *http.Request) {
	var in ClientStatusRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || !validClientID(in.ClientID) || len(in.Version) > 64 || !validState(in.NetworkStatus) || !validState(in.ADStatus) || !validState(in.CameraStatus) || !validState(in.KerberosStatus) {
		s.problem(w, http.StatusBadRequest, "invalid_client_status")
		return
	}
	c, err := s.store.ClientByID(r.Context(), in.ClientID)
	if err != nil {
		s.problem(w, http.StatusUnauthorized, "invalid_client_credentials")
		return
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		s.problem(w, http.StatusUnauthorized, "invalid_client_credentials")
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if token == "" {
		s.problem(w, http.StatusUnauthorized, "invalid_client_credentials")
		return
	}
	got, err := hex.DecodeString(HashClientToken(token))
	if err != nil {
		s.problem(w, http.StatusUnauthorized, "invalid_client_credentials")
		return
	}
	want, err := hex.DecodeString(c.TokenHash)
	if err != nil || subtle.ConstantTimeCompare(got, want) != 1 {
		s.problem(w, http.StatusUnauthorized, "invalid_client_credentials")
		return
	}
	if err = s.store.UpdateClientStatus(r.Context(), in.ClientID, in.Version, in.NetworkStatus, in.ADStatus, in.CameraStatus, in.KerberosStatus); err != nil {
		s.problem(w, http.StatusInternalServerError, "status_update_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) clients(w http.ResponseWriter, r *http.Request) {
	clients, err := s.store.Clients(r.Context())
	if err != nil {
		s.problem(w, http.StatusInternalServerError, "database_error")
		return
	}
	s.json(w, http.StatusOK, clients)
}

type clientView struct {
	ClientID, Version, Network, AD, Camera, Kerberos string
	LastSeen, Health, BadgeClass                     string
}

func clientViews(clients []database.Client, now time.Time) []clientView {
	out := make([]clientView, 0, len(clients))
	for _, c := range clients {
		v := clientView{ClientID: c.ClientID, Version: c.Version, Network: c.NetworkStatus, AD: c.ADStatus, Camera: c.CameraStatus, Kerberos: c.KerberosStatus, LastSeen: "Never", Health: "Never reported", BadgeClass: "neutral"}
		if v.Version == "" {
			v.Version = "—"
		}
		if c.LastSeenAt != nil {
			age := now.Sub(*c.LastSeenAt)
			if age < 0 {
				age = 0
			}
			v.LastSeen = age.Round(time.Second).String() + " ago"
			if age > 3*time.Minute {
				v.Health, v.BadgeClass = "Stale", "warning"
			} else if c.NetworkStatus == "ok" && c.ADStatus == "ok" && c.CameraStatus == "ok" && c.KerberosStatus == "ok" {
				v.Health, v.BadgeClass = "Healthy", "success"
			} else {
				v.Health, v.BadgeClass = "Attention", "danger-text"
			}
		}
		out = append(out, v)
	}
	return out
}

var systemStatusTemplate = template.Must(template.New("system-status").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="dark"><title>System status — StumpfWorks Identity</title><link rel="stylesheet" href="/static/style.css"></head><body><main class="content standalone"><header class="topbar"><div><p class="eyebrow">STUMPFWORKS / IDENTITY</p><h1>System status</h1></div><a class="button secondary" href="/">← Dashboard</a></header><section class="grid status-grid"><article><span class="status-symbol">●</span><small>SERVER</small><strong>Online</strong><p>Authentication API is available.</p></article><article><span class="status-symbol">●</span><small>DATABASE</small><strong>SQLite ready</strong><p>Identity and client data stores are available.</p></article><article><span class="status-symbol">●</span><small>UPTIME</small><strong>{{.Uptime}}</strong><p>Current server process uptime.</p></article><article><span class="status-symbol">●</span><small>VERSION</small><strong>{{.Version}}</strong><p>Running LOGIN01 build.</p></article></section><section class="panel"><div class="panel-head"><div><h2>Registered clients</h2><p>Health reports contain only bounded, non-personal device readiness data. Reports older than three minutes are stale.</p></div><span class="count">{{len .Clients}} registered</span></div><div class="table-wrap"><table><thead><tr><th>Client</th><th>Version</th><th>Last report</th><th>Network</th><th>AD</th><th>Camera</th><th>Kerberos</th><th>Overall</th></tr></thead><tbody>{{range .Clients}}<tr><td class="mono"><strong>{{.ClientID}}</strong></td><td>{{.Version}}</td><td class="muted">{{.LastSeen}}</td><td>{{.Network}}</td><td>{{.AD}}</td><td>{{.Camera}}</td><td>{{.Kerberos}}</td><td><span class="badge {{.BadgeClass}}">{{.Health}}</span></td></tr>{{else}}<tr><td class="empty" colspan="8">No clients registered yet.</td></tr>{{end}}</tbody></table></div></section><p class="privacy">No IP addresses, SSIDs, usernames, Kerberos principals, credentials, or diagnostic logs are collected.</p></main></body></html>`))

func (s *Server) systemStatusPage(w http.ResponseWriter, r *http.Request) {
	clients, err := s.store.Clients(r.Context())
	if err != nil {
		http.Error(w, "System status is temporarily unavailable.", http.StatusInternalServerError)
		return
	}
	data := map[string]any{"Clients": clientViews(clients, time.Now()), "Version": version.Version, "Uptime": time.Since(s.started).Round(time.Second)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := systemStatusTemplate.Execute(w, data); err != nil {
		s.log.Error("system status template failed", "error", err)
	}
}
