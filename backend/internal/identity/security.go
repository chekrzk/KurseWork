package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"golang.org/x/crypto/argon2"
)

var encoding = base64.RawURLEncoding

func random(n int) []byte { b := make([]byte, n); rand.Read(b); return b }
func derive(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
}
func hashPassword(password string) string {
	salt := random(16)
	return "argon2id$v19$m65536,t3,p2$" + encoding.EncodeToString(salt) + "$" + encoding.EncodeToString(derive(password, salt))
}
func matches(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 5 || strings.Join(parts[:3], "$") != "argon2id$v19$m65536,t3,p2" {
		return false
	}
	salt, e := encoding.DecodeString(parts[3])
	if e != nil || len(salt) != 16 {
		return false
	}
	expected, e := encoding.DecodeString(parts[4])
	return e == nil && subtle.ConstantTimeCompare(expected, derive(password, salt)) == 1
}
func digest(token string) []byte { h := sha256.Sum256([]byte(token)); return h[:] }
func CSRF(token string) string {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte("saturnita:csrf:v1"))
	return encoding.EncodeToString(m.Sum(nil))
}
