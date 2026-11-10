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
	return newHandler(frontendOrigin, readiness, catalog, q, nil, nil, nil, nil)
}
func NewHandlerWithAdmin(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, nil, nil, nil)
}
func NewHandlerFull(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, customer, nil, nil)
}
func NewHandlerWithShipping(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler, shipping *handlers.ShippingHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, customer, shipping, nil)
}
func NewHandlerComplete(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler, shipping *handlers.ShippingHandler, payment *handlers.PaymentHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, customer, shipping, payment)
}
func newHandler(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler, shipping *handlers.ShippingHandler, payment *handlers.PaymentHandler) http.Handler {
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
	if customer != nil {
		router.HandleFunc("POST /api/v1/customer/magic-links", customer.RequestLink)
		router.HandleFunc("POST /api/v1/customer/magic-links/verify", customer.Verify)
		router.HandleFunc("DELETE /api/v1/customer/session", customer.Logout)
		router.HandleFunc("GET /api/v1/customer/profile", customer.Profile)
		router.HandleFunc("GET /api/v1/customer/quotes", customer.List)
		router.HandleFunc("GET /api/v1/customer/quotes/{reference}", customer.Get)
		router.HandleFunc("POST /api/v1/customer/quotes/{reference}/decision", customer.Decide)
		router.HandleFunc("POST /api/v1/customer/quotes/{reference}/convert", customer.Convert)
		router.HandleFunc("POST /api/v1/customer/cart/validate", customer.ValidateCart)
		router.HandleFunc("POST /api/v1/customer/orders/catalog", customer.CreateCatalogOrder)
		router.HandleFunc("GET /api/v1/customer/addresses", customer.ListAddresses)
		router.HandleFunc("POST /api/v1/customer/addresses", customer.CreateAddress)
		router.HandleFunc("PUT /api/v1/customer/orders/{reference}/delivery-address", customer.UpdateOrderAddress)
		router.HandleFunc("POST /api/v1/customer/orders/{reference}/prepare-payment", customer.PrepareOrder)
		router.HandleFunc("GET /api/v1/customer/orders", customer.ListOrders)
		router.HandleFunc("GET /api/v1/customer/orders/{reference}", customer.GetOrder)
	}
	if shipping != nil {
		router.HandleFunc("POST /api/v1/customer/orders/{reference}/shipping-quotes", shipping.Quote)
		router.HandleFunc("POST /api/v1/customer/orders/{reference}/shipping-selection", shipping.Select)
		router.HandleFunc("PUT /api/v1/admin/orders/{reference}/packages", shipping.SavePackages)
	}
	if payment != nil {
		router.HandleFunc("POST /api/v1/customer/orders/{reference}/payments/pix", payment.CreatePIX)
		router.HandleFunc("GET /api/v1/customer/orders/{reference}/payment", payment.Status)
		router.HandleFunc("POST /api/v1/webhooks/mercado-pago", payment.Webhook)
	}
	return middleware.CORS(frontendOrigin)(router)
}
