// Command buildrunnerd is the standalone registry build runner (hosting.md §5):
// it claims queued build jobs from the shared Postgres queue and launches an
// ephemeral, language-specific build container per job via Docker. It is the
// out-of-process counterpart to registryd's embedded dispatcher — in production
// registryd runs on App Platform (no Docker daemon) with
// BUILD_DISPATCH_ENABLED=false, and this service runs on the build-runner
// Droplet with the host docker socket.
//
// The trust separation is unchanged: untrusted code runs only inside the
// builder containers, which receive a per-job token and staging-scoped S3
// credentials, never database credentials.
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

	"spade_registry/internal/audit"
	"spade_registry/internal/config"
	"spade_registry/internal/dispatch"
	"spade_registry/internal/mirror"
	"spade_registry/internal/state"
	"spade_registry/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("buildrunnerd exited", "err", err)
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
	// The runner must share registryd's database — a SQLite fallback would be a
	// different queue than the one the control plane writes publishes to.
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("buildrunnerd requires DATABASE_URL (the same Postgres as registryd)")
	}
	// Refuse the config default builder-image map: on a runner host the images
	// come from a registry (DOCR) and must be named explicitly.
	if os.Getenv("BUILDER_IMAGES") == "" {
		return fmt.Errorf("buildrunnerd requires BUILDER_IMAGES (language=image,...)")
	}
	// The URL builder containers report back to (registryd's /builds/:id/*).
	registryURL := os.Getenv("REGISTRY_URL")
	if registryURL == "" {
		return fmt.Errorf("buildrunnerd requires REGISTRY_URL (registryd endpoint for builder callbacks)")
	}

	st, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	// Same transition side effects as registryd: audit always, mirror when on.
	var mr mirror.Mirror = mirror.NoopMirror{}
	if cfg.MirrorEnabled {
		mr = mirror.NewPostgres(st.DB())
	}
	machine := state.New(st, audit.New(st), mr)

	disp := dispatch.New(dispatch.Options{
		Config: cfg, Store: st, State: machine,
		Launcher:    dispatch.DockerLauncher{ExtraArgs: cfg.BuilderDockerArgs},
		RegistryURL: registryURL,
		Logger:      log,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Minimal liveness endpoint for Droplet monitoring.
	addr := os.Getenv("BUILDRUNNER_LISTEN_ADDR")
	if addr == "" {
		addr = ":8091"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	httpSrv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health server error", "err", err)
			stop()
		}
	}()

	log.Info("buildrunnerd dispatching", "registry", registryURL,
		"images", cfg.BuilderImages, "healthz", addr)
	disp.Run(ctx) // blocks until signal

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}
