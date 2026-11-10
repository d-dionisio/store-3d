package customer

import "time"

type Session struct {
	ID, UserID, Email, FullName, CSRFHash string
	ExpiresAt                             time.Time
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
