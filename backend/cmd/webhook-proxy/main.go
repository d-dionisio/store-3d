package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	webhookPath    = "/api/v1/webhooks/mercado-pago"
	maxWebhookBody = 64 << 10
)

type fixedWindowLimiter struct {
	mu       sync.Mutex
	window   time.Time
	requests map[string]int
	limit    int
	now      func() time.Time
}

func newLimiter(limit int) *fixedWindowLimiter {
	return &fixedWindowLimiter{window: time.Now(), requests: make(map[string]int), limit: limit, now: time.Now}
}

func (l *fixedWindowLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.window) >= time.Minute {
		l.window = now
		clear(l.requests)
	}
	if l.requests[key] >= l.limit {
		return false
	}
	l.requests[key]++
	return true
}

func newHandler(upstream *url.URL, limit int) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		// Browser/customer credentials are never needed by this public route.
		r.Header.Del("Authorization")
		r.Header.Del("Cookie")
	}
	limiter := newLimiter(limit)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != webhookPath || r.URL.EscapedPath() != webhookPath {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.ContentLength > maxWebhookBody {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		if !limiter.allow(remoteIP(r.RemoteAddr)) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBody)
		proxy.ServeHTTP(w, r)
	})
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func main() {
	port := envOrDefault("WEBHOOK_PROXY_PORT", "8090")
	upstream, err := parseLoopbackUpstream(envOrDefault("WEBHOOK_PROXY_UPSTREAM", "http://127.0.0.1:8080"))
	if err != nil {
		slog.Error("invalid webhook proxy configuration", "error", err)
		os.Exit(1)
	}
	limit, err := strconv.Atoi(envOrDefault("WEBHOOK_PROXY_RATE_PER_MINUTE", "60"))
	if err != nil || limit < 1 {
		slog.Error("invalid webhook proxy rate limit")
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newHandler(upstream, limit),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		slog.Info("restricted webhook proxy listening", "address", server.Addr, "allowed_path", webhookPath)
		errorsChannel <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case <-signals:
	case err := <-errorsChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("webhook proxy stopped unexpectedly", "error", err)
			os.Exit(1)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("webhook proxy shutdown failed")
		os.Exit(1)
	}
}

func parseLoopbackUpstream(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("WEBHOOK_PROXY_UPSTREAM must be a plain loopback HTTP origin")
	}
	host := strings.Trim(strings.ToLower(u.Hostname()), "[]")
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("WEBHOOK_PROXY_UPSTREAM must use localhost or a loopback IP")
	}
	return u, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
