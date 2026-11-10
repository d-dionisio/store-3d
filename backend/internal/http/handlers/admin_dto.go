package handlers

import (
	domain "example.com/loja-3d/backend/internal/domain/admin"
	"time"
)

type adminQuoteDTO struct {
	Reference      string    `json:"reference"`
	CustomerName   string    `json:"customer_name"`
	MaterialName   string    `json:"material_name"`
	Status         string    `json:"status"`
	SecurityStatus string    `json:"security_status"`
	Quantity       int       `json:"quantity"`
	CreatedAt      time.Time `json:"created_at"`
}
type adminFileDTO struct {
	ID             string    `json:"id"`
	Filename       string    `json:"filename"`
	Extension      string    `json:"extension"`
	SecurityStatus string    `json:"security_status"`
	SizeBytes      int64     `json:"size_bytes"`
	CreatedAt      time.Time `json:"created_at"`
}
type adminHistoryDTO struct {
	Kind   string    `json:"kind"`
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}
type adminDetailDTO struct {
	Reference     string             `json:"reference"`
	CustomerName  string             `json:"customer_name"`
	CustomerEmail string             `json:"customer_email"`
	CustomerPhone string             `json:"customer_phone"`
	MaterialName  string             `json:"material_name"`
	ColorName     string             `json:"color_name"`
	Status        string             `json:"status"`
	Notes         string             `json:"notes"`
	Quantity      int                `json:"quantity"`
	WidthMM       *int               `json:"width_mm"`
	HeightMM      *int               `json:"height_mm"`
	DepthMM       *int               `json:"depth_mm"`
	CreatedAt     time.Time          `json:"created_at"`
	Files         []adminFileDTO     `json:"files"`
	History       []adminHistoryDTO  `json:"history"`
	Proposals     []adminProposalDTO `json:"proposals"`
}
type adminProposalDTO struct {
	ID                      string     `json:"id"`
	Version                 int        `json:"version"`
	Status                  string     `json:"status"`
	NotificationStatus      string     `json:"notification_status"`
	MaterialCostCents       int64      `json:"material_cost_cents"`
	EnergyCostCents         int64      `json:"energy_cost_cents"`
	MachineCostCents        int64      `json:"machine_cost_cents"`
	FinishingCostCents      int64      `json:"finishing_cost_cents"`
	PackagingCostCents      int64      `json:"packaging_cost_cents"`
	OtherCostsCents         int64      `json:"other_costs_cents"`
	MarginBasisPoints       int        `json:"margin_basis_points"`
	FinalPriceCents         int64      `json:"final_price_cents"`
	ProductionLeadTimeDays  int        `json:"production_lead_time_days"`
	EstimatedCompletionDate *time.Time `json:"estimated_completion_date"`
	ValidUntil              time.Time  `json:"valid_until"`
	CommercialNotes         string     `json:"commercial_notes"`
	ShippingIncluded        bool       `json:"shipping_included"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	SentAt                  *time.Time `json:"sent_at"`
}

func mapAdminSummary(v domain.QuoteSummary) adminQuoteDTO {
	return adminQuoteDTO{v.Reference, v.CustomerName, v.MaterialName, v.Status, v.SecurityStatus, v.Quantity, v.CreatedAt}
}
func mapAdminDetail(v domain.QuoteDetail) adminDetailDTO {
	d := adminDetailDTO{Reference: v.Reference, CustomerName: v.CustomerName, CustomerEmail: v.CustomerEmail, CustomerPhone: v.CustomerPhone, MaterialName: v.MaterialName, ColorName: v.ColorName, Status: v.Status, Notes: v.Notes, Quantity: v.Quantity, WidthMM: v.WidthMM, HeightMM: v.HeightMM, DepthMM: v.DepthMM, CreatedAt: v.CreatedAt, Files: []adminFileDTO{}, History: []adminHistoryDTO{}, Proposals: []adminProposalDTO{}}
	for _, f := range v.Files {
		d.Files = append(d.Files, adminFileDTO{f.ID, f.Filename, f.Extension, f.SecurityStatus, f.SizeBytes, f.CreatedAt})
	}
	for _, h := range v.History {
		d.History = append(d.History, adminHistoryDTO{h.Kind, h.Status, h.At})
	}
	for _, p := range v.Proposals {
		d.Proposals = append(d.Proposals, adminProposalDTO{ID: p.ID, Version: p.Version, Status: p.Status, NotificationStatus: p.NotificationStatus, MaterialCostCents: p.MaterialCostCents, EnergyCostCents: p.EnergyCostCents, MachineCostCents: p.MachineCostCents, FinishingCostCents: p.FinishingCostCents, PackagingCostCents: p.PackagingCostCents, OtherCostsCents: p.OtherCostsCents, MarginBasisPoints: p.MarginBasisPoints, FinalPriceCents: p.FinalPriceCents, ProductionLeadTimeDays: p.ProductionLeadTimeDays, EstimatedCompletionDate: p.EstimatedCompletionDate, ValidUntil: p.ValidUntil, CommercialNotes: p.CommercialNotes, ShippingIncluded: p.ShippingIncluded, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, SentAt: p.SentAt})
	}
	return d
}
