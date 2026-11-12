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
	editorialapp "example.com/loja-3d/backend/internal/application/editorial"
	paymentapp "example.com/loja-3d/backend/internal/application/payment"
	productionapp "example.com/loja-3d/backend/internal/application/production"
	quoteapp "example.com/loja-3d/backend/internal/application/quote"
	shippingapp "example.com/loja-3d/backend/internal/application/shipping"
	apihttp "example.com/loja-3d/backend/internal/http"
	"example.com/loja-3d/backend/internal/http/handlers"
	"example.com/loja-3d/backend/internal/infrastructure/mail"
	mercadopago "example.com/loja-3d/backend/internal/infrastructure/payment/mercadopago"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	melhorenvio "example.com/loja-3d/backend/internal/infrastructure/shipping/melhorenvio"
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
	adminService := adminapp.New(adminRepository)
	adminHandler := handlers.NewAdminHandler(adminService, envOrDefault("APP_ENV", "development") == "production")
	editorialService := editorialapp.New(postgres.NewEditorialRepository(pool), catalogRepository)
	editorialHandler := handlers.NewEditorialHandler(editorialService, adminService)
	appEnv := envOrDefault("APP_ENV", "development")
	var customerMailer customerapp.Mailer = mail.Unavailable{}
	if appEnv == "development" && envOrDefault("EMAIL_PROVIDER", "disabled") == "development" {
		customerMailer = mail.Development{}
	}
	customerRepository := postgres.NewCustomerRepository(pool)
	customerService := customerapp.New(customerRepository, customerMailer, envOrDefault("FRONTEND_ORIGIN", "http://localhost:3000"))
	customerHandler := handlers.NewCustomerHandler(customerService, appEnv == "production")
	shippingRepository := postgres.NewShippingRepository(pool)
	var shippingProvider shippingapp.Provider = melhorenvio.Unavailable{}
	shippingTimeout, parseErr := time.ParseDuration(envOrDefault("MELHOR_ENVIO_TIMEOUT", "8s"))
	if parseErr != nil || shippingTimeout <= 0 {
		shippingTimeout = 8 * time.Second
	}
	shippingClient, shippingErr := melhorenvio.New(
		envOrDefault("MELHOR_ENVIO_BASE_URL", "https://sandbox.melhorenvio.com.br"),
		os.Getenv("MELHOR_ENVIO_TOKEN"), os.Getenv("MELHOR_ENVIO_USER_AGENT"),
		&http.Client{Timeout: shippingTimeout},
	)
	if shippingErr == nil {
		shippingProvider = shippingClient
	} else {
		slog.Warn("Melhor Envio is not configured; shipping quotes are unavailable")
	}
	quoteTTL, parseErr := time.ParseDuration(envOrDefault("SHIPPING_QUOTE_TTL", "15m"))
	if parseErr != nil || quoteTTL <= 0 {
		quoteTTL = 15 * time.Minute
	}
	shippingService := shippingapp.New(shippingRepository, shippingProvider, os.Getenv("MELHOR_ENVIO_ORIGIN_POSTAL_CODE"), quoteTTL)
	shippingHandler := handlers.NewShippingHandler(shippingService, customerService, adminapp.New(adminRepository))
	paymentRepository := postgres.NewPaymentRepository(pool)
	var paymentProvider paymentapp.Provider = mercadopago.Unavailable{}
	mpEnvironment := envOrDefault("MERCADO_PAGO_ENVIRONMENT", "sandbox")
	mpTokenKey, mpSecretKey := "MERCADO_PAGO_SANDBOX_ACCESS_TOKEN", "MERCADO_PAGO_SANDBOX_WEBHOOK_SECRET"
	if mpEnvironment == "production" {
		mpTokenKey, mpSecretKey = "MERCADO_PAGO_PRODUCTION_ACCESS_TOKEN", "MERCADO_PAGO_PRODUCTION_WEBHOOK_SECRET"
	}
	mpTimeout, parseErr := time.ParseDuration(envOrDefault("MERCADO_PAGO_TIMEOUT", "8s"))
	if parseErr != nil || mpTimeout <= 0 {
		mpTimeout = 8 * time.Second
	}
	mpClient, mpErr := mercadopago.New("https://api.mercadopago.com", os.Getenv(mpTokenKey), os.Getenv(mpSecretKey), &http.Client{Timeout: mpTimeout})
	if mpErr == nil {
		paymentProvider = mpClient
	} else {
		slog.Warn("Mercado Pago is not configured; payments are unavailable", "environment", mpEnvironment)
	}
	mpTTL, parseErr := time.ParseDuration(envOrDefault("MERCADO_PAGO_PIX_TTL", "30m"))
	if parseErr != nil || mpTTL <= 0 {
		mpTTL = 30 * time.Minute
	}
	paymentService := paymentapp.New(paymentRepository, paymentProvider, mpTTL)
	paymentHandler := handlers.NewPaymentHandler(paymentService, customerService)
	productionService := productionapp.New(postgres.NewProductionRepository(pool))
	productionHandler := handlers.NewProductionHandler(productionService, adminService)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           apihttp.NewHandlerCompleteWithProduction(frontendOrigin, databaseHealth, catalogHandler, quoteHandler, adminHandler, customerHandler, shippingHandler, paymentHandler, productionHandler, editorialHandler),
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
