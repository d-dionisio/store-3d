package admin

import (
	"errors"
	"testing"
	"time"
)

func TestPricingValidate(t *testing.T) {
	now := time.Now()
	valid := Pricing{MaterialCostCents: 1000, EnergyCostCents: 500, MarginBasisPoints: 2000, FinalPriceCents: 1800, ProductionLeadTimeDays: 3, ValidUntil: now.Add(time.Hour)}
	if valid.Costs() != 1500 {
		t.Fatalf("costs = %d", valid.Costs())
	}
	if err := valid.Validate(now); err != nil {
		t.Fatalf("valid pricing: %v", err)
	}
	tests := []Pricing{
		{MaterialCostCents: -1, FinalPriceCents: 1, ProductionLeadTimeDays: 1, ValidUntil: now.Add(time.Hour)},
		{MaterialCostCents: 1000, MarginBasisPoints: 1000, FinalPriceCents: 1099, ProductionLeadTimeDays: 1, ValidUntil: now.Add(time.Hour)},
		{FinalPriceCents: 1, ProductionLeadTimeDays: 0, ValidUntil: now.Add(time.Hour)},
		{FinalPriceCents: 1, ProductionLeadTimeDays: 1, ValidUntil: now.Add(-time.Hour)},
	}
	for i, value := range tests {
		if !errors.Is(value.Validate(now), ErrInvalidPricing) {
			t.Errorf("case %d should be invalid", i)
		}
	}
}
