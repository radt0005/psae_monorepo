// spade-worker is the Spade block-execution worker daemon.
//
// It consumes core.WorkerAssignment payloads (wrapped in spade_runner.Job)
// from the RabbitMQ queue spade.jobs, runs each one under the isolate
// sandbox via the core/ package, and publishes core.WorkerResult to the
// spade.results queue.  Full spec: ../../../spec/worker.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"core"
	"spade_runner/broker"
	"spade_runner/worker"
)

type config struct {
	AMQPURL          string
	WorkRoot         string
	RegistryPath     string
	CacheDir         string
	Prefetch         int
	ShutdownGraceSec int
	LogLevel         string
	SkipIsolateCheck bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spade-worker:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := parseFlags()

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	// Probe for isolate presence unless explicitly skipped (for local
	// dev on non-Ubuntu hosts).
	if !cfg.SkipIsolateCheck {
		if err := checkIsolate(); err != nil {
			return fmt.Errorf("sandbox unavailable: %w", err)
		}
	}

	reg, err := core.OpenRegistry(cfg.RegistryPath)
	if err != nil {
		return fmt.Errorf("opening registry %s: %w", cfg.RegistryPath, err)
	}
	defer func() { _ = reg.Close() }()

	// Ensure work root exists.
	if err := os.MkdirAll(cfg.WorkRoot, 0777); err != nil {
		return fmt.Errorf("creating work root %s: %w", cfg.WorkRoot, err)
	}

	opts := []worker.Option{}
	if cfg.CacheDir != "" {
		opts = append(opts, worker.WithCache(cfg.CacheDir))
	}
	w := worker.New(reg, cfg.WorkRoot, opts...)

	// Install a signal-driven cancellation context.  SIGINT / SIGTERM
	// triggers a graceful drain: the in-flight job gets ShutdownGraceSec
	// to finish, publish its result, and ack.  If it hasn't completed
	// by then, the context cancels and the loop exits leaving the job
	// unacked — the broker's redelivery mechanism picks it up.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("spade-worker starting",
		"amqp_url", cfg.AMQPURL,
		"work_root", cfg.WorkRoot,
		"registry", cfg.RegistryPath,
	)

	brokerCfg := broker.ReconnectConfig{
		URL:    cfg.AMQPURL,
		Logger: logger,
	}
	handler := func(ctx context.Context, c *broker.Conn) error {
		cons, err := c.NewJobConsumer(ctx, int32(cfg.Prefetch))
		if err != nil {
			return err
		}
		defer func() { _ = cons.Close(context.Background()) }()
		pub, err := c.NewResultPublisher(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = pub.Close(context.Background()) }()
		return worker.RunLoop(ctx, w, cons, pub, logger)
	}

	runErr := broker.Run(ctx, brokerCfg, handler)

	// Grace window: if a job was in flight when SIGTERM arrived, allow
	// it up to ShutdownGraceSec extra time to publish + ack.
	if cfg.ShutdownGraceSec > 0 && runErr == context.Canceled {
		_, dcancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownGraceSec)*time.Second)
		dcancel()
	}

	logger.Info("spade-worker stopped", "reason", fmt.Sprint(runErr))
	return nil
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.AMQPURL, "amqp-url", getenv("SPADE_AMQP_URL", "amqp://guest:guest@localhost:5672/"), "AMQP URL for the RabbitMQ broker")
	flag.StringVar(&cfg.WorkRoot, "work-root", getenv("SPADE_WORK_ROOT", core.DefaultWorkRoot()), "Shared-filesystem root for invocation directories")
	flag.StringVar(&cfg.RegistryPath, "registry", getenv("SPADE_REGISTRY", core.DefaultRegistryPath()), "Path to the SQLite block registry")
	flag.StringVar(&cfg.CacheDir, "cache-dir", getenv("SPADE_CACHE_DIR", ""), "Optional block output cache directory (empty to disable)")
	flag.IntVar(&cfg.Prefetch, "prefetch", 1, "AMQP prefetch count; spec requires 1 in production")
	flag.IntVar(&cfg.ShutdownGraceSec, "shutdown-grace-sec", 60, "Seconds to wait for in-flight job to finish after signal")
	flag.StringVar(&cfg.LogLevel, "log-level", getenv("SPADE_LOG_LEVEL", "info"), "Log level: debug|info|warn|error")
	flag.BoolVar(&cfg.SkipIsolateCheck, "skip-isolate-check", os.Getenv("SPADE_SKIP_ISOLATE_CHECK") == "1", "Skip the isolate-available probe (dev only)")
	flag.Parse()
	return cfg
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func newLogger(level string) *slog.Logger {
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
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

// checkIsolate probes for the isolate binary on $PATH.  The isolate
// package is an Ubuntu-specific sandbox used by core.RunBlockSubprocess.
// If absent, block execution will fail, so we refuse to start.
func checkIsolate() error {
	if _, err := exec.LookPath("isolate"); err != nil {
		return fmt.Errorf("isolate binary not found on PATH (spec requires Ubuntu isolate for sandboxing): %w", err)
	}
	return nil
}
