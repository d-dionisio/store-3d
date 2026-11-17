package main

import (
	"context"
	"example.com/loja-3d/backend/internal/infrastructure/bambulab"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	cfg, err := postgres.LoadConfig()
	if err != nil {
		slog.Error("invalid database configuration")
		os.Exit(1)
	}
	pool, err := postgres.NewPool(context.Background(), cfg)
	if err != nil {
		slog.Error("database initialization failed")
		os.Exit(1)
	}
	defer pool.Close()
	version64, err := strconv.ParseInt(os.Getenv("BAMBU_LAN_KEY_VERSION"), 10, 16)
	if err != nil {
		version64 = 1
	}
	cipher, err := bambulab.LoadKeyring(os.Getenv("BAMBU_LAN_ENCRYPTION_KEY"), int16(version64), os.Getenv("BAMBU_LAN_DECRYPTION_KEYS"))
	if err != nil {
		slog.Error("Bambu credential encryption is not configured")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store := bambulab.NewMonitorStore(pool, cipher)
	worker := bambulab.Worker{Store: store, Provider: bambulab.NewClient(), MaxConcurrent: 4, RetryMin: time.Second}
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = store.Cleanup(ctx, 30*24*time.Hour)
			}
		}
	}()
	if err = worker.Run(ctx); err != nil {
		slog.Error("printer monitor stopped")
		os.Exit(1)
	}
}
