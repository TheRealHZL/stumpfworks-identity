package badge

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidPayload = errors.New("invalid badge payload")

func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func HashToken(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}
func VerifyToken(token, hash string) bool {
	a, err := hex.DecodeString(HashToken(token))
	if err != nil {
		return false
	}
	b, err := hex.DecodeString(hash)
	return err == nil && len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1
}
func Payload(code, token string) string { return "SWBADGE:1:" + code + ":" + token }
func ParsePayload(v string) (string, string, error) {
	p := strings.Split(v, ":")
	if len(p) != 4 || p[0] != "SWBADGE" || p[1] != "1" || p[2] == "" || p[3] == "" {
		return "", "", ErrInvalidPayload
	}
	return p[2], p[3], nil
}
