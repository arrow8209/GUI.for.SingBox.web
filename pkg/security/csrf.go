package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
)

// NewCSRFToken 生成 32 字节 base64url 编码的 CSRF token。
func NewCSRFToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// CompareCSRF 常量时间比较。任一为空时返回 false。
func CompareCSRF(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
