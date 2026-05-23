package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kardianos/service"
	"github.com/t0mer/dnsmon/internal/api"
	"github.com/t0mer/dnsmon/internal/cache"
	"github.com/t0mer/dnsmon/internal/checker"
	"github.com/t0mer/dnsmon/internal/config"
	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/resolvers"
	"github.com/t0mer/dnsmon/internal/storage"
	pgstore "github.com/t0mer/dnsmon/internal/storage/postgres"
	sqlitestore "github.com/t0mer/dnsmon/internal/storage/sqlite"
	"github.com/t0mer/dnsmon/internal/version"
)

func main() {
	var (
		cfgFile       = flag.String("config", "", "path to config file")
		resolversFile = flag.String("resolvers-file", "", "path to extra resolvers JSON file")
		listen        = flag.String("listen", "", "override listen address (e.g. :8080)")
		port          = flag.Int("port", 0, "override server port (e.g. 8080)")
		logLevel      = flag.String("log-level", "", "override log level (debug|info|warn|error)")
		serviceAction = flag.String("service", "", "manage the system service: install|uninstall|start|stop|restart")
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
	if *port != 0 {
		cfg.Server.Listen = applyPortOverride(cfg.Server.Listen, *port)
	}
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}
	if *resolversFile != "" {
		cfg.Resolvers.File = *resolversFile
	}

	log := newLogger(cfg)

	prog := &program{cfg: cfg, log: log}

	svcConfig := &service.Config{
		Name:        "dnsmon",
		DisplayName: "dnsmon — DNS Propagation Checker",
		Description: "Self-hosted DNS propagation checker (whatsmydns.net alternative).",
		Arguments:   serviceArguments(*cfgFile, *resolversFile, *listen, *port, *logLevel),
	}

	svc, err := service.New(prog, svcConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create service: %v\n", err)
		os.Exit(1)
	}

	// Service management actions exit after running; they never start the server.
	if *serviceAction != "" {
		if err := service.Control(svc, *serviceAction); err != nil {
			fmt.Fprintf(os.Stderr, "service %s failed: %v\nvalid actions: %s\n",
				*serviceAction, err, strings.Join(service.ControlAction[:], ", "))
			os.Exit(1)
		}
		fmt.Printf("service %q: %s ok\n", svcConfig.Name, *serviceAction)
		return
	}

	info := version.BuildInfo()
	log.Info("starting dnsmon",
		slog.String("version", info.Version),
		slog.String("commit", info.Commit),
		slog.String("listen", cfg.Server.Listen),
	)

	// Run blocks until the service is stopped (SIGINT/SIGTERM when interactive,
	// or a stop request from the OS service manager), then calls Stop.
	if err := svc.Run(); err != nil {
		log.Error("service run error", slog.Any("error", err))
		os.Exit(1)
	}
}

// program implements service.Interface, wiring the HTTP server lifecycle to the
// service manager (or to an interactive run).
type program struct {
	cfg *config.Config
	log *slog.Logger

	httpServer *http.Server
	store      storage.Storage
	cacheStore cache.Cache
}

// Start initialises dependencies and launches the HTTP server. It must not block.
func (p *program) Start(service.Service) error {
	ctx := context.Background()

	store, err := newStorage(ctx, p.cfg, p.log)
	if err != nil {
		return fmt.Errorf("initialising storage: %w", err)
	}
	p.store = store

	cacheStore, err := cache.New(p.cfg)
	if err != nil {
		return fmt.Errorf("initialising cache: %w", err)
	}
	p.cacheStore = cacheStore

	builtin, err := resolvers.LoadBuiltin()
	if err != nil {
		p.log.Warn("failed to load builtin resolvers, continuing with empty list", slog.Any("error", err))
		builtin = nil
	}

	registry := resolvers.NewRegistry()
	if err := registry.Reload(ctx, p.cfg, builtin); err != nil {
		return fmt.Errorf("loading resolvers: %w", err)
	}
	p.log.Info("resolvers loaded", slog.Int("count", len(registry.All())))

	dnsClient := dnsclient.New(p.cfg)
	chkr := checker.New(dnsClient, registry, cacheStore, store, p.cfg)
	srv := api.NewServer(chkr, registry, store, p.cfg, p.log)

	p.httpServer = &http.Server{
		Addr:         p.cfg.Server.Listen,
		Handler:      srv.Handler(),
		ReadTimeout:  p.cfg.Server.ReadTimeout,
		WriteTimeout: p.cfg.Server.WriteTimeout,
	}

	go p.run()
	return nil
}

func (p *program) run() {
	p.log.Info("http server listening", slog.String("addr", p.cfg.Server.Listen))
	if err := p.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		p.log.Error("server error", slog.Any("error", err))
		// When running interactively a fatal bind error should fail the process;
		// under a service manager the manager decides on restart policy.
		if service.Interactive() {
			os.Exit(1)
		}
	}
}

// Stop gracefully shuts down the HTTP server and releases resources.
func (p *program) Stop(service.Service) error {
	p.log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if p.httpServer != nil {
		if err := p.httpServer.Shutdown(shutdownCtx); err != nil {
			p.log.Error("shutdown error", slog.Any("error", err))
		}
	}
	if p.store != nil {
		p.store.Close()
	}
	if p.cacheStore != nil {
		p.cacheStore.Close()
	}

	p.log.Info("shutdown complete")
	return nil
}

// serviceArguments reconstructs the flags the installed service should run with.
// File paths are made absolute because the service runs from a different working
// directory than the install command.
func serviceArguments(cfgFile, resolversFile, listen string, port int, logLevel string) []string {
	var args []string
	if cfgFile != "" {
		args = append(args, "--config", absOrSelf(cfgFile))
	}
	if resolversFile != "" {
		args = append(args, "--resolvers-file", absOrSelf(resolversFile))
	}
	if listen != "" {
		args = append(args, "--listen", listen)
	}
	if port != 0 {
		args = append(args, "--port", strconv.Itoa(port))
	}
	if logLevel != "" {
		args = append(args, "--log-level", logLevel)
	}
	return args
}

func absOrSelf(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// applyPortOverride returns listen with its port replaced by port, preserving any
// host portion. A non-positive port leaves listen unchanged.
func applyPortOverride(listen string, port int) string {
	if port <= 0 {
		return listen
	}
	host := ""
	if h, _, err := net.SplitHostPort(listen); err == nil {
		host = h
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
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
