// Package config loads and validates the server configuration from the
// environment. Everything is validated at boot so the process fails fast and
// loudly rather than misbehaving under load.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Env values.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// MinJWTSecretLength is the shortest HS256 secret we accept. Shorter secrets
// are brute-forceable offline once an attacker holds a single token.
const MinJWTSecretLength = 32

// Getenv reads one environment variable. Injected so tests never touch the
// process environment.
type Getenv func(key string) string

// Client is one API consumer able to exchange credentials for a token.
type Client struct {
	ID string
	// SecretHash is a bcrypt hash. Plaintext secrets are never retained.
	SecretHash []byte
}

// Config is the fully validated server configuration.
type Config struct {
	Env  string
	Addr string

	JWTSecret []byte
	JWTIssuer string
	JWTTTL    time.Duration

	Clients []Client

	CORSOrigins    []string
	TrustedProxies []string

	RateLimitRPS       float64
	RateLimitBurst     int
	AuthRateLimitRPS   float64
	AuthRateLimitBurst int

	MaxExpressionLength int
	MaxDepth            int
	MaxBodyBytes        int64
	MaxBatchSize        int

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration

	// GeneratedJWTSecret reports that no secret was supplied and a random one
	// was minted for this process. Development only; every restart invalidates
	// every outstanding token.
	GeneratedJWTSecret bool
}

// IsProduction reports whether hardened defaults apply.
func (c *Config) IsProduction() bool { return c.Env == EnvProduction }

type loader struct {
	getenv Getenv
	errs   []string
}

func (l *loader) fail(format string, args ...any) {
	l.errs = append(l.errs, fmt.Sprintf(format, args...))
}

