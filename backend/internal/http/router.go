package http

import (
	"net/http"

	"example.com/loja-3d/backend/internal/http/handlers"
	"example.com/loja-3d/backend/internal/http/middleware"
)

// NewHandler assembles the API routes and cross-cutting middleware.
func NewHandler(frontendOrigin string) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /api/v1/health", handlers.Health)

	return middleware.CORS(frontendOrigin)(router)
}
