package server

import (
	"bytes"
	"context"
	"encoding/json"
	adminauth "github.com/TheRealHZL/stumpfworks-identity/internal/auth"
	"github.com/TheRealHZL/stumpfworks-identity/internal/badge"
	"github.com/TheRealHZL/stumpfworks-identity/internal/database"
	"github.com/TheRealHZL/stumpfworks-identity/internal/directory"
	userpin "github.com/TheRealHZL/stumpfworks-identity/internal/pin"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeDirectory struct{}

func (fakeDirectory) UserExists(context.Context, string) (bool, error) { return true, nil }
func (fakeDirectory) GetUser(_ context.Context, u string) (*directory.User, error) {
	return &directory.User{Username: u, DisplayName: "Alice Example", DN: "CN=Alice Example,DC=example"}, nil
}
func (fakeDirectory) ListUsers(context.Context) ([]directory.User, error) {
	return []directory.User{{Username: "alice"}}, nil
}
func (fakeDirectory) AuthenticateUser(_ context.Context, u, p string) (*directory.User, error) {
	if p == "correct" {
		return &directory.User{Username: u, DisplayName: "Alice Example", DN: "CN=Alice Example,DC=example"}, nil
	}
	return nil, directory.ErrInvalidCredentials
}
func (fakeDirectory) AuthenticateAdmin(_ context.Context, u, p string) (*directory.User, error) {
	if u == "alice-admin" && p == "correct" {
		return &directory.User{Username: u}, nil
	}
	return nil, directory.ErrInvalidCredentials
}

func testServer(t *testing.T) (*Server, *database.Store) {
	t.Helper()
	st, e := database.Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, slog.New(slog.NewTextHandler(io.Discard, nil))), st
}
func TestHealth(t *testing.T) {
	s, _ := testServer(t)
	r := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestClientStatus(t *testing.T) {
	s, st := testServer(t)
	token := "a-long-random-client-token"
	if _, err := st.CreateClient(t.Context(), "wyse01-greeter", HashClientToken(token)); err != nil {
		t.Fatal(err)
	}
	body := `{"client_id":"wyse01-greeter","version":"1.2.0-dev","network_status":"ok","ad_status":"ok","camera_status":"degraded","kerberos_status":"unknown"}`
	request := func(given string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/client/status", strings.NewReader(body))
		if given != "" {
			r.Header.Set("Authorization", "Bearer "+given)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := request("wrong"); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token returned %d", w.Code)
	}
	if w := request(token); w.Code != http.StatusNoContent {
		t.Fatalf("valid status returned %d: %s", w.Code, w.Body.String())
	}
	if err := st.SetClientEnabled(t.Context(), "wyse01-greeter", false); err != nil {
		t.Fatal(err)
	}
	if w := request(token); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "client_disabled") {
		t.Fatalf("disabled client returned %d: %s", w.Code, w.Body.String())
	}
	if w := request("wrong"); w.Code != http.StatusUnauthorized {
		t.Fatalf("disabled client disclosed state to wrong token: %d", w.Code)
	}
	c, err := st.ClientByID(t.Context(), "wyse01-greeter")
	if err != nil || c.Version != "1.2.0-dev" || c.CameraStatus != "degraded" || c.LastSeenAt == nil {
		t.Fatalf("client status not persisted: %+v, %v", c, err)
	}
}

func TestSystemStatusClientOverview(t *testing.T) {
	s, st := testServer(t)
	if _, err := st.CreateClient(t.Context(), "client01", HashClientToken("must-not-leak")); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateClientStatus(t.Context(), "client01", "1.2.0-dev", "ok", "ok", "ok", "ok"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "client01") || !strings.Contains(w.Body.String(), "Healthy") {
		t.Fatalf("status page returned %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "must-not-leak") {
		t.Fatal("client token leaked into status page")
	}
}

func TestClientViewMarksStaleReports(t *testing.T) {
	now := time.Now()
	old := now.Add(-4 * time.Minute)
	views := clientViews([]database.Client{{ClientID: "old-client", Enabled: true, Version: "1.1.0", NetworkStatus: "ok", ADStatus: "ok", CameraStatus: "ok", KerberosStatus: "ok", LastSeenAt: &old}}, now, "1.2.0-dev")
	if len(views) != 1 || views[0].Health != "Stale" || views[0].BadgeClass != "warning" {
		t.Fatalf("unexpected stale view: %+v", views)
	}
}

func TestClientVersionUpdateState(t *testing.T) {
	tests := []struct{ client, server, want string }{{"1.1.0", "1.2.0-dev", "Update available"}, {"1.2.0-dev", "1.2.0-dev", "Current"}, {"1.2.0-dev", "1.2.0", "Update available"}, {"1.2.0-rc.2", "1.2.0-rc.10", "Update available"}, {"1.2.0+client", "1.2.0+server", "Current"}, {"1.3.0", "1.2.0", "Client newer"}, {"unknown", "1.2.0", "Unknown"}}
	for _, test := range tests {
		if got, _ := updateState(test.client, test.server); got != test.want {
			t.Errorf("updateState(%q,%q)=%q, want %q", test.client, test.server, got, test.want)
		}
	}
}
func TestAuth(t *testing.T) {
	s, st := testServer(t)
	u, e := st.CreateUser(t.Context(), "alice", "Alice Example", "")
	if e != nil {
		t.Fatal(e)
	}
	token := "secret"
	b, e := st.CreateBadge(t.Context(), u.ID, badge.HashToken(token), "")
	if e != nil {
		t.Fatal(e)
	}
	call := func(tok string) AuthResponse {
		raw, _ := json.Marshal(AuthRequest{BadgeID: b.BadgeCode, Token: tok, ClientID: "test"})
		r := httptest.NewRequest("POST", "/api/v1/auth/badge", bytes.NewReader(raw))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		var out AuthResponse
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	if x := call(token); !x.Valid || x.Username != "alice" {
		t.Fatal(x)
	}
	if x := call("wrong"); x.Valid || x.Reason != "invalid_token" {
		t.Fatal(x)
	}
	pinHash, _ := userpin.Hash("5831")
	if e = st.SetUserPIN(t.Context(), u.ID, pinHash); e != nil {
		t.Fatal(e)
	}
	callPIN := func(pin string) AuthResponse {
		raw, _ := json.Marshal(AuthRequest{BadgeID: b.BadgeCode, Token: token, ClientID: "test", PIN: pin})
		r := httptest.NewRequest("POST", "/api/v1/auth/badge", bytes.NewReader(raw))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		var out AuthResponse
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	if x := callPIN(""); x.Valid || x.Reason != "pin_required" {
		t.Fatal(x)
	}
	if x := callPIN("0000"); x.Valid || x.Reason != "invalid_pin" {
		t.Fatal(x)
	}
	if x := callPIN("5831"); !x.Valid {
		t.Fatal(x)
	}
	_ = st.Revoke(t.Context(), b.ID)
	if x := call(token); x.Valid || x.Reason != "badge_disabled" {
		t.Fatal(x)
	}
	raw, _ := json.Marshal(AuthRequest{BadgeID: "SW-9999", Token: token, ClientID: "test"})
	req, _ := http.NewRequest("POST", "/api/v1/auth/badge", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	var out AuthResponse
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Reason != "badge_unknown" {
		t.Fatal(out)
	}
}

func TestAdminLoginProtection(t *testing.T) {
	_, st := testServer(t)
	sessions, _ := adminauth.NewSessions("01234567890123456789012345678901", time.Hour)
	s := NewProtected(st, slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDirectory{}, sessions)
	r := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 303 || w.Header().Get("Location") != "/login" {
		t.Fatal(w.Code, w.Header())
	}
	form := strings.NewReader("username=alice-admin&password=correct")
	r = httptest.NewRequest("POST", "/login", form)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 303 || len(w.Result().Cookies()) != 1 {
		t.Fatal(w.Code, w.Header())
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal(cookie)
	}
	r = httptest.NewRequest("GET", "/api/v1/directory/users", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "alice") {
		t.Fatal(w.Code, w.Body.String())
	}
	r = httptest.NewRequest("POST", "/directory/users/import", strings.NewReader("username=alice"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("missing CSRF returned %d", w.Code)
	}
	form = strings.NewReader("username=alice&_csrf=" + sessions.CSRF(cookie.Value))
	r = httptest.NewRequest("POST", "/directory/users/import", form)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 303 {
		t.Fatalf("import returned %d: %s", w.Code, w.Body.String())
	}
}

func TestSelfServicePIN(t *testing.T) {
	_, st := testServer(t)
	sessions, _ := adminauth.NewSessions("01234567890123456789012345678901", time.Hour)
	s := NewProtected(st, slog.New(slog.NewTextHandler(io.Discard, nil)), fakeDirectory{}, sessions)

	form := strings.NewReader("username=alice&password=correct")
	r := httptest.NewRequest("POST", "/self-service/login", form)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || len(w.Result().Cookies()) != 1 {
		t.Fatalf("self-service login returned %d", w.Code)
	}
	cookie := w.Result().Cookies()[0]
	if cookie.Name != "swbadge_selfservice" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal(cookie)
	}
	if _, e := sessions.Verify(cookie.Value, time.Now()); e == nil {
		t.Fatal("self-service cookie accepted as admin")
	}
	r = httptest.NewRequest("GET", "/self-service", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Hello, Alice Example") {
		t.Fatalf("authenticated self-service page returned %d: %s", w.Code, w.Body.String())
	}

	form = strings.NewReader("pin=5831&pin_confirm=5831&_csrf=" + sessions.CSRF(cookie.Value))
	r = httptest.NewRequest("POST", "/self-service/pin", form)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("PIN update returned %d: %s", w.Code, w.Body.String())
	}
	u, e := st.UserByUsername(t.Context(), "alice")
	if e != nil || !userpin.Verify("5831", u.PINHash) {
		t.Fatal("self-service PIN was not stored for authenticated user")
	}

	form = strings.NewReader("pin=9999&pin_confirm=9999&_csrf=invalid")
	r = httptest.NewRequest("POST", "/self-service/pin", form)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("invalid CSRF returned %d", w.Code)
	}
}
