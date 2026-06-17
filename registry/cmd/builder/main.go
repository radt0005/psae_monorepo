// Command builder is the registry's build worker. It runs inside a per-language
// container launched by the registry dispatcher: it clones the published commit,
// screens it, builds the language-specific artifact, uploads the unsigned
// tarball to S3 staging, and reports the result to the registry over HTTP. It
// holds no database access and never holds the signing key.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"spade_registry/internal/blob"
	"spade_registry/internal/builder"
	"spade_registry/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "builder:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadBuilder()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bs, err := blob.NewS3Store(ctx, blob.S3Options{
		Endpoint:     cfg.S3Endpoint,
		Region:       cfg.S3Region,
		Bucket:       cfg.S3Bucket,
		AccessKey:    cfg.S3AccessKey,
		SecretKey:    cfg.S3SecretKey,
		UsePathStyle: cfg.S3UsePathStyle,
	})
	if err != nil {
		return fmt.Errorf("init blob store: %w", err)
	}

	client := builder.NewClient(cfg.RegistryURL, cfg.JobID, cfg.Token)
	return builder.Run(ctx, builder.Deps{
		Client:   client,
		Cloner:   builder.GitCloner{},
		Screener: builder.NoopScreener{}, // future: AI screening agent
		Blob:     bs,
	})
}
