package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	catalogapp "example.com/loja-3d/backend/internal/application/catalog"
	customerapp "example.com/loja-3d/backend/internal/application/customer"
	editorialapp "example.com/loja-3d/backend/internal/application/editorial"
	identityapp "example.com/loja-3d/backend/internal/application/identity"
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
	securityauth "example.com/loja-3d/backend/internal/security/auth"
)

func main() {
	port := envOrDefault("HTTP_PORT", "8080")
	host := envOrDefault("HTTP_HOST", "127.0.0.1")
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
	appEnv := envOrDefault("APP_ENV", "development")
	emailPublicURL := envOrDefault("EMAIL_PUBLIC_BASE_URL", envOrDefault("SITE_URL", "http://localhost:3000"))
	if parsed, parseURLErr := url.Parse(emailPublicURL); appEnv == "production" && (parseURLErr != nil || parsed.Scheme != "https" || parsed.Host == "") {
		slog.Error("EMAIL_PUBLIC_BASE_URL must be an absolute HTTPS URL in production")
		os.Exit(1)
	}
	var identityMailer identityapp.Mailer = mail.Unavailable{}
	var customerMailer customerapp.Mailer = mail.Unavailable{}
	var emailOutbox *mail.Outbox
	emailKeyVersion, emailVersionErr := strconv.ParseInt(envOrDefault("EMAIL_OUTBOX_KEY_VERSION", "1"), 10, 16)
	emailKeys := map[int16]string{int16(emailKeyVersion): os.Getenv("EMAIL_OUTBOX_ENCRYPTION_KEY")}
	for _, entry := range strings.Split(os.Getenv("EMAIL_OUTBOX_DECRYPTION_KEYS"), ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 {
			continue
		}
		version, parseKeyErr := strconv.ParseInt(parts[0], 10, 16)
		if parseKeyErr == nil && version > 0 && parts[1] != "" {
			emailKeys[int16(version)] = parts[1]
		}
	}
	emailCipher, emailCipherErr := mail.NewOutboxKeyring(int16(emailKeyVersion), emailKeys)
	if emailVersionErr == nil && emailCipherErr == nil {
		emailOutbox = mail.NewOutbox(pool, emailCipher)
		customerMailer = emailOutbox
		identityMailer = emailOutbox
	} else if appEnv == "development" && envOrDefault("EMAIL_PROVIDER", "disabled") == "development" {
		developmentMailer := mail.Development{}
		customerMailer = developmentMailer
		identityMailer = developmentMailer
	}
	var identityService *identityapp.Service
	mfaVersionValue, versionErr := strconv.ParseInt(envOrDefault("AUTH_MFA_KEY_VERSION", "1"), 10, 16)
	if versionErr != nil || mfaVersionValue < 1 {
		slog.Error("invalid MFA key version")
		os.Exit(1)
	}
	mfaKeyVersion := int16(mfaVersionValue)
	mfaKeys := map[int16]string{mfaKeyVersion: os.Getenv("AUTH_MFA_ENCRYPTION_KEY")}
	for _, entry := range strings.Split(os.Getenv("AUTH_MFA_DECRYPTION_KEYS"), ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 {
			continue
		}
		version, parseKeyErr := strconv.ParseInt(parts[0], 10, 16)
		if parseKeyErr == nil && version > 0 && parts[1] != "" {
			mfaKeys[int16(version)] = parts[1]
		}
	}
	mfaCipher, cipherErr := securityauth.NewKeyring(mfaKeyVersion, mfaKeys)
	if cipherErr == nil {
		identityRepository := postgres.NewIdentityRepository(pool)
		identityRepository.SetEmailOutbox(emailOutbox)
		identityService = identityapp.New(identityRepository, identityMailer, mfaCipher, emailPublicURL)
		if reauthTTL, ttlErr := time.ParseDuration(envOrDefault("AUTH_REAUTH_TTL", "5m")); ttlErr == nil {
			identityService.SetReauthenticationTTL(reauthTTL)
		}
		if breachURL := strings.TrimSpace(os.Getenv("PASSWORD_BREACH_CHECK_URL")); breachURL != "" {
			identityService.SetPasswordChecker(securityauth.KAnonymityChecker{BaseURL: breachURL, Client: &http.Client{Timeout: 3 * time.Second}, FailClosed: envOrDefault("PASSWORD_BREACH_FAILURE_MODE", "allow") == "deny"})
		}
	} else if appEnv == "production" {
		slog.Error("MFA encryption configuration is required in production")
		os.Exit(1)
	} else {
		slog.Warn("MFA encryption is not configured; password and administrative login endpoints fail closed")
	}
	adminHandler := handlers.NewAdminHandler(adminService, appEnv == "production", identityService)
	adminRepository.SetEmailOutbox(emailOutbox)
	editorialService := editorialapp.New(postgres.NewEditorialRepository(pool), catalogRepository)
	editorialHandler := handlers.NewEditorialHandler(editorialService, adminService)
	customerRepository := postgres.NewCustomerRepository(pool)
	customerRepository.SetEmailOutbox(emailOutbox)
	customerService := customerapp.New(customerRepository, customerMailer, emailPublicURL)
	customerHandler := handlers.NewCustomerHandler(customerService, appEnv == "production", identityService)
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
	paymentRepository.SetEmailOutbox(emailOutbox)
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

	baseHandler := apihttp.NewHandlerCompleteWithProduction(frontendOrigin, databaseHealth, catalogHandler, quoteHandler, adminHandler, customerHandler, shippingHandler, paymentHandler, productionHandler, editorialHandler)
	rootHandler := http.NewServeMux()
	rootHandler.HandleFunc("POST /api/v1/webhooks/resend", mail.ResendWebhook(pool, os.Getenv("RESEND_WEBHOOK_SECRET")))
	rootHandler.Handle("/", baseHandler)
	server := &http.Server{
		Addr:              net.JoinHostPort(host, port),
		Handler:           rootHandler,
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
