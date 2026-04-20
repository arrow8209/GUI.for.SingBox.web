package security

import "testing"

func TestNewCSRFTokenIsRandomAndLong(t *testing.T) {
	a, err := NewCSRFToken()
	if err != nil {
		t.Fatalf("NewCSRFToken: %v", err)
	}
	b, _ := NewCSRFToken()
	if a == b {
		t.Error("two CSRF tokens must differ")
	}
	if len(a) < 32 {
		t.Errorf("token too short: %d", len(a))
	}
}

func TestCompareCSRFConstantTime(t *testing.T) {
	tok, _ := NewCSRFToken()
	if !CompareCSRF(tok, tok) {
		t.Error("equal tokens should compare equal")
	}
	if CompareCSRF(tok, tok+"x") {
		t.Error("different tokens should not compare equal")
	}
	if CompareCSRF("", tok) || CompareCSRF(tok, "") {
		t.Error("empty token should never match")
	}
}
