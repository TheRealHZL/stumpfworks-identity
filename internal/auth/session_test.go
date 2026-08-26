package auth

import (
	"testing"
	"time"
)

func TestSessions(t *testing.T) {
	s, e := NewSessions("01234567890123456789012345678901", time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1000, 0)
	token := s.Issue("alice-admin", now)
	u, e := s.Verify(token, now)
	if e != nil || u != "alice-admin" {
		t.Fatal(u, e)
	}
	if _, e = s.Verify(token+"x", now); e == nil {
		t.Fatal("tampered session accepted")
	}
	if _, e = s.Verify(token, now.Add(2*time.Hour)); e == nil {
		t.Fatal("expired session accepted")
	}
	selfToken := s.IssueFor("self-service", "alice", now)
	if _, e = s.Verify(selfToken, now); e == nil {
		t.Fatal("self-service session accepted as admin session")
	}
	if u, e = s.VerifyFor(selfToken, "self-service", now); e != nil || u != "alice" {
		t.Fatal(u, e)
	}
}
func TestShortSecret(t *testing.T) {
	if _, e := NewSessions("short", time.Hour); e == nil {
		t.Fatal("short secret accepted")
	}
}
func TestCSRF(t *testing.T) {
	s, _ := NewSessions("01234567890123456789012345678901", time.Hour)
	session := s.Issue("admin", time.Now())
	csrf := s.CSRF(session)
	if !s.VerifyCSRF(session, csrf) || s.VerifyCSRF(session, csrf+"x") {
		t.Fatal("csrf verification failed")
	}
}
