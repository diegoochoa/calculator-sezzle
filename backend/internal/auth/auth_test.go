package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "test-secret-that-is-long-enough-32"

func newTestIssuer(t *testing.T) *Issuer {
	t.Helper()
	return NewIssuer([]byte(testSecret), "test-issuer", 15*time.Minute)
}

func TestIssueThenVerify(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)

	token, err := issuer.Issue("web")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token.Value == "" {
		t.Fatal("Issue() returned an empty token")
	}
	if token.ExpiresIn != 900 {
		t.Errorf("ExpiresIn = %d, want 900", token.ExpiresIn)
	}

	subject, err := issuer.Verify(token.Value)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if subject != "web" {
		t.Errorf("subject = %q, want web", subject)
	}
}

func TestVerifyRejects(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	valid, err := issuer.Issue("web")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	otherSecret := NewIssuer([]byte("a-completely-different-secret-3232"), "test-issuer", time.Minute)
	foreign, err := otherSecret.Issue("web")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	otherIssuer := NewIssuer([]byte(testSecret), "someone-else", time.Minute)
	wrongIssuer, err := otherIssuer.Issue("web")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"not a jwt", "nonsense"},
		{"tampered payload", valid.Value[:len(valid.Value)-4] + "AAAA"},
		{"signed with another secret", foreign.Value},
		{"issued by someone else", wrongIssuer.Value},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := issuer.Verify(tt.token); err == nil {
				t.Fatal("Verify() error = nil, want a rejection")
			}
		})
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	base := time.Now()
	issuer.now = func() time.Time { return base }

	token, err := issuer.Issue("web")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	// Still valid a minute later, gone sixteen minutes later.
	issuer.now = func() time.Time { return base.Add(time.Minute) }
	if _, err := issuer.Verify(token.Value); err != nil {
		t.Fatalf("Verify() error = %v, want the token to still be valid", err)
	}

	issuer.now = func() time.Time { return base.Add(16 * time.Minute) }
	if _, err := issuer.Verify(token.Value); err == nil {
		t.Fatal("Verify() error = nil, want the expired token rejected")
	}
}

// The classic JWT break: a token asking to be verified with "none", or an
// asymmetric algorithm whose public key is treated as an HMAC secret.
func TestVerifyRejectsUnexpectedAlgorithm(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)

	claims := jwt.RegisteredClaims{
		Subject:   "web",
		Issuer:    "test-issuer",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}

	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("building the unsigned token: %v", err)
	}

	if _, err := issuer.Verify(unsigned); err == nil {
		t.Fatal("Verify() accepted an alg=none token")
	}
}

// A token with a valid signature but no subject identifies nobody, so the rate
// limiter would have nothing to key on.
func TestVerifyRejectsSubjectlessToken(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	claims := jwt.RegisteredClaims{
		Issuer:    "test-issuer",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	signed, err := jwt.NewWithClaims(signingMethod, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	if _, err := issuer.Verify(signed); err == nil {
		t.Fatal("Verify() accepted a token with no subject")
	}
}

func TestIssuedTokensAreUnique(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	seen := map[string]bool{}

	for range 5 {
		token, err := issuer.Issue("web")
		if err != nil {
			t.Fatalf("Issue() error = %v", err)
		}
		if seen[token.Value] {
			t.Fatal("Issue() returned a duplicate token; the jti is not random")
		}
		seen[token.Value] = true
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("dev-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	return NewStore([]Credential{{ID: "web", SecretHash: hash}})
}

func TestStoreAuthenticate(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	tests := []struct {
		name   string
		id     string
		secret string
		want   bool
	}{
		{"correct credentials", "web", "dev-secret", true},
		{"wrong secret", "web", "nope", false},
		{"unknown client", "ghost", "dev-secret", false},
		{"empty id", "", "dev-secret", false},
		{"empty secret", "web", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := store.Authenticate(tt.id, tt.secret); got != tt.want {
				t.Errorf("Authenticate(%q, %q) = %t, want %t", tt.id, tt.secret, got, tt.want)
			}
		})
	}

	if store.Len() != 1 {
		t.Errorf("Len() = %d, want 1", store.Len())
	}
}

func TestStoreDoesNotLeakClientIDsInErrors(t *testing.T) {
	t.Parallel()

	// Authenticate returns a bare bool by design: there is no error string that
	// could distinguish "no such client" from "wrong secret".
	store := newTestStore(t)
	if store.Authenticate("ghost", "x") || store.Authenticate("web", "x") {
		t.Fatal("Authenticate() should have failed both ways")
	}
}

func TestNewTokenIDIsRandom(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for range 100 {
		id := newTokenID()
		if len(id) != 32 {
			t.Fatalf("newTokenID() = %q, want 32 hex characters", id)
		}
		if !strings.ContainsAny(id, "0123456789abcdef") || seen[id] {
			t.Fatalf("newTokenID() returned a repeat or malformed value: %q", id)
		}
		seen[id] = true
	}
}