func (l *loader) str(key, fallback string) string {
	if value := strings.TrimSpace(l.getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (l *loader) int(key string, fallback int, min, max int) int {
	raw := strings.TrimSpace(l.getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		l.fail("%s: %q is not an integer", key, raw)
		return fallback
	}
	if value < min || value > max {
		l.fail("%s: %d is outside [%d, %d]", key, value, min, max)
		return fallback
	}
	return value
}

func (l *loader) float(key string, fallback float64, min, max float64) float64 {
	raw := strings.TrimSpace(l.getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		l.fail("%s: %q is not a number", key, raw)
		return fallback
	}
	if value < min || value > max {
		l.fail("%s: %v is outside [%v, %v]", key, value, min, max)
		return fallback
	}
	return value
}

func (l *loader) duration(key string, fallback time.Duration, min, max time.Duration) time.Duration {
	raw := strings.TrimSpace(l.getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		l.fail("%s: %q is not a duration (try 15m, 5s)", key, raw)
		return fallback
	}
	if value < min || value > max {
		l.fail("%s: %s is outside [%s, %s]", key, value, min, max)
		return fallback
	}
	return value
}

// csv splits a comma separated list, dropping blanks.
func csv(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// Load reads, defaults and validates the configuration. All problems are
// reported together so a misconfigured deploy is fixed in one pass.
func Load(getenv Getenv) (*Config, error) {
	l := &loader{getenv: getenv}

	env := l.str("CALC_ENV", EnvDevelopment)
	if env != EnvDevelopment && env != EnvProduction {
		l.fail("CALC_ENV: %q must be %q or %q", env, EnvDevelopment, EnvProduction)
	}
	isProduction := env == EnvProduction

	cfg := &Config{
		Env:  env,
		Addr: l.str("CALC_ADDR", ":"+l.str("CALC_PORT", "8080")),

		JWTIssuer: l.str("CALC_JWT_ISSUER", "calculator-sezzle-api"),
		JWTTTL:    l.duration("CALC_JWT_TTL", 15*time.Minute, time.Minute, 24*time.Hour),

		TrustedProxies: csv(l.getenv("CALC_TRUSTED_PROXIES")),

		RateLimitRPS:       l.float("CALC_RATE_LIMIT_RPS", 20, 0.01, 10_000),
		RateLimitBurst:     l.int("CALC_RATE_LIMIT_BURST", 40, 1, 100_000),
		AuthRateLimitRPS:   l.float("CALC_AUTH_RATE_LIMIT_RPS", 1, 0.01, 1_000),
		AuthRateLimitBurst: l.int("CALC_AUTH_RATE_LIMIT_BURST", 5, 1, 1_000),

		MaxExpressionLength: l.int("CALC_MAX_EXPRESSION_LENGTH", 256, 1, 64_000),
		MaxDepth:            l.int("CALC_MAX_DEPTH", 32, 1, 1_000),
		MaxBodyBytes:        int64(l.int("CALC_MAX_BODY_BYTES", 8*1024, 64, 10*1024*1024)),
		MaxBatchSize:        l.int("CALC_MAX_BATCH_SIZE", 50, 1, 1_000),

		ReadTimeout:     l.duration("CALC_READ_TIMEOUT", 5*time.Second, time.Second, time.Minute),
		WriteTimeout:    l.duration("CALC_WRITE_TIMEOUT", 10*time.Second, time.Second, 5*time.Minute),
		IdleTimeout:     l.duration("CALC_IDLE_TIMEOUT", 60*time.Second, time.Second, 10*time.Minute),
		RequestTimeout:  l.duration("CALC_REQUEST_TIMEOUT", 3*time.Second, 100*time.Millisecond, time.Minute),
		ShutdownTimeout: l.duration("CALC_SHUTDOWN_TIMEOUT", 10*time.Second, time.Second, 2*time.Minute),
	}

	cfg.JWTSecret, cfg.GeneratedJWTSecret = l.jwtSecret(isProduction)
	cfg.Clients = l.clients(isProduction)
	cfg.CORSOrigins = l.corsOrigins(isProduction)

	if len(l.errs) > 0 {
		return nil, errors.New("invalid configuration:\n  - " + strings.Join(l.errs, "\n  - "))
	}
	return cfg, nil
}

func (l *loader) jwtSecret(isProduction bool) (secret []byte, generated bool) {
	raw := strings.TrimSpace(l.getenv("CALC_JWT_SECRET"))

	if raw == "" {
		if isProduction {
			l.fail("CALC_JWT_SECRET: required in production")
			return nil, false
		}
		// Development convenience: a per-process secret. Tokens die on restart,
		// which is the safe failure mode.
		buf := make([]byte, MinJWTSecretLength)
		if _, err := rand.Read(buf); err != nil {
			l.fail("CALC_JWT_SECRET: could not generate a development secret: %v", err)
			return nil, false
		}
		return []byte(hex.EncodeToString(buf)), true
	}

	if len(raw) < MinJWTSecretLength {
		l.fail("CALC_JWT_SECRET: must be at least %d bytes, got %d", MinJWTSecretLength, len(raw))
	}
	if isProduction && strings.Contains(strings.ToLower(raw), "example") {
		l.fail("CALC_JWT_SECRET: the example secret must not be used in production")
	}
	return []byte(raw), false
}

func (l *loader) clients(isProduction bool) []Client {
	var clients []Client
	seen := map[string]bool{}

	add := func(id string, hash []byte) {
		if seen[id] {
			l.fail("CALC_CLIENTS: duplicate client id %q", id)
			return
		}
		seen[id] = true
		clients = append(clients, Client{ID: id, SecretHash: hash})
	}

	for _, entry := range csv(l.getenv("CALC_CLIENTS")) {
		id, hash, ok := strings.Cut(entry, ":")
		if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(hash) == "" {
			l.fail("CALC_CLIENTS: %q is not in id:bcryptHash form", entry)
			continue
		}
		add(strings.TrimSpace(id), []byte(strings.TrimSpace(hash)))
	}

	if plaintext := csv(l.getenv("CALC_CLIENTS_PLAINTEXT")); len(plaintext) > 0 {
		if isProduction {
			l.fail("CALC_CLIENTS_PLAINTEXT: refused in production, use CALC_CLIENTS with bcrypt hashes")
		} else {
			for _, entry := range plaintext {
				id, secret, ok := strings.Cut(entry, ":")
				if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(secret) == "" {
					l.fail("CALC_CLIENTS_PLAINTEXT: %q is not in id:secret form", entry)
					continue
				}
				hash, err := hashSecret(strings.TrimSpace(secret))
				if err != nil {
					l.fail("CALC_CLIENTS_PLAINTEXT: could not hash the secret for %q: %v", id, err)
					continue
				}
				add(strings.TrimSpace(id), hash)
			}
		}
	}

	if len(clients) == 0 {
		l.fail("CALC_CLIENTS: at least one client is required (or CALC_CLIENTS_PLAINTEXT outside production)")
	}
	return clients
}

func (l *loader) corsOrigins(isProduction bool) []string {
	origins := csv(l.str("CALC_CORS_ORIGINS", "http://localhost:5173"))
	for _, origin := range origins {
		if origin == "*" && isProduction {
			l.fail(`CALC_CORS_ORIGINS: "*" is refused in production, list the origins explicitly`)
		}
	}
	return origins
}
