package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

func TestLimiterAllowsBurstThenRefuses(t *testing.T) {
	t.Parallel()

	limiter := NewLimiter(1, 3)
	base := time.Now()
	limiter.now = func() time.Time { return base }

	for i := range 3 {
		allowed, _, _ := limiter.Allow("client:web")
		if !allowed {
			t.Fatalf("request %d refused inside the burst of 3", i+1)
		}
	}

	allowed, remaining, retryAfter := limiter.Allow("client:web")
	if allowed {
		t.Fatal("the fourth request was allowed past a burst of 3")
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
	if retryAfter <= 0 {
		t.Errorf("retryAfter = %s, want a positive delay", retryAfter)
	}
}

func TestLimiterRefillsOverTime(t *testing.T) {
	t.Parallel()

	limiter := NewLimiter(1, 1) // one per second
	base := time.Now()
	current := base
	limiter.now = func() time.Time { return current }

	if allowed, _, _ := limiter.Allow("client:web"); !allowed {
		t.Fatal("the first request was refused")
	}
	if allowed, _, _ := limiter.Allow("client:web"); allowed {
		t.Fatal("the second immediate request was allowed")
	}

	current = base.Add(time.Second)
	if allowed, _, _ := limiter.Allow("client:web"); !allowed {
		t.Fatal("a token should have refilled after a second")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	t.Parallel()

	limiter := NewLimiter(1, 1)
	if allowed, _, _ := limiter.Allow("client:a"); !allowed {
		t.Fatal("client a was refused")
	}
	if allowed, _, _ := limiter.Allow("client:b"); !allowed {
		t.Fatal("client b was refused because of client a's usage")
	}
	if allowed, _, _ := limiter.Allow("client:a"); allowed {
		t.Fatal("client a exceeded its own bucket")
	}
}

// A bucket map that only grows is an unbounded memory leak: any client can
// drive it by rotating source addresses.
func TestLimiterEvictsIdleBuckets(t *testing.T) {
	t.Parallel()

	limiter := NewLimiter(10, 10)
	base := time.Now()
	current := base
	limiter.now = func() time.Time { return current }

	for _, key := range []string{"ip:1.1.1.1", "ip:2.2.2.2", "ip:3.3.3.3"} {
		limiter.Allow(key)
	}
	if limiter.Size() != 3 {
		t.Fatalf("Size() = %d, want 3", limiter.Size())
	}

	// Past the TTL, one live key keeps its bucket and the rest are swept.
	current = base.Add(idleTTL + time.Minute)
	limiter.Allow("ip:4.4.4.4")

	if size := limiter.Size(); size != 1 {
		t.Errorf("Size() = %d, want 1 after eviction", size)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Parallel()

	resolver, err := NewIPResolver(nil)
	if err != nil {
		t.Fatalf("NewIPResolver() error = %v", err)
	}

	limiter := NewLimiter(1, 2)
	handler := Chain(okHandler(), RateLimit(limiter, IPKey(resolver)))

	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/calculate", nil)
		req.RemoteAddr = "192.0.2.10:5555"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	for i := range 2 {
		rec := request()
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i+1, rec.Code)
		}
		if rec.Header().Get(HeaderRateLimitLimit) != "2" {
			t.Errorf("limit header = %q, want 2", rec.Header().Get(HeaderRateLimitLimit))
		}
	}

	rec := request()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get(HeaderRetryAfter) == "" {
		t.Error("a 429 must carry Retry-After")
	}
	if body := decodeError(t, rec.Body.Bytes()); body.Error.Code != httpx.CodeRateLimited {
		t.Errorf("code = %q, want %q", body.Error.Code, httpx.CodeRateLimited)
	}
}

func TestSubjectKeyPrefersTheAuthenticatedClient(t *testing.T) {
	t.Parallel()

	resolver, err := NewIPResolver(nil)
	if err != nil {
		t.Fatalf("NewIPResolver() error = %v", err)
	}
	key := SubjectKey(resolver)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "192.0.2.10:5555"
	if got := key(req); got != "ip:192.0.2.10" {
		t.Errorf("unauthenticated key = %q, want the peer address", got)
	}

	req = req.WithContext(httpx.WithSubject(req.Context(), "web"))
	if got := key(req); got != "client:web" {
		t.Errorf("authenticated key = %q, want client:web", got)
	}
}

func TestIPResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		trusted   []string
		remote    string
		forwarded string
		want      string
	}{
		{
			name:   "no proxies configured",
			remote: "192.0.2.10:5555",
			want:   "192.0.2.10",
		},
		{
			// Without this, any client forges the header and walks past a
			// per-IP limit.
			name:      "forwarded header from an untrusted peer is ignored",
			remote:    "192.0.2.10:5555",
			forwarded: "1.2.3.4",
			want:      "192.0.2.10",
		},
		{
			name:      "forwarded header from a trusted proxy is honoured",
			trusted:   []string{"192.0.2.0/24"},
			remote:    "192.0.2.10:5555",
			forwarded: "1.2.3.4",
			want:      "1.2.3.4",
		},
		{
			name:      "left-most entry is the original client",
			trusted:   []string{"192.0.2.10"},
			remote:    "192.0.2.10:5555",
			forwarded: "1.2.3.4, 10.0.0.1, 10.0.0.2",
			want:      "1.2.3.4",
		},
		{
			name:      "malformed forwarded header falls back to the peer",
			trusted:   []string{"192.0.2.0/24"},
			remote:    "192.0.2.10:5555",
			forwarded: "not-an-ip",
			want:      "192.0.2.10",
		},
		{
			name:    "ipv6 peer",
			remote:  "[2001:db8::1]:5555",
			want:    "2001:db8::1",
			trusted: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolver, err := NewIPResolver(tt.trusted)
			if err != nil {
				t.Fatalf("NewIPResolver() error = %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.RemoteAddr = tt.remote
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}

			if got := resolver.ClientIP(req); got != tt.want {
				t.Errorf("ClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewIPResolverRejectsGarbage(t *testing.T) {
	t.Parallel()

	if _, err := NewIPResolver([]string{"not-a-network"}); err == nil {
		t.Fatal("NewIPResolver() error = nil, want a rejection")
	}
	if _, err := NewIPResolver([]string{"10.0.0.0/8", "", "  ", "192.0.2.1"}); err != nil {
		t.Fatalf("NewIPResolver() error = %v, want blanks tolerated", err)
	}
}
