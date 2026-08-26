package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidSession = errors.New("invalid session")

type Sessions struct {
	key      []byte
	lifetime time.Duration
}

func NewSessions(secret string, lifetime time.Duration) (*Sessions, error) {
	if len(secret) < 32 {
		return nil, errors.New("session secret must contain at least 32 bytes")
	}
	return &Sessions{key: []byte(secret), lifetime: lifetime}, nil
}
func (s *Sessions) Issue(username string, now time.Time) string {
	return s.IssueFor("admin", username, now)
}

func (s *Sessions) IssueFor(audience, username string, now time.Time) string {
	return s.IssueForDuration(audience, username, now, s.lifetime)
}

func (s *Sessions) IssueForDuration(audience, username string, now time.Time, lifetime time.Duration) string {
	identity := audience + ":" + username
	payload := base64.RawURLEncoding.EncodeToString([]byte(identity)) + "." + strconv.FormatInt(now.Add(lifetime).Unix(), 10)
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Sessions) Verify(token string, now time.Time) (string, error) {
	return s.VerifyFor(token, "admin", now)
}

func (s *Sessions) VerifyFor(token, audience string, now time.Time) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", ErrInvalidSession
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, e := base64.RawURLEncoding.DecodeString(parts[2])
	if e != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return "", ErrInvalidSession
	}
	expiry, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil || now.Unix() > expiry {
		return "", ErrInvalidSession
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[0])
	prefix := audience + ":"
	if e != nil || !strings.HasPrefix(string(raw), prefix) || len(raw) == len(prefix) {
		return "", ErrInvalidSession
	}
	return strings.TrimPrefix(string(raw), prefix), nil
}

func (s *Sessions) CSRF(sessionToken string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("csrf:" + sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Sessions) VerifyCSRF(sessionToken, csrfToken string) bool {
	expected, err := base64.RawURLEncoding.DecodeString(s.CSRF(sessionToken))
	if err != nil {
		return false
	}
	actual, err := base64.RawURLEncoding.DecodeString(csrfToken)
	return err == nil && hmac.Equal(expected, actual)
}
