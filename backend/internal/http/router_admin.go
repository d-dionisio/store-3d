package http

import (
	"example.com/loja-3d/backend/internal/http/handlers"
	"example.com/loja-3d/backend/internal/http/middleware"
	"net/http"
)

func NewHandler(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes ...*handlers.QuoteHandler) http.Handler {
	var q *handlers.QuoteHandler
	if len(quotes) > 0 {
		q = quotes[0]
	}
	return newHandler(frontendOrigin, readiness, catalog, q, nil)
}
func NewHandlerWithAdmin(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin)
}
func newHandler(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /api/v1/health", handlers.Health)
	router.HandleFunc("GET /api/v1/ready", handlers.Ready(readiness))
	if catalog != nil {
		router.HandleFunc("GET /api/v1/products", catalog.ListProducts)
		router.HandleFunc("GET /api/v1/products/{slug}", catalog.GetProduct)
		router.HandleFunc("GET /api/v1/categories", catalog.ListCategories)
	}
	if quotes != nil {
		router.HandleFunc("GET /api/v1/quote-options", quotes.Options)
		router.HandleFunc("POST /api/v1/quote-uploads", quotes.Initiate)
		router.HandleFunc("POST /api/v1/quote-uploads/{id}/confirm", quotes.Confirm)
		router.HandleFunc("POST /api/v1/quotes", quotes.Create)
		router.HandleFunc("GET /api/v1/quotes/{reference}", quotes.Get)
	}
	if admin != nil {
		router.HandleFunc("POST /api/v1/admin/session", admin.Login)
		router.HandleFunc("DELETE /api/v1/admin/session", admin.Logout)
		router.HandleFunc("GET /api/v1/admin/dashboard", admin.Stats)
		router.HandleFunc("GET /api/v1/admin/quotes", admin.List)
		router.HandleFunc("GET /api/v1/admin/quotes/{reference}", admin.Get)
		router.HandleFunc("POST /api/v1/admin/quotes/{reference}/analysis", admin.StartAnalysis)
		router.HandleFunc("PUT /api/v1/admin/quotes/{reference}/pricing", admin.SavePricing)
		router.HandleFunc("POST /api/v1/admin/quotes/{reference}/proposal", admin.SendProposal)
		router.HandleFunc("POST /api/v1/admin/quotes/expire", admin.Expire)
	}
	return middleware.CORS(frontendOrigin)(router)
}
