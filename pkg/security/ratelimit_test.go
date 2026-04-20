package security

import (
	"testing"
	"time"
)

func TestRateLimiterAllowsUnderThreshold(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute, 5*time.Minute)
	for i := 0; i < 5; i++ {
		if !rl.Allow("ip:1.1.1.1") {
			t.Fatalf("expected attempt %d to be allowed", i+1)
		}
		rl.RecordFailure("ip:1.1.1.1")
	}
}

func TestRateLimiterBlocksAfterMaxFailures(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute, 5*time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("ip:2.2.2.2")
		rl.RecordFailure("ip:2.2.2.2")
	}
	if rl.Allow("ip:2.2.2.2") {
		t.Error("4th attempt should be blocked (locked out)")
	}
}

func TestRateLimiterResetsAfterSuccess(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute, 5*time.Minute)
	rl.RecordFailure("ip:3.3.3.3")
	rl.RecordFailure("ip:3.3.3.3")
	rl.RecordSuccess("ip:3.3.3.3")
	for i := 0; i < 3; i++ {
		if !rl.Allow("ip:3.3.3.3") {
			t.Errorf("attempt %d after success should be allowed", i+1)
		}
		rl.RecordFailure("ip:3.3.3.3")
	}
}

func TestRateLimiterIsolatesKeys(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute, 5*time.Minute)
	rl.RecordFailure("ip:a")
	rl.RecordFailure("ip:a")
	rl.RecordFailure("ip:a")
	if !rl.Allow("ip:b") {
		t.Error("different key should not be affected")
	}
}
