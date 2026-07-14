package api

import "testing"

func TestLoginLimiter_BlocksAfterMaxFailures(t *testing.T) {
	l := newLoginLimiter()
	ip := "203.0.113.7"

	for i := 0; i < loginMaxFailures; i++ {
		if !l.allow(ip) {
			t.Fatalf("blocked too early, after %d failures", i)
		}
		l.recordFailure(ip)
	}

	if l.allow(ip) {
		t.Error("expected IP to be blocked after max failures")
	}

	if !l.allow("203.0.113.8") {
		t.Error("other IPs should not be affected")
	}
}

func TestLoginLimiter_ResetClearsFailures(t *testing.T) {
	l := newLoginLimiter()
	ip := "203.0.113.7"

	for i := 0; i < loginMaxFailures; i++ {
		l.recordFailure(ip)
	}
	l.reset(ip)

	if !l.allow(ip) {
		t.Error("expected reset to clear the block")
	}
}
