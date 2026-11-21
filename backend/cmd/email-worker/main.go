package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/loja-3d/backend/internal/infrastructure/mail"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
)

func main() {
	appEnv, e := validateWorkerEnvironment(os.Getenv("APP_ENV"))
	if e != nil {
		slog.Error("invalid transactional email worker environment", "reason", e.Error(), "guard_result", "rejected")
		os.Exit(1)
	}
	delivery, e := loadDeliveryConfig(os.Getenv)
	if e != nil {
		slog.Error("invalid transactional email worker configuration", "reason", e.Error())
		os.Exit(1)
	}
	databaseConfig, e := postgres.LoadConfig()
	if e != nil {
		slog.Error("invalid database configuration")
		os.Exit(1)
	}
	poolConfig, target, guardResult, e := prepareWorkerDatabase(appEnv, databaseConfig)
	slog.Info("email worker database startup attestation", "app_env", appEnv, "database_host", target.Host, "database_port", target.Port, "database_name", target.Database, "database_user", target.User, "guard_result", guardResult)
	if e != nil {
		slog.Error("email worker database target rejected", "reason", e.Error())
		os.Exit(1)
	}
	pool, e := postgres.NewPoolWithConfig(context.Background(), databaseConfig, poolConfig)
	if e != nil {
		slog.Error("database unavailable")
		os.Exit(1)
	}
	defer pool.Close()
	if e = verifyWorkerDatabaseIdentity(context.Background(), pool, target); e != nil {
		slog.Error("email worker database startup attestation", "app_env", appEnv, "database_host", target.Host, "database_port", target.Port, "database_name", target.Database, "database_user", target.User, "guard_result", "rejected", "connection_result", "rejected")
		slog.Error("email worker database identity validation failed", "reason", e.Error())
		os.Exit(1)
	}
	slog.Info("email worker database connection verified", "app_env", appEnv, "database_host", target.Host, "database_port", target.Port, "database_name", target.Database, "database_user", target.User, "guard_result", guardResult, "connection_result", "verified")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	worker := mail.Worker{Outbox: mail.NewOutbox(pool, delivery.Cipher), Sender: mail.ResendSender{BaseURL: "https://api.resend.com", APIKey: delivery.APIKey, Client: &http.Client{Timeout: 10 * time.Second}}, From: delivery.From, ReplyTo: delivery.ReplyTo, Batch: 1, Rate: time.Second, MessageRetention: delivery.MessageRetention, EventRetention: delivery.EventRetention}
	if e = worker.Run(ctx); e != nil {
		slog.Error("email worker stopped", "error_type", "run_failed")
		os.Exit(1)
	}
}
