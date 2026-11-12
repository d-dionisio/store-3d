package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/loja-3d/backend/internal/http/gateway"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func loopbackAddress(value string) bool {
	host, _, err := net.SplitHostPort(value)
	return err == nil && net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func main() {
	next, err := gateway.ParseUpstream(env("GATEWAY_NEXT_UPSTREAM", "http://127.0.0.1:3000"))
	if err != nil {
		slog.Error("invalid Next upstream")
		os.Exit(1)
	}
	api, err := gateway.ParseUpstream(env("GATEWAY_API_UPSTREAM", "http://127.0.0.1:8080"))
	if err != nil {
		slog.Error("invalid API upstream")
		os.Exit(1)
	}
	publicAddress, adminAddress := env("PUBLIC_GATEWAY_ADDR", "127.0.0.1:8081"), env("ADMIN_GATEWAY_ADDR", "127.0.0.1:8082")
	if !loopbackAddress(publicAddress) || !loopbackAddress(adminAddress) {
		slog.Error("gateways must bind to explicit loopback addresses")
		os.Exit(1)
	}
	appEnv, localToken := env("APP_ENV", "development"), os.Getenv("ZERO_TRUST_LOCAL_TOKEN")
	if appEnv != "production" && len(localToken) < 24 {
		slog.Error("ZERO_TRUST_LOCAL_TOKEN must contain at least 24 characters outside production")
		os.Exit(1)
	}
	if appEnv == "production" && localToken != "" {
		slog.Error("local Zero Trust token is forbidden in production")
		os.Exit(1)
	}
	config := gateway.Config{NextUpstream: next, APIUpstream: api, PublicHost: os.Getenv("PUBLIC_HOSTNAME"), AdminHost: os.Getenv("ADMIN_HOSTNAME"), LocalAccessToken: localToken}
	publicServer := &http.Server{Addr: publicAddress, Handler: gateway.Public(config), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	adminServer := &http.Server{Addr: adminAddress, Handler: gateway.Admin(config), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	errorsChannel := make(chan error, 2)
	go func() {
		slog.Info("public gateway listening", "address", publicAddress)
		errorsChannel <- publicServer.ListenAndServe()
	}()
	go func() {
		slog.Info("admin gateway listening", "address", adminAddress)
		errorsChannel <- adminServer.ListenAndServe()
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-signals:
	case err = <-errorsChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("gateway stopped", "error", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = publicServer.Shutdown(ctx)
	_ = adminServer.Shutdown(ctx)
}
