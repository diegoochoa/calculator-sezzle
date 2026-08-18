package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
	"golang.org/x/time/rate"
)

// Rate limit headers, following the widely implemented draft conventions.
const (
	HeaderRateLimitLimit     = "X-RateLimit-Limit"
	HeaderRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderRetryAfter         = "Retry-After"
)

// idleTTL is how long an unused bucket is kept before eviction.
const idleTTL = 10 * time.Minute

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter is a per-key token bucket.
//
// The bucket map is swept on a TTL. A map that only ever grows is the standard
// bug in this pattern: with per-IP keys it is an unbounded memory leak that any
// client can drive by rotating source addresses.
type Limiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time

	rps   rate.Limit
	burst int
	ttl   time.Duration

	// now is injectable so eviction is testable without sleeping.
	now func() time.Time
}

// NewLimiter builds a limiter allowing rps sustained requests with the given
// burst.
func NewLimiter(rps float64, burst int) *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		rps:     rate.Limit(rps),
		burst:   burst,
		ttl:     idleTTL,
		now:     time.Now,
	}
}

// Burst reports the configured burst, reported as the limit header.
func (l *Limiter) Burst() int { return l.burst }

// Allow consumes a token for key. When refused it reports how long the caller
// should wait.
func (l *Limiter) Allow(key string) (allowed bool, remaining int, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)

	entry, ok := l.buckets[key]
	if !ok {
		entry = &bucket{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.buckets[key] = entry
	}
	entry.lastSeen = now

	// Reserve rather than Allow, so a refusal can report a real delay. The
	// reservation is cancelled when it cannot be honoured immediately.
	reservation := entry.limiter.ReserveN(now, 1)
	if !reservation.OK() {
		return false, 0, time.Second
	}
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.CancelAt(now)
		return false, 0, delay
	}

	return true, int(entry.limiter.TokensAt(now)), 0
}

// sweepLocked drops buckets nobody has touched for a TTL. Called under the
// mutex, at most once per TTL, so the cost is amortised to nothing.
func (l *Limiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < l.ttl {
		return
	}
	l.lastSweep = now

	for key, entry := range l.buckets {
		if now.Sub(entry.lastSeen) > l.ttl {
			delete(l.buckets, key)
		}
	}
}

// Size reports how many buckets are held, for tests and diagnostics.
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// KeyFunc derives the rate limit key from a request.
type KeyFunc func(*http.Request) string

// SubjectKey limits per authenticated client. It falls back to the peer address
// so an unauthenticated route is still bounded.
func SubjectKey(resolver *IPResolver) KeyFunc {
	return func(r *http.Request) string {
		if subject := httpx.Subject(r.Context()); subject != "" {
			return "client:" + subject
		}
		return "ip:" + resolver.ClientIP(r)
	}
}

// IPKey limits per source address, for routes that run before authentication.
func IPKey(resolver *IPResolver) KeyFunc {
	return func(r *http.Request) string { return "ip:" + resolver.ClientIP(r) }
}

// RateLimit refuses traffic above the configured rate with a 429 in the standard
// envelope.
func RateLimit(limiter *Limiter, key KeyFunc) Middleware {
	limit := strconv.Itoa(limiter.Burst())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, remaining, retryAfter := limiter.Allow(key(r))

			w.Header().Set(HeaderRateLimitLimit, limit)
			w.Header().Set(HeaderRateLimitRemaining, strconv.Itoa(remaining))

			if !allowed {
				seconds := int(retryAfter.Seconds())
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set(HeaderRetryAfter, strconv.Itoa(seconds))
				httpx.WriteError(w, r, http.StatusTooManyRequests, httpx.CodeRateLimited,
					"Too many requests, please slow down")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
