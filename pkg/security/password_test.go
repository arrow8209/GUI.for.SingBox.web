package security

import (
	"strings"
	"testing"
)

func TestHashPasswordProducesArgon2idEncoding(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("expected argon2id prefix, got %q", hash)
	}
}

func TestVerifyPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Error("VerifyPassword should succeed for correct password")
	}
	if VerifyPassword("wrong", hash) {
		t.Error("VerifyPassword should fail for wrong password")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	if VerifyPassword("any", "not-argon2id") {
		t.Error("malformed hash should not verify")
	}
	if VerifyPassword("any", "") {
		t.Error("empty hash should not verify")
	}
}

func TestHashPasswordSaltDiffers(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of same password must differ (salt)")
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	a, err := GenerateRandomPassword()
	if err != nil {
		t.Fatalf("GenerateRandomPassword: %v", err)
	}
	b, _ := GenerateRandomPassword()
	if a == b {
		t.Error("two random passwords must differ")
	}
	if len(a) < 20 {
		t.Errorf("password too short: %d chars", len(a))
	}
}
