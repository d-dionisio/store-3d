package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"example.com/loja-3d/backend/internal/infrastructure/bambulab"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
)

func main() {
	execute := flag.Bool("execute", false, "commit the verified key rotation")
	flag.Parse()
	version, err := strconv.ParseInt(os.Getenv("BAMBU_LAN_KEY_VERSION"), 10, 16)
	if err != nil || version < 1 {
		slog.Error("invalid BAMBU_LAN_KEY_VERSION")
		os.Exit(1)
	}
	keyring, err := bambulab.LoadKeyring(os.Getenv("BAMBU_LAN_ENCRYPTION_KEY"), int16(version), os.Getenv("BAMBU_LAN_DECRYPTION_KEYS"))
	if err != nil {
		slog.Error("invalid Bambu LAN key configuration")
		os.Exit(1)
	}
	cfg, err := postgres.LoadConfig()
	if err != nil {
		slog.Error("invalid database configuration")
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		slog.Error("database initialization failed")
		os.Exit(1)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		slog.Error("could not begin rotation")
		os.Exit(1)
	}
	defer tx.Rollback(ctx)
	count, err := bambulab.RotateCredentials(ctx, tx, keyring)
	if err != nil {
		slog.Error("credential rotation verification failed")
		os.Exit(1)
	}
	if !*execute {
		fmt.Printf("dry-run verified %d credential(s); no changes committed\n", count)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		slog.Error("could not commit credential rotation")
		os.Exit(1)
	}
	fmt.Printf("rotated %d credential(s) to key version %d\n", count, keyring.Version())
}
