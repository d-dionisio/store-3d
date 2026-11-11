package middleware

import "net/http"

// CORS allows browser requests only from the configured frontend origin.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if request.Header.Get("Origin") == allowedOrigin {
				response.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				response.Header().Set("Access-Control-Allow-Credentials", "true")
				response.Header().Set("Vary", "Origin")
				response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token, Idempotency-Key")
			}

			if request.Method == http.MethodOptions {
				response.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(response, request)
		})
	}
}
