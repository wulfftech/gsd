package mfa

import (
	"testing"
	"time"
)

// fakeClock lets tests advance time deterministically instead of sleeping.
type fakeClock struct {
	t time.Time
}

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(maxAttempts int, window time.Duration) (*AttemptLimiter, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := NewAttemptLimiter(maxAttempts, window)
	l.now = clock.now
	return l, clock
}

func TestAttemptLimiter_AllowsUpToLimitThenRefuses(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)
	key := "user-1"

	for i := 0; i < 3; i++ {
		if !l.Allow(key) {
			t.Fatalf("Allow() = false on attempt %d, want true (under limit)", i+1)
		}
		l.RecordFailure(key)
	}

	if l.Allow(key) {
		t.Error("Allow() = true after maxAttempts failures, want false")
	}
}

func TestAttemptLimiter_WindowExpiryReAllows(t *testing.T) {
	l, clock := newTestLimiter(2, time.Minute)
	key := "user-1"

	l.RecordFailure(key)
	l.RecordFailure(key)
	if l.Allow(key) {
		t.Fatal("Allow() = true after reaching maxAttempts, want false")
	}

	// Advance past the window; the old failures should no longer count.
	clock.advance(time.Minute + time.Second)

	if !l.Allow(key) {
		t.Error("Allow() = false after window expired, want true")
	}
}

func TestAttemptLimiter_SuccessResetsCounter(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)
	key := "user-1"

	l.RecordFailure(key)
	l.RecordFailure(key)

	l.Reset(key)

	if !l.Allow(key) {
		t.Fatal("Allow() = false immediately after Reset(), want true")
	}

	// A user who fumbles once more after a reset should not be treated as
	// already near the limit.
	l.RecordFailure(key)
	if !l.Allow(key) {
		t.Error("Allow() = false after a single failure post-reset, want true")
	}
}

func TestAttemptLimiter_DistinctKeysTrackedIndependently(t *testing.T) {
	l, _ := newTestLimiter(2, time.Minute)

	l.RecordFailure("user-1")
	l.RecordFailure("user-1")

	if l.Allow("user-1") {
		t.Error("Allow(user-1) = true after it exhausted its budget, want false")
	}
	if !l.Allow("user-2") {
		t.Error("Allow(user-2) = false, want true: a different key must not share user-1's budget")
	}
}

func TestNewAttemptLimiter_ZeroConfigFallsBackToDefault(t *testing.T) {
	// A config file written before MaxVerificationAttempts/RateLimitWindow
	// existed leaves both fields at the Go zero value. That must not be
	// interpreted as "0 attempts allowed" (which would lock out every user
	// immediately); it must fall back to a sane default.
	l, _ := newTestLimiter(0, 0)
	key := "user-1"

	if l.maxAttempts != defaultMaxVerificationAttempts {
		t.Fatalf("maxAttempts = %d, want default %d", l.maxAttempts, defaultMaxVerificationAttempts)
	}
	if l.window != defaultRateLimitWindow {
		t.Fatalf("window = %v, want default %v", l.window, defaultRateLimitWindow)
	}

	for i := 0; i < defaultMaxVerificationAttempts; i++ {
		if !l.Allow(key) {
			t.Fatalf("Allow() = false on attempt %d under the fallback default, want true", i+1)
		}
		l.RecordFailure(key)
	}
	if l.Allow(key) {
		t.Error("Allow() = true after exhausting the fallback default budget, want false")
	}
}

func TestAttemptLimiter_UnknownKeyIsAllowed(t *testing.T) {
	l, _ := newTestLimiter(1, time.Minute)
	if !l.Allow("never-seen-before") {
		t.Error("Allow() = false for a key with no recorded failures, want true")
	}
}
