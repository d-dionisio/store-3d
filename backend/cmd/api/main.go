package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	catalogapp "example.com/loja-3d/backend/internal/application/catalog"
	customerapp "example.com/loja-3d/backend/internal/application/customer"
	quoteapp "example.com/loja-3d/backend/internal/application/quote"
	apihttp "example.com/loja-3d/backend/internal/http"
	"example.com/loja-3d/backend/internal/http/handlers"
	"example.com/loja-3d/backend/internal/infrastructure/mail"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"example.com/loja-3d/backend/internal/infrastructure/storage"
)

func main() {
	port := envOrDefault("HTTP_PORT", "8080")
	frontendOrigin := envOrDefault("FRONTEND_ORIGIN", "http://localhost:3000")
	databaseConfig, err := postgres.LoadConfig()
	if err != nil {
		slog.Error("invalid database configuration", "error", err)
		os.Exit(1)
	}

	pool, err := postgres.NewPool(context.Background(), databaseConfig)
	if err != nil {
		// Connection errors can contain endpoint metadata. Keep startup logs free of DSN details.
		slog.Error("database initialization failed")
		os.Exit(1)
	}
	defer pool.Close()
	databaseHealth := postgres.NewHealth(pool, databaseConfig.ConnectTimeout)
	catalogRepository := postgres.NewCatalogRepository(pool)
	catalogHandler := handlers.NewCatalogHandler(
		catalogapp.NewListProducts(catalogRepository),
		catalogapp.NewGetProductBySlug(catalogRepository),
		catalogapp.NewListCategories(catalogRepository),
	)
	quoteRepository := postgres.NewQuoteRepository(pool)
	r2, storageErr := storage.NewR2FromEnv()
	var quoteStorage quoteapp.Storage = storage.Unavailable{}
	bucket := envOrDefault("R2_BUCKET", "unconfigured")
	if storageErr == nil {
		quoteStorage = r2
		bucket = r2.Bucket()
	} else {
		slog.Warn("object storage is not configured; uploads are unavailable")
	}
	quoteHandler := handlers.NewQuoteHandler(quoteapp.New(quoteRepository, quoteStorage, bucket))
	adminRepository := postgres.NewAdminRepository(pool)
	adminHandler := handlers.NewAdminHandler(adminapp.New(adminRepository), envOrDefault("APP_ENV", "development") == "production")
	appEnv := envOrDefault("APP_ENV", "development")
	var customerMailer customerapp.Mailer = mail.Unavailable{}
	if appEnv == "development" && envOrDefault("EMAIL_PROVIDER", "disabled") == "development" {
		customerMailer = mail.Development{}
	}
	customerRepository := postgres.NewCustomerRepository(pool)
	customerService := customerapp.New(customerRepository, customerMailer, envOrDefault("FRONTEND_ORIGIN", "http://localhost:3000"))
	customerHandler := handlers.NewCustomerHandler(customerService, appEnv == "production")

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           apihttp.NewHandlerFull(frontendOrigin, databaseHealth, catalogHandler, quoteHandler, adminHandler, customerHandler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("API listening", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case signal := <-signals:
		slog.Info("shutdown signal received", "signal", signal.String())
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
		return
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("API stopped")
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
