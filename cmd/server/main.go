// Command server is the single deployable artifact: it serves the REST/OpenAPI
// API, the MCP endpoint and the built frontend (embedded at build time) all
// from one binary.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/deranjer/cookira/internal/api"
	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
	"github.com/deranjer/cookira/internal/recipes"
	"github.com/deranjer/cookira/internal/storage"
	"github.com/deranjer/cookira/internal/webapp"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cookira?sslmode=disable")
	addr := getenv("ADDR", ":8080")
	publicURL := getenv("PUBLIC_URL", "http://localhost:8080")
	auth.SecureCookies = strings.HasPrefix(publicURL, "https://")

	if err := db.Migrate(databaseURL); err != nil {
		return err
	}
	slog.Info("migrations applied")

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.Info("connected to database")

	store, err := storage.NewDisk(getenv("IMAGES_DIR", "./data/images"))
	if err != nil {
		return err
	}

	svc := recipes.New(pool, db.New(pool), store, publicURL)
	secret, err := jwtSecret()
	if err != nil {
		return err
	}
	srv := api.New(pool, svc, publicURL, secret)
	go pruneRefreshTokens(ctx, db.New(pool))

	frontend, err := webapp.Handler()
	if err != nil {
		return err
	}
	srv.Router.NotFound(frontend.ServeHTTP)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown error", "err", err)
		}
	}()

	slog.Info("listening", "addr", addr, "publicURL", publicURL)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// jwtSecret returns the key that signs login access tokens. Set JWT_SECRET
// (32+ characters) in production. Without it a random key is generated at
// startup: everything still works, but access tokens die on restart and
// clients silently refresh, so it is only suitable for development.
func jwtSecret() ([]byte, error) {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		if len(s) < 32 {
			return nil, errors.New("JWT_SECRET must be at least 32 characters")
		}
		return []byte(s), nil
	}
	slog.Warn("JWT_SECRET is not set; using a random key for this run")
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// pruneRefreshTokens deletes long-expired refresh tokens once a day.
func pruneRefreshTokens(ctx context.Context, q *db.Queries) {
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		if err := q.DeleteExpiredRefreshTokens(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("pruning refresh tokens", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
