package pin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	timeCost uint32 = 3
	memory   uint32 = 64 * 1024
	threads  uint8  = 2
	keyLen   uint32 = 32
)

func Hash(value string) (string, error) {
	if len(value) < 4 || len(value) > 32 {
		return "", errors.New("PIN must contain 4 to 32 characters")
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	key := argon2.IDKey([]byte(value), salt, timeCost, memory, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, timeCost, threads, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
func Verify(value, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 6 || p[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var th uint8
	if _, e := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &m, &t, &th); e != nil || m > 256*1024 || t > 10 || th > 16 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(p[4])
	if e != nil {
		return false
	}
	want, e := base64.RawStdEncoding.DecodeString(p[5])
	if e != nil {
		return false
	}
	got := argon2.IDKey([]byte(value), salt, t, m, th, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
