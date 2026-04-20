package security

import "testing"

func TestOriginCheckerExactMatch(t *testing.T) {
	c := NewOriginChecker([]string{"https://panel.example.com"})
	if !c.Allow("https://panel.example.com") {
		t.Error("exact match should allow")
	}
	if c.Allow("https://evil.com") {
		t.Error("non-listed origin should be denied")
	}
}

func TestOriginCheckerPortWildcard(t *testing.T) {
	c := NewOriginChecker([]string{"http://127.0.0.1:*"})
	if !c.Allow("http://127.0.0.1:5173") {
		t.Error("port wildcard should match dev server")
	}
	if !c.Allow("http://127.0.0.1:22345") {
		t.Error("port wildcard should match prod port")
	}
	if c.Allow("http://127.0.0.1") {
		t.Error("missing port should not match (when pattern has port)")
	}
	if c.Allow("https://127.0.0.1:5173") {
		t.Error("scheme mismatch should be denied")
	}
}

func TestOriginCheckerEmptyAlwaysDeny(t *testing.T) {
	c := NewOriginChecker(nil)
	if c.Allow("http://anything") {
		t.Error("empty whitelist should deny everything")
	}
	if c.Allow("") {
		t.Error("empty origin should be denied")
	}
}

func TestOriginCheckerEmptyOrigin(t *testing.T) {
	c := NewOriginChecker([]string{"http://127.0.0.1:*"})
	if c.Allow("") {
		t.Error("empty origin must always be denied (curl etc.)")
	}
}
