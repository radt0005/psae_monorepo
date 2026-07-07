// Command spade-kms is the Spade key-management service (spec/secrets.md §5):
// it stores envelope-encrypted user secrets, serves secret management to
// authenticated users, and resolves capability-token-scoped secrets for the
// worker. The key-encryption key lives only in this process.
package main

import (
	"log/slog"
	"os"

	"captoken"

	"spade_kms/internal/api"
	"spade_kms/internal/envelope"
	"spade_kms/internal/store"
	"spade_kms/internal/token"
)

func main() {
	if err := run(); err != nil {
		slog.Error("spade-kms exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	// Storage: PostgreSQL in production, SQLite for local/dev.
	var st store.Store
	var err error
	if dsn := os.Getenv("KMS_DATABASE_DSN"); dsn != "" {
		st, err = store.NewPgStore(dsn)
	} else {
		log.Warn("KMS_DATABASE_DSN unset; using SQLite (development only)")
		st, err = store.NewSQLiteStore(os.Getenv("KMS_SQLITE_PATH"))
	}
	if err != nil {
		return err
	}

	// Key set: KMS_KEKS="id:base64,..." with KMS_ACTIVE_KEK naming the active id.
	keyMap, err := envelope.ParseKeys(os.Getenv("KMS_KEKS"))
	if err != nil {
		return err
	}
	keys, err := envelope.NewKeySet(keyMap, os.Getenv("KMS_ACTIVE_KEK"))
	if err != nil {
		return err
	}

	// Capability-token verifier: the trusted scheduler public keys come from
	// KMS_TOKEN_PUBKEYS (comma-separated base64, a list for rotation). Without
	// them, /resolve rejects all tokens.
	var verifier token.Verifier = token.Unconfigured{}
	if spec := os.Getenv("KMS_TOKEN_PUBKEYS"); spec != "" {
		pubkeys, err := captoken.ParsePublicKeys(spec)
		if err != nil {
			return err
		}
		verifier = token.NewEd25519Verifier(pubkeys)
		log.Info("capability-token verification enabled", "trusted_keys", len(pubkeys))
	} else {
		log.Warn("KMS_TOKEN_PUBKEYS unset; /resolve will reject all tokens")
	}

	srv := api.New(st, keys, verifier, log)

	addr := os.Getenv("KMS_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	log.Info("spade-kms listening", "addr", addr)
	return srv.Start(addr)
}
