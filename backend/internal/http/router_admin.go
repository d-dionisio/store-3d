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
	return newHandler(frontendOrigin, readiness, catalog, q, nil, nil, nil, nil, nil)
}
func NewHandlerWithAdmin(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, nil, nil, nil, nil)
}
func NewHandlerFull(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, customer, nil, nil, nil)
}
func NewHandlerWithShipping(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler, shipping *handlers.ShippingHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, customer, shipping, nil, nil)
}
func NewHandlerComplete(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler, shipping *handlers.ShippingHandler, payment *handlers.PaymentHandler, editorial ...*handlers.EditorialHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, customer, shipping, payment, nil, editorial...)
}
func NewHandlerCompleteWithProduction(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler, shipping *handlers.ShippingHandler, payment *handlers.PaymentHandler, production *handlers.ProductionHandler, editorial ...*handlers.EditorialHandler) http.Handler {
	return newHandler(frontendOrigin, readiness, catalog, quotes, admin, customer, shipping, payment, production, editorial...)
}
func newHandler(frontendOrigin string, readiness handlers.ReadinessChecker, catalog *handlers.CatalogHandler, quotes *handlers.QuoteHandler, admin *handlers.AdminHandler, customer *handlers.CustomerHandler, shipping *handlers.ShippingHandler, payment *handlers.PaymentHandler, production *handlers.ProductionHandler, editorial ...*handlers.EditorialHandler) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /api/v1/health", handlers.Health)
	router.HandleFunc("GET /api/v1/ready", handlers.Ready(readiness))
	if catalog != nil {
		router.HandleFunc("GET /api/v1/products", catalog.ListProducts)
		router.HandleFunc("GET /api/v1/products/{slug}", catalog.GetProduct)
		router.HandleFunc("GET /api/v1/categories", catalog.ListCategories)
	}
	if len(editorial) > 0 && editorial[0] != nil {
		h := editorial[0]
		router.HandleFunc("GET /api/v1/editorial-catalogs", h.PublicList)
		router.HandleFunc("GET /api/v1/editorial-catalogs/{slug}", h.PublicGet)
		router.HandleFunc("GET /api/v1/admin/catalogs", h.AdminList)
		router.HandleFunc("POST /api/v1/admin/catalogs", h.Create)
		router.HandleFunc("GET /api/v1/admin/catalogs/{id}", h.AdminGet)
		router.HandleFunc("PUT /api/v1/admin/catalogs/{id}", h.Update)
		router.HandleFunc("PUT /api/v1/admin/catalogs/{id}/products", h.ReplaceProducts)
		router.HandleFunc("POST /api/v1/admin/catalogs/{id}/products", h.AddProduct)
		router.HandleFunc("DELETE /api/v1/admin/catalogs/{id}/products/{productID}", h.RemoveProduct)
		router.HandleFunc("POST /api/v1/admin/catalogs/{id}/publish", h.Publish)
		router.HandleFunc("POST /api/v1/admin/catalogs/{id}/unpublish", h.Unpublish)
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
		router.HandleFunc("POST /api/v1/admin/session/mfa", admin.CompleteIdentityMFA)
		router.HandleFunc("GET /api/v1/admin/security", admin.SecurityStatus)
		router.HandleFunc("POST /api/v1/admin/security/recovery-codes", admin.RegenerateRecoveryCodes)
		router.HandleFunc("DELETE /api/v1/admin/session", admin.Logout)
		router.HandleFunc("GET /api/v1/admin/users", admin.ListInternalUsers)
		router.HandleFunc("POST /api/v1/admin/users", admin.CreateInternalUser)
		router.HandleFunc("PATCH /api/v1/admin/users/{id}", admin.UpdateInternalUser)
		router.HandleFunc("PUT /api/v1/admin/users/{id}/password", admin.ResetInternalUserPassword)
		router.HandleFunc("POST /api/v1/admin/users/{id}/sessions/revoke", admin.RevokeInternalUserSessions)
		router.HandleFunc("GET /api/v1/admin/dashboard", admin.Stats)
		router.HandleFunc("GET /api/v1/admin/quotes", admin.List)
		router.HandleFunc("GET /api/v1/admin/quotes/{reference}", admin.Get)
		router.HandleFunc("POST /api/v1/admin/quotes/{reference}/analysis", admin.StartAnalysis)
		router.HandleFunc("PUT /api/v1/admin/quotes/{reference}/pricing", admin.SavePricing)
		router.HandleFunc("POST /api/v1/admin/quotes/{reference}/proposal", admin.SendProposal)
		router.HandleFunc("POST /api/v1/admin/quotes/expire", admin.Expire)
	}
	if customer != nil {
		router.HandleFunc("POST /api/v1/customer/register", customer.Register)
		router.HandleFunc("POST /api/v1/customer/session", customer.PasswordLogin)
		router.HandleFunc("POST /api/v1/customer/session/mfa", customer.CompleteIdentityMFA)
		router.HandleFunc("POST /api/v1/customer/email/verify", customer.VerifyEmail)
		router.HandleFunc("POST /api/v1/customer/email/resend", customer.ResendVerification)
		router.HandleFunc("POST /api/v1/customer/password/forgot", customer.RequestPasswordReset)
		router.HandleFunc("POST /api/v1/customer/password/reset", customer.ResetPassword)
		router.HandleFunc("PUT /api/v1/customer/password", customer.SetPassword)
		router.HandleFunc("GET /api/v1/customer/security", customer.SecurityStatus)
		router.HandleFunc("POST /api/v1/customer/security/mfa", customer.BeginMFA)
		router.HandleFunc("POST /api/v1/customer/security/mfa/confirm", customer.CompleteIdentityMFA)
		router.HandleFunc("DELETE /api/v1/customer/security/mfa", customer.DisableMFA)
		router.HandleFunc("POST /api/v1/customer/security/recovery-codes", customer.RegenerateRecoveryCodes)
		router.HandleFunc("POST /api/v1/customer/sessions/revoke", customer.RevokeAllSessions)
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
		router.HandleFunc("POST /api/v1/customer/orders/{reference}/payments/card", payment.CreateCard)
		router.HandleFunc("GET /api/v1/customer/orders/{reference}/payment", payment.Status)
		router.HandleFunc("POST /api/v1/webhooks/mercado-pago", payment.Webhook)
	}
	if production != nil {
		router.HandleFunc("GET /api/v1/admin/production/jobs", production.ListJobs)
		router.HandleFunc("POST /api/v1/admin/production/orders/{reference}/jobs", production.CreateOrderJobs)
		router.HandleFunc("GET /api/v1/admin/production/jobs/{id}", production.GetJob)
		router.HandleFunc("POST /api/v1/admin/production/jobs/{id}/queue", production.QueueJob)
		router.HandleFunc("POST /api/v1/admin/production/jobs/{id}/prepare", production.Prepare)
		router.HandleFunc("POST /api/v1/admin/production/jobs/{id}/complete", production.CompleteJob)
		router.HandleFunc("POST /api/v1/admin/production/jobs/{id}/cancel", production.CancelJob)
		router.HandleFunc("POST /api/v1/admin/production/runs/{id}/start", production.StartRun)
		router.HandleFunc("POST /api/v1/admin/production/runs/{id}/finish", production.FinishRun)
		router.HandleFunc("POST /api/v1/admin/production/runs/{id}/fail", production.FailRun)
		router.HandleFunc("POST /api/v1/admin/production/runs/{id}/reprint", production.Reprint)
		router.HandleFunc("POST /api/v1/admin/production/runs/{id}/materials", production.Consume)
		router.HandleFunc("POST /api/v1/admin/production/runs/{id}/inspections", production.Inspect)
		router.HandleFunc("POST /api/v1/admin/production/runs/{id}/approve", production.Approve)
		router.HandleFunc("POST /api/v1/admin/production/runs/{id}/reject", production.Reject)
		router.HandleFunc("GET /api/v1/admin/production/printers", production.ListPrinters)
		router.HandleFunc("POST /api/v1/admin/production/printers", production.CreatePrinter)
		router.HandleFunc("PUT /api/v1/admin/production/printers/{id}", production.UpdatePrinter)
		router.HandleFunc("POST /api/v1/admin/production/printers/{id}/status", production.PrinterStatus)
		router.HandleFunc("POST /api/v1/admin/production/printers/{id}/maintenance", production.Maintenance)
		router.HandleFunc("GET /api/v1/admin/production/spools", production.ListSpools)
		router.HandleFunc("POST /api/v1/admin/production/spools", production.CreateSpool)
		router.HandleFunc("GET /api/v1/admin/production/spools/{id}", production.GetSpool)
		router.HandleFunc("PUT /api/v1/admin/production/spools/{id}", production.UpdateSpool)
		router.HandleFunc("POST /api/v1/admin/production/spools/{id}/adjust", production.AdjustSpool)
		router.HandleFunc("POST /api/v1/admin/production/spools/{id}/quarantine", production.QuarantineSpool)
	}
	return middleware.CORS(frontendOrigin)(router)
}
