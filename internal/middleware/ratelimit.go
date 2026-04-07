package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── RateLimiter ──────────────────────────────────────────────────────────────

// RateLimiter is a per-IP sliding-window rate limiter with automatic IP blocking
// for repeat offenders — protects against DDoS and brute-force attacks.
type RateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	blocked    map[string]time.Time // IP → blocked until
	rate       int
	window     time.Duration
	blockAfter int           // block after this many consecutive window violations
	blockFor   time.Duration // how long to block the IP
}

type bucket struct {
	count   int
	resetAt time.Time
	strikes int // times the limit was exceeded within the window
}

// NewRateLimiter creates a new limiter: maxRequests per window.
// After 5 consecutive violations the IP is blocked for 15 minutes.
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets:    make(map[string]*bucket),
		blocked:    make(map[string]time.Time),
		rate:       maxRequests,
		window:     window,
		blockAfter: 5,
		blockFor:   15 * time.Minute,
	}
	go rl.cleanup()
	return rl
}

// Allow reports whether the given IP may proceed.
// Returns (true, 0) on success, or (false, retryAfter) when rejected.
func (rl *RateLimiter) Allow(ip string) (ok bool, retryAfter time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Check permanent block list
	if until, isBlocked := rl.blocked[ip]; isBlocked {
		if time.Now().Before(until) {
			return false, time.Until(until)
		}
		delete(rl.blocked, ip) // block expired
	}

	b, exists := rl.buckets[ip]
	if !exists || time.Now().After(b.resetAt) {
		rl.buckets[ip] = &bucket{count: 1, resetAt: time.Now().Add(rl.window)}
		return true, 0
	}

	b.count++
	if b.count > rl.rate {
		b.strikes++
		if b.strikes >= rl.blockAfter {
			rl.blocked[ip] = time.Now().Add(rl.blockFor)
			slog.Warn("IP auto-blocked (DDoS protection)",
				"ip", ip, "strikes", b.strikes, "block_minutes", int(rl.blockFor.Minutes()))
			return false, rl.blockFor
		}
		return false, time.Until(b.resetAt)
	}
	return true, 0
}

// Limit returns an http.Handler middleware that enforces rate limiting.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		ok, retryAfter := rl.Allow(ip)
		if !ok {
			secs := int(retryAfter.Seconds())
			if secs < 1 {
				secs = 60
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.rate))
			http.Error(w, "429 Too Many Requests — подождите перед повторным запросом", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) cleanup() {
	for range time.Tick(5 * time.Minute) {
		rl.mu.Lock()
		now := time.Now()
		for ip, b := range rl.buckets {
			if now.After(b.resetAt) {
				delete(rl.buckets, ip)
			}
		}
		for ip, until := range rl.blocked {
			if now.After(until) {
				slog.Info("IP block expired", "ip", ip)
				delete(rl.blocked, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// ─── MaxBodySize ──────────────────────────────────────────────────────────────

// MaxBodySize is middleware that limits request body to maxBytes.
// Blocks oversized payloads that could exhaust server memory (DDoS).
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				http.Error(w, "413 Request Too Large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// ─── clientIP helper ──────────────────────────────────────────────────────────

// clientIP extracts the real client IP, respecting X-Forwarded-For from proxies.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the forwarded list
		if i := strings.IndexByte(xff, ','); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
