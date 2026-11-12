package gateway

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

type Config struct {
	NextUpstream, APIUpstream *url.URL
	PublicHost, AdminHost     string
	LocalAccessToken          string
}

func ParseUpstream(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" {
		return nil, errors.New("invalid upstream")
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return nil, errors.New("upstream must use an explicit loopback IP and port")
	}
	return u, nil
}

func proxy(upstream *url.URL) http.Handler {
	p := httputil.NewSingleHostReverseProxy(upstream)
	director := p.Director
	p.Director = func(r *http.Request) {
		director(r)
		r.Header.Del("X-Local-Zero-Trust")
		r.Header.Del("Authorization")
	}
	p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
	return p
}

func normalized(raw string) (string, bool) {
	if raw == "" {
		return "/", true
	}
	if strings.ContainsAny(raw, "\\\x00") {
		return "", false
	}
	current := raw
	for range 3 {
		decoded, err := url.PathUnescape(current)
		if err != nil || strings.ContainsAny(decoded, "\\\x00") {
			return "", false
		}
		if decoded == current {
			break
		}
		current = decoded
	}
	if strings.Contains(current, "%") {
		return "", false
	}
	clean := path.Clean("/" + strings.TrimLeft(current, "/"))
	return clean, true
}

func adminPath(p string) bool {
	return p == "/admin" || strings.HasPrefix(p, "/admin/") || p == "/api/v1/admin" || strings.HasPrefix(p, "/api/v1/admin/")
}
func publicAPIPath(p string) bool {
	exact := map[string]bool{"/api/v1/health": true, "/api/v1/ready": true, "/api/v1/products": true, "/api/v1/categories": true, "/api/v1/editorial-catalogs": true, "/api/v1/quote-options": true}
	if exact[p] {
		return true
	}
	for _, prefix := range []string{"/api/v1/products", "/api/v1/editorial-catalogs", "/api/v1/quote-uploads", "/api/v1/quotes", "/api/v1/customer"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
func hostAllowed(requestHost, configured string) bool {
	if configured == "" {
		return true
	}
	host := requestHost
	if parsed, _, err := net.SplitHostPort(requestHost); err == nil {
		host = parsed
	}
	return strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(configured, "."))
}

func Public(config Config) http.Handler {
	nextProxy, apiProxy := proxy(config.NextUpstream), proxy(config.APIUpstream)
	return secureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host, config.PublicHost) {
			http.NotFound(w, r)
			return
		}
		p, ok := normalized(r.URL.EscapedPath())
		if !ok || adminPath(p) {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(p, "/api/") {
			if !publicAPIPath(p) {
				http.NotFound(w, r)
				return
			}
			r.URL.Path, r.URL.RawPath = p, ""
			apiProxy.ServeHTTP(w, r)
			return
		}
		r.URL.Path, r.URL.RawPath = p, ""
		nextProxy.ServeHTTP(w, r)
	}))
}

func Admin(config Config) http.Handler {
	nextProxy, apiProxy := proxy(config.NextUpstream), proxy(config.APIUpstream)
	return secureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host, config.AdminHost) {
			http.NotFound(w, r)
			return
		}
		if config.LocalAccessToken != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Local-Zero-Trust")), []byte(config.LocalAccessToken)) != 1 {
			http.Error(w, "zero trust access required", http.StatusUnauthorized)
			return
		}
		p, ok := normalized(r.URL.EscapedPath())
		if !ok {
			http.NotFound(w, r)
			return
		}
		if p == "/" {
			http.Redirect(w, r, "/admin", http.StatusTemporaryRedirect)
			return
		}
		if p == "/api/v1/admin" || strings.HasPrefix(p, "/api/v1/admin/") {
			r.URL.Path, r.URL.RawPath = p, ""
			apiProxy.ServeHTTP(w, r)
			return
		}
		if p == "/admin" || strings.HasPrefix(p, "/admin/") || strings.HasPrefix(p, "/_next/") || strings.HasPrefix(p, "/images/") || p == "/favicon.ico" {
			r.URL.Path, r.URL.RawPath = p, ""
			nextProxy.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	}))
}
