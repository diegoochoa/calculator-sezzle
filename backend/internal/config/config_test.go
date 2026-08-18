package config

import (
	"strings"
	"testing"
	"time"
)

// env builds a Getenv over a literal map so tests never touch the process
// environment and can run in parallel.
func env(values map[string]string) Getenv {
	return func(key string) string { return values[key] }
}

// devEnv is the smallest configuration that should load cleanly.
func devEnv(overrides map[string]string) Getenv {
	values := map[string]string{
		"CALC_CLIENTS_PLAINTEXT": "web:dev-secret",
	}
	for key, value := range overrides {
		values[key] = value
	}
	return env(values)
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load(devEnv(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want %q", cfg.Env, EnvDevelopment)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want \":8080\"", cfg.Addr)
	}
	if cfg.JWTTTL != 15*time.Minute {
		t.Errorf("JWTTTL = %s, want 15m", cfg.JWTTTL)
	}
	if cfg.MaxExpressionLength != 256 || cfg.MaxDepth != 32 {
		t.Errorf("limits = (%d, %d), want (256, 32)", cfg.MaxExpressionLength, cfg.MaxDepth)
	}
	if cfg.IsProduction() {
		t.Error("IsProduction() = true, want false")
	}
}

func TestLoadGeneratesDevelopmentSecret(t *testing.T) {
	t.Parallel()

	cfg, err := Load(devEnv(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.GeneratedJWTSecret {
		t.Fatal("GeneratedJWTSecret = false, want true when CALC_JWT_SECRET is unset in development")
	}
	if len(cfg.JWTSecret) < MinJWTSecretLength {
		t.Errorf("generated secret is %d bytes, want at least %d", len(cfg.JWTSecret), MinJWTSecretLength)
	}
}

func TestLoadPortOverride(t *testing.T) {
	t.Parallel()

	cfg, err := Load(devEnv(map[string]string{"CALC_PORT": "9999"}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != ":9999" {
		t.Errorf("Addr = %q, want \":9999\"", cfg.Addr)
	}
}

func TestLoadPlaintextClientIsHashed(t *testing.T) {
	t.Parallel()

	cfg, err := Load(devEnv(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Clients) != 1 || cfg.Clients[0].ID != "web" {
		t.Fatalf("Clients = %+v, want a single \"web\" client", cfg.Clients)
	}
	if hash := string(cfg.Clients[0].SecretHash); !strings.HasPrefix(hash, "$2") {
		t.Errorf("secret hash = %q, want a bcrypt hash", hash)
	}
}

func TestLoadRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  map[string]string
		wantErr string
	}{
		{
			name:    "no clients at all",
			values:  map[string]string{},
			wantErr: "at least one client is required",
		},
		{
			name:    "unknown environment",
			values:  map[string]string{"CALC_ENV": "staging", "CALC_CLIENTS_PLAINTEXT": "web:s"},
			wantErr: "CALC_ENV",
		},
		{
			name: "production without a secret",
			values: map[string]string{
				"CALC_ENV":     EnvProduction,
				"CALC_CLIENTS": "web:$2a$10$abcdefghijklmnopqrstuv",
			},
			wantErr: "required in production",
		},
		{
			name: "secret too short",
			values: map[string]string{
				"CALC_JWT_SECRET":        "too-short",
				"CALC_CLIENTS_PLAINTEXT": "web:s",
			},
			wantErr: "at least 32 bytes",
		},
		{
			name: "example secret in production",
			values: map[string]string{
				"CALC_ENV":        EnvProduction,
				"CALC_JWT_SECRET": "this-is-the-example-secret-do-not-use",
				"CALC_CLIENTS":    "web:$2a$10$abcdefghijklmnopqrstuv",
			},
			wantErr: "example secret",
		},
		{
			name: "plaintext clients in production",
			values: map[string]string{
				"CALC_ENV":               EnvProduction,
				"CALC_JWT_SECRET":        strings.Repeat("s", 32),
				"CALC_CLIENTS_PLAINTEXT": "web:dev-secret",
			},
			wantErr: "refused in production",
		},
		{
			name: "wildcard cors in production",
			values: map[string]string{
				"CALC_ENV":          EnvProduction,
				"CALC_JWT_SECRET":   strings.Repeat("s", 32),
				"CALC_CLIENTS":      "web:$2a$10$abcdefghijklmnopqrstuv",
				"CALC_CORS_ORIGINS": "*",
			},
			wantErr: "refused in production",
		},
		{
			name: "malformed client entry",
			values: map[string]string{
				"CALC_CLIENTS": "no-separator",
			},
			wantErr: "id:bcryptHash form",
		},
		{
			name: "duplicate client id",
			values: map[string]string{
				"CALC_CLIENTS":           "web:$2a$10$abcdefghijklmnopqrstuv",
				"CALC_CLIENTS_PLAINTEXT": "web:dev-secret",
			},
			wantErr: "duplicate client id",
		},
		{
			name: "non numeric limit",
			values: map[string]string{
				"CALC_CLIENTS_PLAINTEXT": "web:s",
				"CALC_MAX_DEPTH":         "deep",
			},
			wantErr: "is not an integer",
		},
		{
			name: "limit out of range",
			values: map[string]string{
				"CALC_CLIENTS_PLAINTEXT": "web:s",
				"CALC_MAX_DEPTH":         "0",
			},
			wantErr: "outside",
		},
		{
			name: "malformed duration",
			values: map[string]string{
				"CALC_CLIENTS_PLAINTEXT": "web:s",
				"CALC_JWT_TTL":           "fifteen",
			},
			wantErr: "is not a duration",
		},
		{
			name: "malformed rate",
			values: map[string]string{
				"CALC_CLIENTS_PLAINTEXT": "web:s",
				"CALC_RATE_LIMIT_RPS":    "fast",
			},
			wantErr: "is not a number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(env(tt.values))
			if err == nil {
				t.Fatalf("Load() error = nil, want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	t.Parallel()

	_, err := Load(env(map[string]string{
		"CALC_MAX_DEPTH": "nope",
		"CALC_JWT_TTL":   "nope",
	}))
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}

	// Depth, TTL and the missing client should all be reported together so a
	// broken deploy is fixed in one pass rather than three.
	for _, want := range []string{"CALC_MAX_DEPTH", "CALC_JWT_TTL", "CALC_CLIENTS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Load() error = %q, want it to mention %s", err, want)
		}
	}
}

func TestLoadProductionAcceptsCompleteConfig(t *testing.T) {
	t.Parallel()

	cfg, err := Load(env(map[string]string{
		"CALC_ENV":          EnvProduction,
		"CALC_JWT_SECRET":   strings.Repeat("s", 40),
		"CALC_CLIENTS":      "web:$2a$10$abcdefghijklmnopqrstuv,cli:$2a$10$vutsrqponmlkjihgfedcba",
		"CALC_CORS_ORIGINS": "https://calc.example.com, https://admin.example.com",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.IsProduction() {
		t.Error("IsProduction() = false, want true")
	}
	if len(cfg.Clients) != 2 {
		t.Errorf("len(Clients) = %d, want 2", len(cfg.Clients))
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[1] != "https://admin.example.com" {
		t.Errorf("CORSOrigins = %q, want the list trimmed and split", cfg.CORSOrigins)
	}
	if cfg.GeneratedJWTSecret {
		t.Error("GeneratedJWTSecret = true, want false when a secret is supplied")
	}
}
