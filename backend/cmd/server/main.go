// Command server runs the calculation API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/diegoochoa/calculator-sezzle-api/internal/auth"
	"github.com/diegoochoa/calculator-sezzle-api/internal/calc"
	"github.com/diegoochoa/calculator-sezzle-api/internal/config"
	"github.com/diegoochoa/calculator-sezzle-api/internal/httpapi"
	"github.com/diegoochoa/calculator-sezzle-api/internal/middleware"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

// timeoutBody is served when a request exceeds the per-request deadline. It is
// pre-rendered because http.TimeoutHandler takes a fixed string.
const timeoutBody = `{"error":{"code":"TIMEOUT","message":"The request took too long"}}`

func main() {
	// The runtime image is distroless: no shell, no curl, nothing for a
	// container healthcheck to call. So the binary probes itself.
	healthcheck := flag.Bool("healthcheck", false, "probe the local server and exit 0 when healthy")
	flag.Parse()

	if *healthcheck {
		os.Exit(probeHealth())
	}

	if err := run(); err != nil {
		// The logger may not exist yet, so failures go to stderr directly.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// probeHealth returns a process exit code, not an error: it is the entire
// contract with the container runtime.
func probeHealth() int {
	port := os.Getenv("CALC_PORT")
	if port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	handler, err := buildHandler(cfg, logger)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           http.TimeoutHandler(handler, cfg.RequestTimeout, timeoutBody),
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	return serve(server, cfg, logger)
}

func buildHandler(cfg *config.Config, logger *slog.Logger) (http.Handler, error) {
	credentials := make([]auth.Credential, 0, len(cfg.Clients))
	for _, client := range cfg.Clients {
		credentials = append(credentials, auth.Credential{ID: client.ID, SecretHash: client.SecretHash})
	}

	issuer := auth.NewIssuer(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	clients := auth.NewStore(credentials)

	resolver, err := middleware.NewIPResolver(cfg.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("CALC_TRUSTED_PROXIES: %w", err)
	}

	server := httpapi.NewServer(httpapi.ServerConfig{
		CalcOptions: calc.Options{
			MaxLength: cfg.MaxExpressionLength,
			MaxDepth:  cfg.MaxDepth,
			Precision: calc.Precision,
		},
		MaxBatch: cfg.MaxBatchSize,
		Issuer:   issuer,
		Clients:  clients,
		Logger:   logger,
		Version:  version,
	})

	return httpapi.NewRouter(httpapi.RouterConfig{
		Server:       server,
		Verifier:     issuer,
		APILimiter:   middleware.NewLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst),
		AuthLimiter:  middleware.NewLimiter(cfg.AuthRateLimitRPS, cfg.AuthRateLimitBurst),
		IPResolver:   resolver,
		CORSOrigins:  cfg.CORSOrigins,
		MaxBodyBytes: cfg.MaxBodyBytes,
	}), nil
}

// serve runs the server until interrupted, then drains in-flight requests.
func serve(server *http.Server, cfg *config.Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("server starting",
		"addr", cfg.Addr,
		"env", cfg.Env,
		"version", version,
		"clients", len(cfg.Clients),
		"corsOrigins", cfg.CORSOrigins,
		"rateLimitRps", cfg.RateLimitRPS,
	)
	if cfg.GeneratedJWTSecret {
		logger.Warn("no CALC_JWT_SECRET was set, so a random one was generated; " +
			"every restart invalidates all outstanding tokens")
	}

	failed := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
	}()

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down", "timeout", cfg.ShutdownTimeout)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		// In-flight requests outlasted the grace period; report it rather than
		// pretending the drain was clean.
		return fmt.Errorf("shutdown: %w", err)
	}

	logger.Info("stopped")
	return nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	options := &slog.HandlerOptions{Level: slog.LevelInfo}

	// Machine-readable in production, readable at a terminal in development.
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, options))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, options))
}

// Compile-time assertion that the issuer satisfies both roles it is wired into.
var (
	_ httpapi.TokenIssuer = (*auth.Issuer)(nil)
	_ middleware.Verifier = (*auth.Issuer)(nil)
)
