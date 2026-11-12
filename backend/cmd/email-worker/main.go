package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"example.com/loja-3d/backend/internal/infrastructure/mail"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
)

func main() {
	if os.Getenv("EMAIL_PROVIDER") != "resend" || os.Getenv("EMAIL_DELIVERY_ENABLED") != "true" {
		slog.Error("external email delivery is disabled")
		os.Exit(1)
	}
	version64, e := strconv.ParseInt(os.Getenv("EMAIL_OUTBOX_KEY_VERSION"), 10, 16)
	if e != nil {
		version64 = 1
	}
	keys := map[int16]string{int16(version64): os.Getenv("EMAIL_OUTBOX_ENCRYPTION_KEY")}
	for _, entry := range strings.Split(os.Getenv("EMAIL_OUTBOX_DECRYPTION_KEYS"), ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 {
			continue
		}
		version, parseErr := strconv.ParseInt(parts[0], 10, 16)
		if parseErr == nil && version > 0 && parts[1] != "" {
			keys[int16(version)] = parts[1]
		}
	}
	cipher, e := mail.NewOutboxKeyring(int16(version64), keys)
	if e != nil {
		slog.Error("invalid email outbox encryption configuration")
		os.Exit(1)
	}
	if mail.ValidateAddress(os.Getenv("EMAIL_FROM")) != nil {
		slog.Error("invalid EMAIL_FROM")
		os.Exit(1)
	}
	config, e := postgres.LoadConfig()
	if e != nil {
		slog.Error("invalid database configuration")
		os.Exit(1)
	}
	pool, e := postgres.NewPool(context.Background(), config)
	if e != nil {
		slog.Error("database unavailable")
		os.Exit(1)
	}
	defer pool.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	messageRetention, e := time.ParseDuration(envOrDefault("EMAIL_MESSAGE_RETENTION", "720h"))
	if e != nil || messageRetention <= 0 {
		slog.Error("invalid EMAIL_MESSAGE_RETENTION")
		os.Exit(1)
	}
	eventRetention, e := time.ParseDuration(envOrDefault("EMAIL_EVENT_RETENTION", "2160h"))
	if e != nil || eventRetention <= 0 {
		slog.Error("invalid EMAIL_EVENT_RETENTION")
		os.Exit(1)
	}
	worker := mail.Worker{Outbox: mail.NewOutbox(pool, cipher), Sender: mail.ResendSender{BaseURL: "https://api.resend.com", APIKey: os.Getenv("RESEND_API_KEY"), Client: &http.Client{Timeout: 10 * time.Second}}, From: os.Getenv("EMAIL_FROM"), ReplyTo: os.Getenv("EMAIL_REPLY_TO"), Batch: 1, Rate: time.Second, MessageRetention: messageRetention, EventRetention: eventRetention}
	slog.Info("transactional email worker started")
	if e = worker.Run(ctx); e != nil {
		slog.Error("email worker stopped", "error", e)
		os.Exit(1)
	}
	slog.Info("transactional email worker stopped")
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
