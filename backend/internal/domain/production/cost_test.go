package production

import (
	"errors"
	"testing"
)

func TestCalculateManufacturingCost(t *testing.T) {
	got, err := CalculateManufacturingCost(CostInput{
		Materials:     []MaterialCostInput{{ConsumedGrams: 100, WastedGrams: 10, CostPerKGCents: 10000}},
		EnergyMilliWh: 500000, EnergyRateCentsKWh: 100,
		MachineAcquisitionCents: 360000, MachineUsefulLifeSeconds: 3600,
		MaintenanceRateCentsHour: 3600, LaborRateCentsHour: 7200,
		RunSeconds: 3600, PreparationSeconds: 1800, FinishingSeconds: 900,
		RevenueCents: 5000,
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Material: 110g at 10000 cents/kg = 1100 cents. Energy 50,
	// depreciation 360000, maintenance 3600, prep 3600 and finishing 1800.
	if got.MaterialTotalCents != 1100 || got.MaterialConsumedCents != 1000 || got.MaterialWasteCents != 100 || got.EnergyCents != 50 || got.DepreciationCents != 360000 || got.MaintenanceCents != 3600 || got.PreparationCents != 3600 || got.FinishingCents != 1800 {
		t.Fatalf("unexpected breakdown: %+v", got)
	}
	if got.TotalCostCents != 370150 || got.UnitCostCents != 185075 || got.ContributionMarginCents != -5150 || got.ProfitCents != -365150 || got.ContributionMarginBPS == nil || *got.ContributionMarginBPS != -10300 || got.ProfitMarginBPS == nil || *got.ProfitMarginBPS != -730300 {
		t.Fatalf("unexpected totals: %+v", got)
	}
}

func TestEnergyMilliWhFromExplicitPower(t *testing.T) {
	energy, err := EnergyMilliWhFromPower(150000, 3600)
	if err != nil || energy != 150000 {
		t.Fatalf("energy=%d err=%v", energy, err)
	}
}

func TestCalculateManufacturingCostRoundsHalfUp(t *testing.T) {
	got, err := CalculateManufacturingCost(CostInput{
		Materials:                []MaterialCostInput{{ConsumedGrams: 1, CostPerKGCents: 500}},
		MachineUsefulLifeSeconds: 1,
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaterialTotalCents != 1 || got.UnitCostCents != 1 {
		t.Fatalf("half-up rounding mismatch: %+v", got)
	}
}

func TestCalculateManufacturingCostRejectsInvalidAndOverflow(t *testing.T) {
	if _, err := CalculateManufacturingCost(CostInput{MachineUsefulLifeSeconds: 0}, 1); !errors.Is(err, ErrInvalidCostInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if _, err := CalculateManufacturingCost(CostInput{
		Materials: []MaterialCostInput{{ConsumedGrams: 1001, CostPerKGCents: int64(^uint64(0) >> 1)}}, MachineUsefulLifeSeconds: 1,
	}, 1); !errors.Is(err, ErrCostOverflow) {
		t.Fatalf("expected overflow, got %v", err)
	}
}
