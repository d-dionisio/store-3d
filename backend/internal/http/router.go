package http

import (
	"net/http"

	"example.com/loja-3d/backend/internal/http/handlers"
	"example.com/loja-3d/backend/internal/http/middleware"
)

// NewHandler assembles the API routes and cross-cutting middleware.
func NewHandler(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /api/v1/health", handlers.Health)
	router.HandleFunc("GET /api/v1/ready", handlers.Ready(readiness))
	if catalog != nil {
		router.HandleFunc("GET /api/v1/products", catalog.ListProducts)
		router.HandleFunc("GET /api/v1/products/{slug}", catalog.GetProduct)
		router.HandleFunc("GET /api/v1/categories", catalog.ListCategories)
	}

	return middleware.CORS(frontendOrigin)(router)
}
