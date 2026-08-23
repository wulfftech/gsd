package mfa

import (
	"sync"
	"time"
)

// defaultMaxVerificationAttempts and defaultRateLimitWindow back-stop
// config.MFAConfig.MaxVerificationAttempts / RateLimitWindow when they are
// unset (Go zero value). Deployments with a config file written before these
// keys existed would otherwise get maxAttempts=0, which would refuse every
// attempt outright instead of applying a sane limit.
const (
	defaultMaxVerificationAttempts = 5
	defaultRateLimitWindow         = 5 * time.Minute
)

// sweepInterval bounds how often a full-map stale-entry sweep runs.
const sweepInterval = 10 * time.Minute

// TooManyAttemptsMessage is the client-facing message for a refused attempt.
// Shared so every call site answers a throttled request identically.
const TooManyAttemptsMessage = "Too many MFA verification attempts. Please try again later."

// attemptRecord tracks failures for a single key within the current window.
type attemptRecord struct {
	failures    int
	windowStart time.Time
}

// AttemptLimiter tracks failed MFA verification attempts per key (normally a
// user ID) and refuses further attempts once maxAttempts failures land
// inside window. It exists because MFA code verification (login, disable,
// setup confirmation, backup-code regeneration, and the optional
// X-MFA-Code header check) previously had no throttle beyond the global
// per-IP limiter (config.Server.RateLimit), which makes TOTP brute force
// against a single account feasible.
//
// In-memory and mutex-guarded, matching the house pattern in
// internal/realtime.RateLimiter — this is a single-node SQLite deployment,
// so there is no need for a shared/external store.
//
// Unlike RateLimiter, stale entries are pruned opportunistically on access
// rather than via a ticker goroutine: MFA verification is comparatively
// low-frequency (nowhere near connection-level traffic), each entry is a
// few dozen bytes, and avoiding a background goroutine means AttemptLimiter
// needs no Stop()/fx lifecycle wiring and cannot leak a goroutine in tests.
type AttemptLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*attemptRecord
	maxAttempts int
	window      time.Duration
	lastSweep   time.Time

	// now is overridable so tests can control time without real sleeps.
	now func() time.Time
}

// NewAttemptLimiter builds a limiter allowing maxAttempts failures per
// window for any given key. A non-positive maxAttempts or window falls back
// to a default rather than locking every user out immediately.
func NewAttemptLimiter(maxAttempts int, window time.Duration) *AttemptLimiter {
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxVerificationAttempts
	}
	if window <= 0 {
		window = defaultRateLimitWindow
	}

	return &AttemptLimiter{
		attempts:    make(map[string]*attemptRecord),
		maxAttempts: maxAttempts,
		window:      window,
		now:         time.Now,
	}
}

// Allow reports whether another verification attempt for key is currently
// permitted.
func (l *AttemptLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)

	rec, exists := l.attempts[key]
	if !exists {
		return true
	}
	if now.Sub(rec.windowStart) >= l.window {
		// The window has fully elapsed; drop the stale record so a fresh
		// one starts on the next failure.
		delete(l.attempts, key)
		return true
	}
	return rec.failures < l.maxAttempts
}

// RecordFailure registers a failed verification attempt for key.
func (l *AttemptLimiter) RecordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)

	rec, exists := l.attempts[key]
	if !exists || now.Sub(rec.windowStart) >= l.window {
		rec = &attemptRecord{windowStart: now}
		l.attempts[key] = rec
	}
	rec.failures++
}

// Reset clears the failure count for key. Call this on a successful
// verification so a legitimate user who fumbles a code and then gets it
// right doesn't stay pinned near the limit.
func (l *AttemptLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// sweepLocked removes entries whose window has fully expired. Must be
// called with mu held. It runs at most once per sweepInterval so the cost
// is amortized across normal traffic instead of needing a dedicated
// goroutine.
func (l *AttemptLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < sweepInterval {
		return
	}
	l.lastSweep = now
	for key, rec := range l.attempts {
		if now.Sub(rec.windowStart) >= l.window {
			delete(l.attempts, key)
		}
	}
}
