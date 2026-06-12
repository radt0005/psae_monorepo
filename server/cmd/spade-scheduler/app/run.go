package app

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

	"gopkg.in/yaml.v3"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"core"
	"github.com/google/uuid"

	rbroker "spade_runner/broker"
	"spade_server/api"
	"spade_server/broker"
	"spade_server/engine"
	"spade_server/outbox"
	"spade_server/store"
)

// Config holds the runtime configuration for spade-scheduler.
type Config struct {
	AMQPURL          string
	DatabaseURL      string
	HTTPAddr         string
	LogLevel         string
	ShutdownGraceSec int
	// SkipBroker disables the RabbitMQ wiring (for local development
	// and tests that drive the engine directly).
	SkipBroker bool

	// UIBaseURL is the base URL of the Nuxt web UI.  When set, the
	// scheduler calls PATCH <UIBaseURL>/api/runs/:id after each pipeline
	// state transition so the web UI can update its run status.
	UIBaseURL string
	// UICallbackSecret is presented as "Authorization: Bearer <secret>"
	// on every callback request.  Must match WORKER_CALLBACK_SECRET in
	// the web UI's environment.
	UICallbackSecret string
	// UIDBUrl is the PostgreSQL DSN for the web UI's database.  The
	// scheduler polls for queued runs using the Postgres outbox pattern
	// (spec/worker.md §Communication).  Defaults to DatabaseURL when
	// empty (single-database topology).
	UIDBUrl string
}

// ParseFlags parses argv into a Config, honoring SPADE_* environment
// variable defaults.  argv is typically os.Args[1:].
func ParseFlags(argv []string) Config {
	fs := flag.NewFlagSet("spade-scheduler", flag.ContinueOnError)
	cfg := Config{}
	fs.StringVar(&cfg.AMQPURL, "amqp-url", getenv("SPADE_AMQP_URL", "amqp://guest:guest@localhost:5672/"), "AMQP URL for RabbitMQ")
	fs.StringVar(&cfg.DatabaseURL, "database-url", getenv("SPADE_DATABASE_URL", ""), "PostgreSQL DSN (empty enables a SQLite-backed fallback)")
	fs.StringVar(&cfg.HTTPAddr, "http-addr", getenv("SPADE_HTTP_ADDR", ":1323"), "HTTP listen address")
	fs.StringVar(&cfg.LogLevel, "log-level", getenv("SPADE_LOG_LEVEL", "info"), "Log level: debug|info|warn|error")
	fs.IntVar(&cfg.ShutdownGraceSec, "shutdown-grace-sec", 30, "Seconds to wait for in-flight work after signal")
	fs.BoolVar(&cfg.SkipBroker, "skip-broker", os.Getenv("SPADE_SKIP_BROKER") == "1", "Skip RabbitMQ wiring (for local dev)")
	fs.StringVar(&cfg.UIBaseURL, "ui-base-url", getenv("SPADE_UI_BASE_URL", ""), "Base URL of the Spade web UI for run callbacks")
	fs.StringVar(&cfg.UICallbackSecret, "ui-callback-secret", getenv("SPADE_UI_CALLBACK_SECRET", ""), "Bearer token for PATCH /api/runs/:id callbacks")
	fs.StringVar(&cfg.UIDBUrl, "ui-db-url", getenv("SPADE_UI_DB_URL", ""), "PostgreSQL DSN for the web UI database (outbox source); defaults to database-url")
	_ = fs.Parse(argv)
	return cfg
}

