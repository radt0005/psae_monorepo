// Command registryd is the Plugin Registry control plane (registry.md): the
// HTTP API, lifecycle state machine, audit log, metadata mirror, ed25519
// signing, the trusted-public-key set, worker fetch, and the build dispatcher.
// It shares the application Postgres database; build workers do not.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"spade_registry/internal/api"
	"spade_registry/internal/audit"
	"spade_registry/internal/auth"
	"spade_registry/internal/blob"
	"spade_registry/internal/config"
	"spade_registry/internal/dispatch"
	"spade_registry/internal/mirror"
	"spade_registry/internal/sign"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("registryd exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg, err := config.LoadRegistry()
	if err != nil {
		return err
	}

	// Persistence: Postgres in production, SQLite for local/dev.
	st, err := openStore(cfg)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	// Blob storage: S3-compatible when configured, else filesystem.
	bs, err := openBlob(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("open blob store: %w", err)
	}

	// Signing keyset: ensure an active key (or import one from env).
	keyset := sign.NewKeyset(st)
	if err := ensureSigningKey(cfg, keyset); err != nil {
		return fmt.Errorf("init signing key: %w", err)
	}

	// Metadata mirror: write to the shared blocks table only when on Postgres.
	var mr mirror.Mirror = mirror.NoopMirror{}
	if cfg.MirrorEnabled && cfg.DatabaseURL != "" {
		mr = mirror.NewPostgres(st.DB())
		log.Info("metadata mirror enabled")
	}

	auditor := audit.New(st)
	machine := state.New(st, auditor, mr)

	// Developer auth validates Better Auth sessions against the shared DB.
	dev := auth.NewSessionVerifier(st.DB())

	baseURL := os.Getenv("REGISTRY_PUBLIC_URL")
	srv := api.New(api.Options{
		Config: cfg, Store: st, Keyset: keyset, Audit: auditor,
		State: machine, Blob: bs, Dev: dev, BaseURL: baseURL,
	})
	e := srv.Routes()

	// Build dispatcher: launch an ephemeral container per build.
	internalURL := os.Getenv("REGISTRY_INTERNAL_URL")
	if internalURL == "" {
		internalURL = "http://localhost" + cfg.ListenAddr
	}
	disp := dispatch.New(dispatch.Options{
		Config: cfg, Store: st, State: machine,
		Launcher:    dispatch.DockerLauncher{ExtraArgs: cfg.BuilderDockerArgs},
		RegistryURL: internalURL,
		Logger:      log,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go disp.Run(ctx)

	httpSrv := &http.Server{Addr: cfg.ListenAddr, Handler: e}
	go func() {
		log.Info("registryd listening", "addr", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}

func openStore(cfg config.RegistryConfig) (*store.Store, error) {
	if cfg.DatabaseURL != "" {
		return store.Open(cfg.DatabaseURL)
	}
	return store.OpenSQLite(cfg.SQLitePath)
}

func openBlob(ctx context.Context, cfg config.RegistryConfig) (blob.Store, error) {
	if cfg.S3Endpoint != "" || cfg.S3AccessKey != "" {
		return blob.NewS3Store(ctx, blob.S3Options{
			Endpoint:     cfg.S3Endpoint,
			Region:       cfg.S3Region,
			Bucket:       cfg.S3Bucket,
			AccessKey:    cfg.S3AccessKey,
			SecretKey:    cfg.S3SecretKey,
			UsePathStyle: cfg.S3UsePathStyle,
		})
	}
	return blob.NewFSStore(cfg.BlobDir)
}

func ensureSigningKey(cfg config.RegistryConfig, keyset *sign.Keyset) error {
	if cfg.SigningKeySource == "env" {
		_, err := keyset.ImportKey(cfg.SigningPublicKey, cfg.SigningPrivateKey)
		return err
	}
	_, err := keyset.EnsureActiveKey()
	return err
}
