package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/t0mer/dnsmon/internal/api"
	"github.com/t0mer/dnsmon/internal/cache"
	"github.com/t0mer/dnsmon/internal/checker"
	"github.com/t0mer/dnsmon/internal/config"
	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/resolvers"
	"github.com/t0mer/dnsmon/internal/storage"
	sqlitestore "github.com/t0mer/dnsmon/internal/storage/sqlite"
	pgstore "github.com/t0mer/dnsmon/internal/storage/postgres"
	"github.com/t0mer/dnsmon/internal/version"
)

func main() {
	var (
		cfgFile       = flag.String("config", "", "path to config file")
		resolversFile = flag.String("resolvers-file", "", "path to extra resolvers JSON file")
		listen        = flag.String("listen", "", "override listen address (e.g. :8080)")
		logLevel      = flag.String("log-level", "", "override log level (debug|info|warn|error)")
	)
	flag.Parse()

	cfg, err := config.Load(*cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *listen != "" {
		cfg.Server.Listen = *listen
	}
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}
	if *resolversFile != "" {
		cfg.Resolvers.File = *resolversFile
	}

	log := newLogger(cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	info := version.BuildInfo()
	log.Info("starting dnsmon",
		slog.String("version", info.Version),
		slog.String("commit", info.Commit),
		slog.String("listen", cfg.Server.Listen),
	)

	store, err := newStorage(ctx, cfg, log)
	if err != nil {
		log.Error("failed to initialise storage", slog.Any("error", err))
		os.Exit(1)
	}
	defer store.Close()

	cacheStore, err := cache.New(cfg)
	if err != nil {
		log.Error("failed to initialise cache", slog.Any("error", err))
		os.Exit(1)
	}
	defer cacheStore.Close()

	builtin, err := resolvers.LoadBuiltin()
	if err != nil {
		log.Warn("failed to load builtin resolvers, continuing with empty list", slog.Any("error", err))
		builtin = nil
	}

	registry := resolvers.NewRegistry()
	if err := registry.Reload(ctx, cfg, builtin); err != nil {
		log.Error("failed to load resolvers", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("resolvers loaded", slog.Int("count", len(registry.All())))

	dnsClient := dnsclient.New(cfg)
	chkr := checker.New(dnsClient, registry, cacheStore, store, cfg)

	srv := api.NewServer(chkr, registry, store, cfg, log)

	httpServer := &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      srv.Handler(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", slog.String("addr", cfg.Server.Listen))
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", slog.Any("error", err))
	}

	log.Info("shutdown complete")
}

func newLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	if cfg.Log.Format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func newStorage(ctx context.Context, cfg *config.Config, log *slog.Logger) (storage.Storage, error) {
	switch cfg.Storage.Driver {
	case "sqlite":
		log.Info("using sqlite storage", slog.String("dsn", cfg.Storage.DSN))
		return sqlitestore.New(ctx, cfg.Storage.DSN)
	case "postgres":
		log.Info("using postgres storage")
		return pgstore.New(ctx, cfg.Storage.DSN)
	case "none":
		log.Info("using no-op storage")
		return &storage.Noop{}, nil
	default:
		return nil, fmt.Errorf("unknown storage driver: %s", cfg.Storage.Driver)
	}
}