// Run is the testable entry point.  argv is the program arguments
// (typically os.Args[1:]).  When SPADE_SKIP_BROKER=1 the binary serves
// the HTTP API without dialling a broker — useful for local UI
// development.
func Run(argv []string) error {
	cfg := ParseFlags(argv)
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	st, err := openStore(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	// Open the web UI database once: it backs both the block-manifest
	// provider (the registry metadata mirror, registry.md §10 — locally
	// populated by the seed-blocks service) and the outbox poller below.
	uiDB, uiDBErr := openUIDB(uiDBUrl(cfg))

	var mp engine.ManifestProvider
	if uiDBErr != nil {
		logger.Warn("could not open UI database; pipeline validation will reject all blocks until it is reachable", "err", uiDBErr)
		mp = engine.NewMapManifestProvider()
	} else {
		mp = engine.NewPgManifestProvider(uiDB)
	}

	logger.Info("spade-scheduler starting",
		"http_addr", cfg.HTTPAddr,
		"amqp_url", cfg.AMQPURL,
		"database_url", maskDSN(cfg.DatabaseURL),
		"skip_broker", cfg.SkipBroker,
		"ui_base_url", cfg.UIBaseURL,
		"ui_db_url", maskDSN(uiDBUrl(cfg)),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Build engine.  If we're skipping the broker, the JobPublisher is
	// a no-op fake — useful for local UI development against an empty
	// queue.
	var pub broker.JobPublisher = &broker.FakeJobPublisher{}
	var resultConsumer broker.ResultConsumer

	eng := engine.New(st, pub, mp, logger)

	// Wire the web UI callback if configured.
	if cfg.UIBaseURL != "" && cfg.UICallbackSecret != "" {
		uiBase, uiSecret := cfg.UIBaseURL, cfg.UICallbackSecret
		eng.SetCallback(func(ctx context.Context, pipelineID uuid.UUID, payload engine.CallbackPayload) {
			if err := api.PatchRunStatus(ctx, uiBase, uiSecret, pipelineID.String(), payload); err != nil {
				logger.Warn("run callback failed", "run_id", pipelineID, "err", err)
			}
		})
	} else if cfg.UIBaseURL != "" {
		logger.Warn("SPADE_UI_BASE_URL is set but SPADE_UI_CALLBACK_SECRET is empty; callbacks disabled")
	}

	if err := eng.Recover(ctx); err != nil {
		return fmt.Errorf("recover: %w", err)
	}

	httpSrv := api.New(eng, st, logger)
	httpErr := make(chan error, 1)
	go func() {
		if err := httpSrv.Start(cfg.HTTPAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErr <- err
		}
	}()

	// Broker wiring runs in its own goroutine inside the reconnect loop.
	brokerErr := make(chan error, 1)
	if !cfg.SkipBroker {
		go func() {
			brokerErr <- runBroker(ctx, cfg, eng, logger, &pub, &resultConsumer)
		}()
	} else {
		// Start the engine with no broker; the dispatch loop still
		// runs and publishes via the fake publisher so anyone driving
		// the API can see dispatches go out.
		go func() {
			brokerErr <- eng.Run(ctx, broker.NewFakeResultConsumer())
		}()
	}

	// Postgres outbox poller: pick up queued runs from the web UI's database.
	// Reuses the uiDB handle opened above for the manifest provider.
	if uiDBErr != nil {
		logger.Warn("outbox: UI database unavailable; outbox polling disabled", "err", uiDBErr)
	} else {
		submit := buildSubmitFn(eng)
		go outbox.Run(ctx, uiDB, submit, 5*time.Second, logger)
		defer func() {
			if sqlDB, err := uiDB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}()
	}

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-httpErr:
		logger.Error("http server failed", "err", err)
	case err := <-brokerErr:
		logger.Error("broker loop failed", "err", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownGraceSec)*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	logger.Info("spade-scheduler stopped")
	return nil
}

// buildSubmitFn returns the closure the outbox poller calls for each queued run.
func buildSubmitFn(eng *engine.Engine) func(ctx context.Context, run outbox.QueuedRun) error {
	return func(ctx context.Context, run outbox.QueuedRun) error {
		var p core.Pipeline
		if err := yaml.Unmarshal([]byte(run.YAML), &p); err != nil {
			return fmt.Errorf("parse yaml: %w", err)
		}
		// The run id is the canonical submission/pipeline id: the engine keys
		// its store on it and the run-status callback targets
		// PATCH /api/runs/<id>.  Override any top-level id carried in the YAML
		// so the callback updates the right run row (pipeline.md §10: the
		// pipeline id is assigned at submission, not authored into the file).
		runUUID, err := uuid.Parse(run.ID)
		if err != nil {
			return fmt.Errorf("parse run id %q: %w", run.ID, err)
		}
		p.Id = runUUID
		if err := eng.SubmitPipeline(ctx, &p, []byte(run.YAML), run.OwnerID); err != nil {
			if errors.Is(err, store.ErrAlreadyExists) {
				// Scheduler restarted and already has this pipeline — not an error.
				return nil
			}
			return err
		}
		return nil
	}
}

// runBroker drives the broker reconnect loop.  Inside, it opens a
// JobPublisher and ResultConsumer per connection and feeds them into
// engine.Run.  Returns when ctx is cancelled.
func runBroker(ctx context.Context, cfg Config, eng *engine.Engine, logger *slog.Logger, pubOut *broker.JobPublisher, consOut *broker.ResultConsumer) error {
	rb := rbroker.ReconnectConfig{URL: cfg.AMQPURL, Logger: logger}
	return rbroker.Run(ctx, rb, func(ctx context.Context, c *rbroker.Conn) error {
		// Open the two halves on each (re)connect.
		jobPub, err := c.NewJobPublisher(ctx)
		if err != nil {
			return err
		}
		defer jobPub.Close(context.Background())
		resCons, err := c.NewResultConsumer(ctx, 32)
		if err != nil {
			return err
		}
		defer resCons.Close(context.Background())
		_ = pubOut
		_ = consOut
		return runEngineWithBroker(ctx, eng, jobPub, resCons, logger)
	})
}

// runEngineWithBroker wraps the runner-side broker adapters and drives
// the engine with them.  Each reconnect produces fresh adapters.
func runEngineWithBroker(ctx context.Context, eng *engine.Engine, jobPub rbroker.ResultPublisher, resCons rbroker.JobConsumer, logger *slog.Logger) error {
	pub := broker.NewJobPublisher(jobPub)
	cons := broker.NewResultConsumer(resCons)
	eng.UpdatePublisher(pub)
	return eng.Run(ctx, cons)
}

// openStore opens the persistence layer for the scheduler's own tables.
// Falls back to SQLite when SPADE_DATABASE_URL is empty so the binary
// remains runnable without a PostgreSQL dependency for local dev.
func openStore(dsn string) (store.Store, error) {
	if dsn == "" {
		return store.NewSQLiteStore(":memory:")
	}
	return store.NewPgStore(dsn)
}

// openUIDB opens a raw GORM connection to the web UI's database for outbox
// polling.  It does NOT run auto-migrations — the web UI's schema is managed
// by Drizzle and must not be touched by the scheduler's GORM layer.
func openUIDB(dsn string) (*gorm.DB, error) {
	cfg := &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)}
	if dsn == "" {
		return gorm.Open(sqlite.Open(":memory:"), cfg)
	}
	return gorm.Open(postgres.Open(dsn), cfg)
}

// uiDBUrl resolves the effective UI database DSN: falls back to DatabaseURL
// when UIDBUrl is unset (single-database topology where both services share
// one PostgreSQL instance).
func uiDBUrl(cfg Config) string {
	if cfg.UIDBUrl != "" {
		return cfg.UIDBUrl
	}
	return cfg.DatabaseURL
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

// maskDSN returns a DSN with the password component replaced.  Used for
// log output so credentials don't leak.
func maskDSN(dsn string) string {
	if dsn == "" {
		return "(sqlite:memory)"
	}
	return "(set)"
}
