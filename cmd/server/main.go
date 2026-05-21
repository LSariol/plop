package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lsariol/plop/internal/api"
	"github.com/lsariol/plop/internal/auth"
	"github.com/lsariol/plop/internal/config"
	"github.com/lsariol/plop/internal/db"
	"github.com/lsariol/plop/internal/notify"
	"github.com/lsariol/plop/internal/sse"
	"github.com/lsariol/plop/internal/store"
)

func main() {
	if err := godotenv.Load(); err != nil {
		// Not fatal — production runs on real env vars, not a .env file.
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	setupLogging(cfg.LogFormat, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool); err != nil {
		slog.Error("migrations", "error", err)
		os.Exit(1)
	}

	s := store.New(pool, cfg.PayloadDir)
	s.StartCleanup(ctx, 5*time.Minute)

	hub := notify.NewHub()
	sseHub := sse.NewHub()
	h := api.New(pool, hub, sseHub, s, cfg)

	requireSession := func(next http.Handler) http.Handler {
		return auth.RequireSession(pool, next)
	}
	requireClient := func(next http.Handler) http.Handler {
		return auth.RequireDesktopToken(pool, next)
	}

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, h, ctx, requireSession, requireClient)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      api.SecurityHeaders(api.RequestLogger(mux)),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("listening", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "error", err)
	}
	slog.Info("shutdown complete")
}

func setupLogging(format, level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	// SetDefault also bridges the legacy log package — all log.Printf calls
	// in dependencies automatically flow through slog.
	slog.SetDefault(slog.New(handler))
}
