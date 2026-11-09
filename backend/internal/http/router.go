package http

import (
	"net/http"

	"example.com/loja-3d/backend/internal/http/handlers"
	"example.com/loja-3d/backend/internal/http/middleware"
)

// NewHandler assembles the API routes and cross-cutting middleware.
func NewHandler(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes ...*handlers.QuoteHandler) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /api/v1/health", handlers.Health)
	router.HandleFunc("GET /api/v1/ready", handlers.Ready(readiness))
	if catalog != nil {
		router.HandleFunc("GET /api/v1/products", catalog.ListProducts)
		router.HandleFunc("GET /api/v1/products/{slug}", catalog.GetProduct)
		router.HandleFunc("GET /api/v1/categories", catalog.ListCategories)
	}
	if len(quotes) > 0 && quotes[0] != nil {
		router.HandleFunc("GET /api/v1/quote-options", quotes[0].Options)
		router.HandleFunc("POST /api/v1/quote-uploads", quotes[0].Initiate)
		router.HandleFunc("POST /api/v1/quote-uploads/{id}/confirm", quotes[0].Confirm)
		router.HandleFunc("POST /api/v1/quotes", quotes[0].Create)
		router.HandleFunc("GET /api/v1/quotes/{reference}", quotes[0].Get)
	}

	return middleware.CORS(frontendOrigin)(router)
}
