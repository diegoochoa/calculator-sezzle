package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/diegoochoa/calculator-sezzle-api/internal/auth"
	"github.com/diegoochoa/calculator-sezzle-api/internal/calc"
	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
	"github.com/diegoochoa/calculator-sezzle-api/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

const (
	testClientID     = "web"
	testClientSecret = "dev-secret"
	testJWTSecret    = "test-secret-that-is-long-enough-32"
)

type harness struct {
	handler http.Handler
	token   string
}

type harnessOptions struct {
	maxBodyBytes int64
	maxBatch     int
	apiLimiter   *middleware.Limiter
	authLimiter  *middleware.Limiter
}

func newHarness(t *testing.T, configure ...func(*harnessOptions)) *harness {
	t.Helper()

	options := harnessOptions{
		maxBodyBytes: 8 * 1024,
		maxBatch:     5,
		// Generous by default so unrelated tests never trip the limiter.
		apiLimiter:  middleware.NewLimiter(1000, 1000),
		authLimiter: middleware.NewLimiter(1000, 1000),
	}
	for _, apply := range configure {
		apply(&options)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(testClientSecret), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing the test secret: %v", err)
	}

	issuer := auth.NewIssuer([]byte(testJWTSecret), "test-issuer", 15*time.Minute)
	clients := auth.NewStore([]auth.Credential{{ID: testClientID, SecretHash: hash}})

	resolver, err := middleware.NewIPResolver(nil)
	if err != nil {
		t.Fatalf("NewIPResolver() error = %v", err)
	}

	server := NewServer(ServerConfig{
		CalcOptions: calc.Options{MaxLength: 256, MaxDepth: 32, Precision: calc.Precision},
		MaxBatch:    options.maxBatch,
		Issuer:      issuer,
		Clients:     clients,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version:     "test",
	})

	handler := NewRouter(RouterConfig{
		Server:       server,
		Verifier:     issuer,
		APILimiter:   options.apiLimiter,
		AuthLimiter:  options.authLimiter,
		IPResolver:   resolver,
		CORSOrigins:  []string{"http://localhost:5173"},
		MaxBodyBytes: options.maxBodyBytes,
	})

	token, err := issuer.Issue(testClientID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	return &harness{handler: handler, token: token.Value}
}

// do sends a request with the harness token attached.
func (h *harness) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return h.doWithToken(t, method, path, body, h.token)
}

func (h *harness) doWithToken(t *testing.T, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.RemoteAddr = "192.0.2.10:5555"

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func decodeInto[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var target T
	if err := json.Unmarshal(rec.Body.Bytes(), &target); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, rec.Body.String())
	}
	return target
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) httpx.ErrorResponse {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	body := decodeInto[httpx.ErrorResponse](t, rec)
	if body.Error.Code != code {
		t.Errorf("code = %q, want %q (message %q)", body.Error.Code, code, body.Error.Message)
	}
	if body.RequestID == "" {
		t.Error("the error envelope has no requestId, so a report cannot be traced")
	}
	return body
}

func TestCalculate(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	tests := []struct {
		name          string
		body          string
		wantResult    float64
		wantFormatted string
	}{
		{
			name:          "precedence, which the frontend engine does not have",
			body:          `{"expression":"2 + 3 * 4"}`,
			wantResult:    14,
			wantFormatted: "14",
		},
		{
			name:          "parentheses",
			body:          `{"expression":"(2 + 3) * 4"}`,
			wantResult:    20,
			wantFormatted: "20",
		},
		{
			name:          "float drift is absent",
			body:          `{"expression":"0.1 + 0.2"}`,
			wantResult:    0.3,
			wantFormatted: "0.3",
		},
		{
			name:          "square root",
			body:          `{"expression":"\u221a9 + 1"}`,
			wantResult:    4,
			wantFormatted: "4",
		},
		{
			name:          "precision is honoured",
			body:          `{"expression":"1/3","precision":4}`,
			wantResult:    0.333333333333,
			wantFormatted: "0.3333",
		},
		{
			name:          "unicode operators from the UI",
			body:          `{"expression":"6 ÷ 2 × 3"}`,
			wantResult:    9,
			wantFormatted: "9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := h.do(t, http.MethodPost, "/v1/calculate", tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
			}

			body := decodeInto[calculateResponse](t, rec)
			if body.Result != tt.wantResult {
				t.Errorf("result = %v, want %v", body.Result, tt.wantResult)
			}
			if body.Formatted != tt.wantFormatted {
				t.Errorf("formatted = %q, want %q", body.Formatted, tt.wantFormatted)
			}
		})
	}
}

func TestCalculateEngineErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	tests := []struct {
		name         string
		body         string
		wantCode     string
		wantPosition *int
	}{
		{name: "division by zero", body: `{"expression":"1/0"}`, wantCode: calc.CodeDivisionByZero, wantPosition: ptr(1)},
		{name: "nested division by zero", body: `{"expression":"100 + 1/0"}`, wantCode: calc.CodeDivisionByZero, wantPosition: ptr(7)},
		{name: "syntax error", body: `{"expression":"2 +"}`, wantCode: calc.CodeSyntax},
		{name: "unbalanced parenthesis", body: `{"expression":"(1+2"}`, wantCode: calc.CodeUnbalancedParen},
		{name: "unknown name", body: `{"expression":"foo"}`, wantCode: calc.CodeUnknownIdentifier},
		{name: "wrong arity", body: `{"expression":"sqrt(1,2)"}`, wantCode: calc.CodeWrongArity},
		{name: "domain error", body: `{"expression":"sqrt(-1)"}`, wantCode: calc.CodeDomain},
		{name: "overflow", body: `{"expression":"10 ^ 400"}`, wantCode: calc.CodeOverflow},
		{name: "empty expression", body: `{"expression":"  "}`, wantCode: calc.CodeEmpty},
		{name: "missing expression field", body: `{}`, wantCode: calc.CodeEmpty},
		{name: "expression too long", body: `{"expression":"` + strings.Repeat("1+", 200) + `1"}`, wantCode: calc.CodeTooLong},
		{name: "nesting too deep", body: `{"expression":"` + strings.Repeat("(", 60) + "1" + strings.Repeat(")", 60) + `"}`, wantCode: calc.CodeDepthExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := h.do(t, http.MethodPost, "/v1/calculate", tt.body)
			body := assertError(t, rec, http.StatusUnprocessableEntity, tt.wantCode)

			if tt.wantPosition != nil {
				if body.Error.Position == nil {
					t.Fatalf("position = nil, want %d", *tt.wantPosition)
				}
				if *body.Error.Position != *tt.wantPosition {
					t.Errorf("position = %d, want %d", *body.Error.Position, *tt.wantPosition)
				}
			}
		})
	}
}

func ptr[T any](value T) *T { return &value }

func TestCalculateRequestErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"malformed JSON", `{"expression":`, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"not JSON at all", `hello`, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"empty body", ``, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"wrong field type", `{"expression":5}`, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"unknown field", `{"expresion":"1+1"}`, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"two JSON objects", `{"expression":"1+1"}{"expression":"2+2"}`, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"precision too low", `{"expression":"1+1","precision":-1}`, http.StatusBadRequest, httpx.CodeInvalidRequest},
		{"precision too high", `{"expression":"1+1","precision":99}`, http.StatusBadRequest, httpx.CodeInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertError(t, h.do(t, http.MethodPost, "/v1/calculate", tt.body), tt.wantStatus, tt.wantCode)
		})
	}
}

func TestCalculateRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	h := newHarness(t, func(o *harnessOptions) { o.maxBodyBytes = 64 })
	body := `{"expression":"` + strings.Repeat("1+", 100) + `1"}`

	assertError(t, h.do(t, http.MethodPost, "/v1/calculate", body),
		http.StatusRequestEntityTooLarge, httpx.CodePayloadTooLarge)
}

