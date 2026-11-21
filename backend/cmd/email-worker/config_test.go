package main

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"github.com/jackc/pgx/v5"
)

type identityTestRow struct {
	database string
	user     string
	err      error
}

func (r identityTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*string)) = r.database
	*(dest[1].(*string)) = r.user
	return nil
}

type identityTestReader struct{ row identityTestRow }

func (r identityTestReader) QueryRow(context.Context, string, ...any) pgx.Row { return r.row }

func TestLoadDeliveryConfig(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	base := map[string]string{
		"EMAIL_PROVIDER": "resend", "EMAIL_DELIVERY_ENABLED": "true",
		"RESEND_API_KEY": "unit-test-key", "EMAIL_FROM": "Studio <noreply@example.test>",
		"EMAIL_OUTBOX_KEY_VERSION": "2", "EMAIL_OUTBOX_ENCRYPTION_KEY": key,
	}
	getenv := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	cfg, err := loadDeliveryConfig(getenv(base))
	if err != nil {
		t.Fatal("valid delivery configuration rejected")
	}
	if cfg.Cipher.Version != 2 || cfg.APIKey != "unit-test-key" || cfg.MessageRetention != 720*time.Hour || cfg.EventRetention != 2160*time.Hour {
		t.Fatal("valid delivery configuration was not parsed as expected")
	}

	cases := []struct {
		name string
		edit func(map[string]string)
	}{
		{"provider disabled", func(v map[string]string) { v["EMAIL_PROVIDER"] = "disabled" }},
		{"delivery disabled", func(v map[string]string) { v["EMAIL_DELIVERY_ENABLED"] = "false" }},
		{"missing api key", func(v map[string]string) { v["RESEND_API_KEY"] = " " }},
		{"invalid from", func(v map[string]string) { v["EMAIL_FROM"] = "not-an-address" }},
		{"invalid active version", func(v map[string]string) { v["EMAIL_OUTBOX_KEY_VERSION"] = "0" }},
		{"invalid active key", func(v map[string]string) { v["EMAIL_OUTBOX_ENCRYPTION_KEY"] = "not-base64" }},
		{"invalid historical key", func(v map[string]string) { v["EMAIL_OUTBOX_DECRYPTION_KEYS"] = "broken" }},
		{"invalid retention", func(v map[string]string) { v["EMAIL_MESSAGE_RETENTION"] = "0s" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := make(map[string]string, len(base))
			for k, v := range base {
				values[k] = v
			}
			tc.edit(values)
			if _, err := loadDeliveryConfig(getenv(values)); err == nil {
				t.Fatal("invalid delivery configuration accepted")
			}
		})
	}
}

func TestValidateWorkerEnvironment(t *testing.T) {
	for _, value := range []string{"development", "homologation", "production"} {
		if parsed, err := validateWorkerEnvironment(value); err != nil || parsed != value {
			t.Fatalf("supported APP_ENV %q rejected", value)
		}
	}
	for _, value := range []string{"", "homologaton", "Production", "test"} {
		if _, err := validateWorkerEnvironment(value); err == nil {
			t.Fatalf("unsupported or missing APP_ENV %q accepted", value)
		}
	}
}

func TestPrepareWorkerDatabaseHomologationGuard(t *testing.T) {
	valid := "postgres://formo3d_test:secret@localhost:5433/loja_3d_test?sslmode=disable"
	if _, target, result, err := prepareWorkerDatabase("homologation", postgres.Config{DatabaseURL: valid}); err != nil || result != "approved" || target.Database != "loja_3d_test" || target.User != "formo3d_test" {
		t.Fatal("isolated homologation target rejected")
	}
	for _, dsn := range []string{
		"postgres://formo3d_test:secret@localhost:5432/loja_3d_test",
		"postgres://formo3d_test:secret@localhost:5433/loja_3d",
		"postgres://other:secret@localhost:5433/loja_3d_test",
		"postgres://formo3d_test:secret@example.invalid:5433/loja_3d_test",
		"postgres://formo3d_test:secret@localhost:5433/loja_3d_test?host=example.invalid",
	} {
		if _, _, result, err := prepareWorkerDatabase("homologation", postgres.Config{DatabaseURL: dsn}); err == nil || result != "rejected" {
			t.Fatal("invalid homologation database target accepted")
		}
	}
	if _, _, result, err := prepareWorkerDatabase("development", postgres.Config{DatabaseURL: valid}); err != nil || result != "not_required" {
		t.Fatal("non-homologation database behavior changed")
	}
	if _, _, _, err := prepareWorkerDatabase("homologation", postgres.Config{}); err == nil || !strings.Contains(err.Error(), "invalid database configuration") {
		t.Fatal("missing database URL accepted")
	}
}

func TestVerifyWorkerDatabaseIdentity(t *testing.T) {
	expected := postgres.DatabaseTarget{Database: "loja_3d_test", User: "formo3d_test"}
	if err := verifyWorkerDatabaseIdentity(context.Background(), identityTestReader{identityTestRow{database: expected.Database, user: expected.User}}, expected); err != nil {
		t.Fatal("expected isolated database identity rejected")
	}
	for _, row := range []identityTestRow{
		{database: "loja_3d", user: "formo3d_test"},
		{database: "loja_3d_test", user: "wrong_user"},
		{err: errors.New("private driver error")},
	} {
		if err := verifyWorkerDatabaseIdentity(context.Background(), identityTestReader{row}, expected); err == nil || strings.Contains(err.Error(), "private driver error") {
			t.Fatal("mismatched or unverified database identity was accepted or leaked driver detail")
		}
	}
}
