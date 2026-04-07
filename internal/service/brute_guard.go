package service

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ─── BruteGuard ───────────────────────────────────────────────────────────────
//
// Protects against password brute-force by tracking failed login attempts
// per email address with progressive lockout:
//
//	5  failures → locked 15 minutes
//	10 failures → locked 1 hour
//	20 failures → locked 24 hours

type loginRecord struct {
	count       int
	lockedUntil time.Time
	lastFail    time.Time
}

// BruteGuard is an in-memory per-email brute-force guard.
type BruteGuard struct {
	mu      sync.Mutex
	records map[string]*loginRecord
}

func newBruteGuard() *BruteGuard {
	g := &BruteGuard{records: make(map[string]*loginRecord)}
	go g.cleanup()
	return g
}

// lockDuration returns how long to lock the account based on total failure count.
func lockDuration(count int) time.Duration {
	switch {
	case count >= 20:
		return 24 * time.Hour
	case count >= 10:
		return 1 * time.Hour
	case count >= 5:
		return 15 * time.Minute
	default:
		return 0
	}
}

// Check returns an error if the account is currently locked.
func (g *BruteGuard) Check(email string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	rec, ok := g.records[email]
	if !ok {
		return nil
	}
	if time.Now().Before(rec.lockedUntil) {
		remaining := time.Until(rec.lockedUntil).Round(time.Second)
		return fmt.Errorf("аккаунт временно заблокирован из-за многочисленных неудачных попыток входа. Повторите через %s", remaining)
	}
	return nil
}

// RecordFailure records one failed login attempt and applies lockout if threshold reached.
func (g *BruteGuard) RecordFailure(email string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	rec, ok := g.records[email]
	if !ok {
		rec = &loginRecord{}
		g.records[email] = rec
	}

	rec.count++
	rec.lastFail = time.Now()

	if dur := lockDuration(rec.count); dur > 0 && time.Now().After(rec.lockedUntil) {
		rec.lockedUntil = time.Now().Add(dur)
		slog.Warn("account locked (brute force protection)",
			"email", email,
			"attempt_count", rec.count,
			"locked_for", dur.String(),
		)
	} else {
		slog.Info("failed login attempt", "email", email, "attempt", rec.count)
	}
}

// RecordSuccess clears the failure counter after a successful login.
func (g *BruteGuard) RecordSuccess(email string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.records, email)
	slog.Info("successful login, attempts cleared", "email", email)
}

// AttemptsLeft returns how many more failures are allowed before the next lockout.
func (g *BruteGuard) AttemptsLeft(email string) int {
	g.mu.Lock()
	defer g.mu.Unlock()

	rec, ok := g.records[email]
	if !ok {
		return 5 // fresh account
	}
	// Calculate next threshold
	var nextThreshold int
	switch {
	case rec.count >= 10:
		nextThreshold = 20
	case rec.count >= 5:
		nextThreshold = 10
	default:
		nextThreshold = 5
	}
	left := nextThreshold - rec.count
	if left < 0 {
		return 0
	}
	return left
}

// cleanup removes stale records every 10 minutes.
func (g *BruteGuard) cleanup() {
	for range time.Tick(10 * time.Minute) {
		g.mu.Lock()
		now := time.Now()
		for email, rec := range g.records {
			// Drop records that are unlocked AND not touched in 24h
			if now.After(rec.lockedUntil) && now.Sub(rec.lastFail) > 24*time.Hour {
				delete(g.records, email)
			}
		}
		g.mu.Unlock()
	}
}