func TestBatch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	rec := h.do(t, http.MethodPost, "/v1/calculate/batch",
		`{"expressions":["1+1","1/0","2 * 3","2 +"]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — a partial failure must not discard the successes", rec.Code)
	}

	body := decodeInto[batchResponse](t, rec)
	if len(body.Results) != 4 {
		t.Fatalf("got %d results, want 4", len(body.Results))
	}

	if body.Results[0].Result == nil || *body.Results[0].Result != 2 {
		t.Errorf("result 0 = %+v, want 2", body.Results[0])
	}
	if body.Results[1].Error == nil || body.Results[1].Error.Code != calc.CodeDivisionByZero {
		t.Errorf("result 1 = %+v, want a division by zero", body.Results[1])
	}
	if body.Results[2].Result == nil || *body.Results[2].Result != 6 {
		t.Errorf("result 2 = %+v, want 6", body.Results[2])
	}
	if body.Results[3].Error == nil || body.Results[3].Error.Code != calc.CodeSyntax {
		t.Errorf("result 3 = %+v, want a syntax error", body.Results[3])
	}
}

func TestBatchLimits(t *testing.T) {
	t.Parallel()
	h := newHarness(t, func(o *harnessOptions) { o.maxBatch = 3 })

	assertError(t, h.do(t, http.MethodPost, "/v1/calculate/batch", `{"expressions":[]}`),
		http.StatusBadRequest, httpx.CodeInvalidRequest)

	assertError(t, h.do(t, http.MethodPost, "/v1/calculate/batch",
		`{"expressions":["1","2","3","4"]}`),
		http.StatusBadRequest, httpx.CodeInvalidRequest)
}

func TestValidate(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	tests := []struct {
		name      string
		body      string
		wantValid bool
		wantCode  string
	}{
		{"well formed", `{"expression":"2 + 3 * 4"}`, true, ""},
		// Parse-only: this is what lets a UI check syntax on every keystroke.
		{"evaluation would fail but the syntax is fine", `{"expression":"1/0"}`, true, ""},
		{"trailing operator", `{"expression":"1 +"}`, false, calc.CodeSyntax},
		{"unknown name", `{"expression":"wat"}`, false, calc.CodeUnknownIdentifier},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := h.do(t, http.MethodPost, "/v1/validate", tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 — validity is the answer, not the status", rec.Code)
			}

			body := decodeInto[validateResponse](t, rec)
			if body.Valid != tt.wantValid {
				t.Fatalf("valid = %t, want %t (%+v)", body.Valid, tt.wantValid, body.Error)
			}
			if tt.wantCode != "" && (body.Error == nil || body.Error.Code != tt.wantCode) {
				t.Errorf("error = %+v, want code %s", body.Error, tt.wantCode)
			}
		})
	}
}

func TestFunctionsCatalog(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	rec := h.do(t, http.MethodGet, "/v1/functions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	catalog := decodeInto[calc.Catalog](t, rec)
	if len(catalog.Functions) == 0 || len(catalog.Operators) == 0 {
		t.Fatalf("catalog is incomplete: %+v", catalog)
	}
	if catalog.MaxLength != 256 || catalog.MaxDepth != 32 {
		t.Errorf("catalog limits = (%d, %d), want the server's configured limits", catalog.MaxLength, catalog.MaxDepth)
	}

	names := map[string]bool{}
	for _, fn := range catalog.Functions {
		names[fn.Name] = true
	}
	for _, want := range []string{"sqrt", "abs"} {
		if !names[want] {
			t.Errorf("catalog is missing %q", want)
		}
	}
}

func TestTokenEndpoint(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	t.Run("valid credentials mint a usable token", func(t *testing.T) {
		t.Parallel()

		rec := h.doWithToken(t, http.MethodPost, "/v1/auth/token",
			`{"clientId":"web","clientSecret":"dev-secret"}`, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
		}

		body := decodeInto[tokenResponse](t, rec)
		if body.Token == "" || body.TokenType != "Bearer" || body.ExpiresIn != 900 {
			t.Fatalf("token response = %+v", body)
		}
		// Credentials must not be cached by a proxy or the browser.
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}

		calculated := h.doWithToken(t, http.MethodPost, "/v1/calculate", `{"expression":"1+1"}`, body.Token)
		if calculated.Code != http.StatusOK {
			t.Errorf("the minted token was rejected: %d %s", calculated.Code, calculated.Body.String())
		}
	})

	rejections := []struct {
		name string
		body string
	}{
		{"unknown client", `{"clientId":"ghost","clientSecret":"dev-secret"}`},
		{"wrong secret", `{"clientId":"web","clientSecret":"nope"}`},
		{"missing secret", `{"clientId":"web"}`},
		{"missing id", `{"clientSecret":"dev-secret"}`},
		{"empty object", `{}`},
	}

	var messages []string
	for _, tt := range rejections {
		t.Run(tt.name, func(t *testing.T) {
			rec := h.doWithToken(t, http.MethodPost, "/v1/auth/token", tt.body, "")
			body := assertError(t, rec, http.StatusUnauthorized, httpx.CodeUnauthorized)
			messages = append(messages, body.Error.Message)
		})
	}

	// Every rejection must read the same, or the message becomes an oracle for
	// which client ids exist.
	for _, message := range messages {
		if message != messages[0] {
			t.Errorf("rejection messages differ (%q vs %q); that distinguishes a bad id from a bad secret",
				message, messages[0])
		}
	}
}

func TestAuthenticationIsRequired(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	protected := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/calculate", `{"expression":"1+1"}`},
		{http.MethodPost, "/v1/calculate/batch", `{"expressions":["1+1"]}`},
		{http.MethodPost, "/v1/validate", `{"expression":"1+1"}`},
		{http.MethodGet, "/v1/functions", ""},
	}

	for _, tt := range protected {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			rec := h.doWithToken(t, tt.method, tt.path, tt.body, "")
			assertError(t, rec, http.StatusUnauthorized, httpx.CodeUnauthorized)

			if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Bearer") {
				t.Errorf("WWW-Authenticate = %q", got)
			}
		})
	}
}

func TestProbesNeedNoCredentials(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	for _, path := range []string{"/healthz", "/readyz"} {
		rec := h.doWithToken(t, http.MethodGet, path, "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d, want 200", path, rec.Code)
		}
		body := decodeInto[healthResponse](t, rec)
		if body.Status == "" || body.Version != "test" {
			t.Errorf("%s body = %+v", path, body)
		}
	}
}

func TestRoutingErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	t.Run("unknown path", func(t *testing.T) {
		t.Parallel()
		assertError(t, h.do(t, http.MethodGet, "/v1/nope", ""), http.StatusNotFound, httpx.CodeNotFound)
	})

	t.Run("wrong method returns the envelope, not plain text", func(t *testing.T) {
		t.Parallel()

		rec := h.do(t, http.MethodGet, "/v1/calculate", "")
		assertError(t, rec, http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed)

		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, http.MethodPost) {
			t.Errorf("Allow = %q, want it to list POST", allow)
		}
	})
}

func TestRateLimitReturns429(t *testing.T) {
	t.Parallel()

	h := newHarness(t, func(o *harnessOptions) {
		o.apiLimiter = middleware.NewLimiter(1, 2)
	})

	for i := range 2 {
		if rec := h.do(t, http.MethodPost, "/v1/calculate", `{"expression":"1+1"}`); rec.Code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i+1, rec.Code)
		}
	}

	rec := h.do(t, http.MethodPost, "/v1/calculate", `{"expression":"1+1"}`)
	assertError(t, rec, http.StatusTooManyRequests, httpx.CodeRateLimited)

	if rec.Header().Get(middleware.HeaderRetryAfter) == "" {
		t.Error("a 429 must carry Retry-After")
	}
}

// The token route is limited separately and more tightly, because bcrypt makes
// it the expensive path and the obvious brute-force target.
func TestTokenRouteHasItsOwnLimit(t *testing.T) {
	t.Parallel()

	h := newHarness(t, func(o *harnessOptions) {
		o.authLimiter = middleware.NewLimiter(1, 1)
	})

	first := h.doWithToken(t, http.MethodPost, "/v1/auth/token",
		`{"clientId":"web","clientSecret":"dev-secret"}`, "")
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", first.Code)
	}

	second := h.doWithToken(t, http.MethodPost, "/v1/auth/token",
		`{"clientId":"web","clientSecret":"dev-secret"}`, "")
	assertError(t, second, http.StatusTooManyRequests, httpx.CodeRateLimited)

	// The calculation limiter is untouched by traffic on the token route.
	if rec := h.do(t, http.MethodPost, "/v1/calculate", `{"expression":"1+1"}`); rec.Code != http.StatusOK {
		t.Errorf("calculate = %d, want 200; the two limiters must be independent", rec.Code)
	}
}

func TestResponsesAreJSON(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	cases := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/v1/calculate", `{"expression":"1+1"}`},
		{http.MethodPost, "/v1/calculate", `{"expression":"1/0"}`},
		{http.MethodGet, "/healthz", ""},
		{http.MethodGet, "/v1/nope", ""},
	}

	for _, tt := range cases {
		rec := h.do(t, tt.method, tt.path, tt.body)
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("%s %s Content-Type = %q", tt.method, tt.path, got)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s %s is missing the nosniff header", tt.method, tt.path)
		}
		if !json.Valid(rec.Body.Bytes()) {
			t.Errorf("%s %s body is not valid JSON: %s", tt.method, tt.path, rec.Body.String())
		}
	}
}

func TestRequestIDIsEchoed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	rec := h.do(t, http.MethodPost, "/v1/calculate", `{"expression":"1+1"}`)
	if rec.Header().Get(middleware.HeaderRequestID) == "" {
		t.Error("every response should carry a request id")
	}
}
