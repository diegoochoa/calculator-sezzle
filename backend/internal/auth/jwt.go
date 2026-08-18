// Package auth issues and verifies the bearer tokens that gate the calculation
// routes, and authenticates the clients allowed to ask for one.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned for every verification failure. Callers must not
// tell a client which check failed — expiry, signature and issuer are all the
// same answer, or the error message becomes an oracle.
var ErrInvalidToken = errors.New("invalid token")

// signingMethod is pinned. Accepting whatever the token's header claims is the
// classic JWT vulnerability: "none" forges freely, and an RS256 token verified
// as HS256 turns a public key into a signing key.
var signingMethod = jwt.SigningMethodHS256

// Token is a freshly issued credential.
type Token struct {
	Value     string
	ExpiresIn int
	ExpiresAt time.Time
}

// Issuer mints and validates HS256 tokens.
type Issuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
	// now is injectable so expiry is testable without sleeping.
	now func() time.Time
}

// NewIssuer builds an Issuer. The secret length is validated at config load.
func NewIssuer(secret []byte, issuer string, ttl time.Duration) *Issuer {
	return &Issuer{secret: secret, issuer: issuer, ttl: ttl, now: time.Now}
}

// Issue mints a token for a client.
func (i *Issuer) Issue(subject string) (Token, error) {
	issuedAt := i.now()
	expiresAt := issuedAt.Add(i.ttl)

	claims := jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    i.issuer,
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		NotBefore: jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		ID:        newTokenID(),
	}

	signed, err := jwt.NewWithClaims(signingMethod, claims).SignedString(i.secret)
	if err != nil {
		return Token{}, err
	}

	return Token{
		Value:     signed,
		ExpiresIn: int(i.ttl.Seconds()),
		ExpiresAt: expiresAt,
	}, nil
}

// Verify checks a token and returns the client it was issued to.
func (i *Issuer) Verify(raw string) (string, error) {
	claims := &jwt.RegisteredClaims{}

	_, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return i.secret, nil
	},
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(i.now),
	)
	if err != nil {
		return "", ErrInvalidToken
	}
	if claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
