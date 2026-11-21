package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"example.com/loja-3d/backend/internal/infrastructure/mail"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type deliveryConfig struct {
	Cipher                           *mail.OutboxCipher
	APIKey, From, ReplyTo            string
	MessageRetention, EventRetention time.Duration
}

func validateWorkerEnvironment(appEnv string) (string, error) {
	appEnv = strings.TrimSpace(appEnv)
	switch appEnv {
	case "development", "homologation", "production":
		return appEnv, nil
	default:
		return "", errors.New("APP_ENV must be explicitly set to development, homologation or production")
	}
}

func loadDeliveryConfig(getenv func(string) string) (deliveryConfig, error) {
	if getenv("EMAIL_PROVIDER") != "resend" || strings.TrimSpace(getenv("EMAIL_DELIVERY_ENABLED")) != "true" {
		return deliveryConfig{}, errors.New("external email delivery is not explicitly enabled")
	}
	apiKey := strings.TrimSpace(getenv("RESEND_API_KEY"))
	if apiKey == "" {
		return deliveryConfig{}, errors.New("RESEND_API_KEY is required")
	}
	from := strings.TrimSpace(getenv("EMAIL_FROM"))
	if mail.ValidateAddress(from) != nil {
		return deliveryConfig{}, errors.New("invalid EMAIL_FROM")
	}
	replyTo := strings.TrimSpace(getenv("EMAIL_REPLY_TO"))
	if replyTo != "" && mail.ValidateAddress(replyTo) != nil {
		return deliveryConfig{}, errors.New("invalid EMAIL_REPLY_TO")
	}
	cipher, err := mail.ParseOutboxKeyring(getenv("EMAIL_OUTBOX_KEY_VERSION"), getenv("EMAIL_OUTBOX_ENCRYPTION_KEY"), getenv("EMAIL_OUTBOX_DECRYPTION_KEYS"))
	if err != nil {
		return deliveryConfig{}, errors.New("invalid email outbox encryption configuration")
	}
	messageRetention, err := positiveDuration(getenv("EMAIL_MESSAGE_RETENTION"), 720*time.Hour)
	if err != nil {
		return deliveryConfig{}, errors.New("invalid EMAIL_MESSAGE_RETENTION")
	}
	eventRetention, err := positiveDuration(getenv("EMAIL_EVENT_RETENTION"), 2160*time.Hour)
	if err != nil {
		return deliveryConfig{}, errors.New("invalid EMAIL_EVENT_RETENTION")
	}
	return deliveryConfig{Cipher: cipher, APIKey: apiKey, From: from, ReplyTo: replyTo, MessageRetention: messageRetention, EventRetention: eventRetention}, nil
}

func positiveDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, errors.New("duration must be positive")
	}
	return value, nil
}

func prepareWorkerDatabase(appEnv string, databaseConfig postgres.Config) (*pgxpool.Config, postgres.DatabaseTarget, string, error) {
	if strings.TrimSpace(databaseConfig.DatabaseURL) == "" {
		return nil, postgres.DatabaseTarget{}, "rejected", errors.New("invalid database configuration")
	}
	poolConfig, err := postgres.BuildPoolConfig(databaseConfig)
	target := postgres.TargetFromPoolConfig(poolConfig)
	if err != nil {
		return nil, target, "rejected", errors.New("invalid database configuration")
	}
	if appEnv == "homologation" {
		if err = postgres.ValidateHomologationPoolConfig(poolConfig); err != nil {
			return nil, target, "rejected", errors.New("homologation database target rejected")
		}
		return poolConfig, target, "approved", nil
	}
	return poolConfig, target, "not_required", nil
}

type databaseIdentityReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func verifyWorkerDatabaseIdentity(ctx context.Context, reader databaseIdentityReader, expected postgres.DatabaseTarget) error {
	var database, user string
	if err := reader.QueryRow(ctx, `SELECT current_database(), current_user`).Scan(&database, &user); err != nil {
		return errors.New("could not verify connected database identity")
	}
	if database != expected.Database || user != expected.User {
		return errors.New("connected database identity differs from validated target")
	}
	return nil
}
