package customer

import "time"

type Session struct {
	ID, UserID, Email, FullName, CSRFHash, AuthenticationLevel string
	ExpiresAt                                                  time.Time
}
type Profile struct{ ID, Email, FullName, Phone string }
type QuoteSummary struct {
	Reference, Status, Material string
	Quantity                    int
	CreatedAt                   time.Time
}
type Proposal struct {
	ID, Status             string
	Version                int
	FinalPriceCents        int64
	ProductionLeadTimeDays int
	ValidUntil             time.Time
	CommercialNotes        string
	ShippingIncluded       bool
	Decision               string
	DecidedAt              *time.Time
}
type QuoteDetail struct {
	QuoteSummary
	Color, Notes string
	Proposals    []Proposal
}
type OrderResult struct{ Reference, Status string }
type CartInput struct {
	VariantID string
	Quantity  int
}
type CartLine struct {
	VariantID, ProductName, SKU, Material, Color string
	Quantity, AvailableQuantity                  int
	UnitPriceCents, LineTotalCents               int64
}
type Cart struct {
	Items         []CartLine
	SubtotalCents int64
}
type Address struct {
	ID, RecipientName, PostalCode, Street, Number, Complement, District, City, StateCode, CountryCode string
	IsDefault                                                                                         bool
}
type OrderSummary struct {
	Reference, Status, PaymentReadiness string
	TotalCents                          int64
	CreatedAt                           time.Time
}
type OrderDetail struct {
	OrderSummary
	Items            []CartLine
	Address          *Address
	ShippingRequired bool
}
