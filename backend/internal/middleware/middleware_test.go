package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

func decodeError(t *testing.T, body []byte) httpx.ErrorResponse {
	t.Helper()
	var parsed httpx.ErrorResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not the error envelope: %v (%s)", err, body)
	}
	return parsed
}

func TestChainAppliesOutermostFirst(t *testing.T) {
	t.Parallel()

	var order []string
	tag := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	handler := Chain(okHandler(), tag("first"), tag("second"), tag("third"))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"first", "second", "third"}
	for i, name := range want {
		if order[i] != name {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inbound  string
		wantEcho string
		generate bool
	}{
		{name: "generates when absent", generate: true},
		{name: "honours a safe inbound id", inbound: "trace-abc_123", wantEcho: "trace-abc_123"},
		{name: "replaces an id with a newline", inbound: "abc\ndef", generate: true},
		{name: "replaces an id with a space", inbound: "abc def", generate: true},
		{name: "replaces an overlong id", inbound: strings.Repeat("a", 65), generate: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var seen string
			handler := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = httpx.RequestID(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.inbound != "" {
				req.Header.Set(HeaderRequestID, tt.inbound)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			echoed := rec.Header().Get(HeaderRequestID)
			if echoed == "" || echoed != seen {
				t.Fatalf("header %q and context %q disagree", echoed, seen)
			}
			if tt.generate {
				if echoed == tt.inbound {
					t.Errorf("unsafe inbound id %q was echoed", tt.inbound)
				}
			} else if echoed != tt.wantEcho {
				t.Errorf("id = %q, want %q", echoed, tt.wantEcho)
			}
		})
	}
}

func TestRecoverTurnsPanicIntoEnvelope(t *testing.T) {
	t.Parallel()

	handler := Chain(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }),
		Recover(discardLogger()),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := decodeError(t, rec.Body.Bytes())
	if body.Error.Code != httpx.CodeInternal {
		t.Errorf("code = %q, want %q", body.Error.Code, httpx.CodeInternal)
	}
	// The panic value must not reach the client.
	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("the panic value leaked into the response")
	}
}

// Once bytes are on the wire the status is already sent; recovery must not try
// to write a second header.
func TestRecoverAfterPartialWrite(t *testing.T) {
	t.Parallel()

	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"partial":`))
			panic("boom")
		}),
		Recover(discardLogger()),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want the already-sent 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), httpx.CodeInternal) {
		t.Error("an error envelope was appended to a partially written body")
	}
}

func TestLoggingRecordsStatus(t *testing.T) {
	t.Parallel()

	var sink strings.Builder
	logger := slog.New(slog.NewTextHandler(&sink, nil))

	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}),
		Logging(logger),
	)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/calculate", nil))

	logged := sink.String()
	for _, want := range []string{"status=418", "method=POST", "/v1/calculate"} {
		if !strings.Contains(logged, want) {
			t.Errorf("log line %q missing %q", logged, want)
		}
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()

	handler := Chain(okHandler(), CORS([]string{"http://localhost:5173"}))

	t.Run("allowed origin is echoed", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodPost, "/v1/calculate", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Errorf("allow-origin = %q", got)
		}
		if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
			t.Error("Vary must include Origin so caches do not cross origins")
		}
	})

	t.Run("unknown origin gets no allowance", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodPost, "/v1/calculate", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("allow-origin = %q, want empty", got)
		}
	})

	t.Run("preflight is answered without reaching the handler", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodOptions, "/v1/calculate", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
			t.Error("preflight must permit the Authorization header")
		}
	})

	t.Run("credentials are never allowed", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodPost, "/v1/calculate", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Error("the API authenticates by header, so credentials must stay disallowed")
		}
	})
}

func TestCORSWildcard(t *testing.T) {
	t.Parallel()

	handler := Chain(okHandler(), CORS([]string{"*"}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://anywhere.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allow-origin = %q, want *", got)
	}
}

func TestBodyLimit(t *testing.T) {
	t.Parallel()

	var readErr error
	handler := Chain(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			_, readErr = io.ReadAll(r.Body)
		}),
		BodyLimit(16),
	)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 64)))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var maxBytes *http.MaxBytesError
	if !errors.As(readErr, &maxBytes) {
		t.Fatalf("read error = %v, want *http.MaxBytesError", readErr)
	}
}

type fakeVerifier struct {
	subject string
	err     error
}

func (f fakeVerifier) Verify(string) (string, error) { return f.subject, f.err }

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	t.Run("valid token reaches the handler with a subject", func(t *testing.T) {
		t.Parallel()

		var subject string
		handler := Chain(
			http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				subject = httpx.Subject(r.Context())
			}),
			Authenticate(fakeVerifier{subject: "web"}),
		)

		req := httptest.NewRequest(http.MethodPost, "/v1/calculate", nil)
		req.Header.Set("Authorization", "Bearer anything")
		handler.ServeHTTP(httptest.NewRecorder(), req)

		if subject != "web" {
			t.Errorf("subject = %q, want web", subject)
		}
	})

	tests := []struct {
		name     string
		header   string
		verifier Verifier
	}{
		{name: "no header", verifier: fakeVerifier{subject: "web"}},
		{name: "wrong scheme", header: "Basic abc", verifier: fakeVerifier{subject: "web"}},
		{name: "bearer with no credential", header: "Bearer   ", verifier: fakeVerifier{subject: "web"}},
		{name: "rejected token", header: "Bearer abc", verifier: fakeVerifier{err: errors.New("nope")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reached := false
			handler := Chain(
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }),
				Authenticate(tt.verifier),
			)

			req := httptest.NewRequest(http.MethodPost, "/v1/calculate", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if reached {
				t.Fatal("the handler ran for an unauthenticated request")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Bearer") {
				t.Errorf("WWW-Authenticate = %q", got)
			}
			if body := decodeError(t, rec.Body.Bytes()); body.Error.Code != httpx.CodeUnauthorized {
				t.Errorf("code = %q", body.Error.Code)
			}
		})
	}
}

// The scheme is case-insensitive per RFC 7235, and clients do vary it.
func TestAuthenticateSchemeIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	for _, scheme := range []string{"Bearer", "bearer", "BEARER"} {
		reached := false
		handler := Chain(
			http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }),
			Authenticate(fakeVerifier{subject: "web"}),
		)

		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Authorization", scheme+" token")
		handler.ServeHTTP(httptest.NewRecorder(), req)

		if !reached {
			t.Errorf("scheme %q was rejected", scheme)
		}
	}
}
