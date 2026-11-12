package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	cipher, e := mail.NewOutboxCipher(os.Getenv("EMAIL_OUTBOX_ENCRYPTION_KEY"), int16(version64))
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
	worker := mail.Worker{Outbox: mail.NewOutbox(pool, cipher), Sender: mail.ResendSender{BaseURL: "https://api.resend.com", APIKey: os.Getenv("RESEND_API_KEY"), Client: &http.Client{Timeout: 10 * time.Second}}, From: os.Getenv("EMAIL_FROM"), ReplyTo: os.Getenv("EMAIL_REPLY_TO"), Batch: 1, Rate: time.Second}
	slog.Info("transactional email worker started")
	if e = worker.Run(ctx); e != nil {
		slog.Error("email worker stopped", "error", e)
		os.Exit(1)
	}
	slog.Info("transactional email worker stopped")
}
