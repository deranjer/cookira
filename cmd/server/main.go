// Command server is the single deployable artifact: it serves the REST/OpenAPI
// API, the MCP endpoint and the built frontend (embedded at build time) all
// from one binary.
package main

import (
	"context"
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
	srv := api.New(pool, svc, publicURL)

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

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
