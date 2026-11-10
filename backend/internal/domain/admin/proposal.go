package admin

import (
	"errors"
	"time"
)

var ErrInvalidTransition = errors.New("invalid quote transition")
var ErrInvalidPricing = errors.New("invalid pricing")

type Pricing struct {
	MaterialCostCents, EnergyCostCents, MachineCostCents, FinishingCostCents, PackagingCostCents, OtherCostsCents int64
	MarginBasisPoints                                                                                             int
	FinalPriceCents                                                                                               int64
	ProductionLeadTimeDays                                                                                        int
	EstimatedCompletionDate                                                                                       *time.Time
	ValidUntil                                                                                                    time.Time
	CommercialNotes                                                                                               string
	ShippingIncluded                                                                                              bool
}

type Proposal struct {
	ID, Status, NotificationStatus, CommercialNotes string
	Version                                         int
	Pricing
	CreatedAt, UpdatedAt time.Time
	SentAt               *time.Time
}

func (p Pricing) Costs() int64 {
	return p.MaterialCostCents + p.EnergyCostCents + p.MachineCostCents + p.FinishingCostCents + p.PackagingCostCents + p.OtherCostsCents
}

func (p Pricing) Validate(now time.Time) error {
	values := []int64{p.MaterialCostCents, p.EnergyCostCents, p.MachineCostCents, p.FinishingCostCents, p.PackagingCostCents, p.OtherCostsCents, p.FinalPriceCents}
	for _, value := range values {
		if value < 0 {
			return ErrInvalidPricing
		}
	}
	if p.ProductionLeadTimeDays < 1 || !p.ValidUntil.After(now) || p.MarginBasisPoints < 0 || p.MarginBasisPoints > 100000 {
		return ErrInvalidPricing
	}
	minimum := p.Costs() + (p.Costs()*int64(p.MarginBasisPoints))/10000
	if p.FinalPriceCents < minimum {
		return ErrInvalidPricing
	}
	return nil
}
