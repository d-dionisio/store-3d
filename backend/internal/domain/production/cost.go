package production

import (
	"errors"
	"math/big"
	"strings"
)

var (
	ErrInvalidCostInput = errors.New("invalid manufacturing cost input")
	ErrCostOverflow     = errors.New("manufacturing cost overflow")
)

const milliWhPerKWh int64 = 1_000_000

// MaterialCostInput uses the immutable unit-price snapshot captured when a
// production_run_materials row is recorded. Grams are integer physical units.
type MaterialCostInput struct {
	ConsumedGrams, WastedGrams int64
	CostPerKGCents             int64
}

// CostInput contains only explicit measurements, snapshots, or configured
// values. All money is cents; energy is milliWh; durations are seconds.
type CostInput struct {
	Materials                         []MaterialCostInput
	EnergyMilliWh, EnergyRateCentsKWh int64
	MachineAcquisitionCents           int64
	MachineUsefulLifeSeconds          int64
	MaintenanceRateCentsHour          int64
	LaborRateCentsHour                int64
	RunSeconds, PreparationSeconds    int64
	FinishingSeconds                  int64
	PreparationOtherCostCents         int64
	FinishingOtherCostCents           int64
	RevenueCents                      int64
}

type CostProfile struct {
	PrinterID                   string
	Version                     int64
	ElectricityRateCentsPerKWh  int64
	PrinterPowerMilliwatts      *int64
	PowerMeasurementNote        string
	MachineAcquisitionCents     int64
	MachineUsefulLifeSeconds    int64
	MaintenanceRateCentsPerHour int64
	LaborRateCentsPerHour       int64
	PreparationSeconds          int64
	FinishingSeconds            int64
}

func (p CostProfile) Validate() error {
	if p.PrinterID == "" || p.Version < 0 || p.ElectricityRateCentsPerKWh < 0 ||
		p.MachineAcquisitionCents < 0 || p.MachineUsefulLifeSeconds <= 0 ||
		p.MaintenanceRateCentsPerHour < 0 || p.LaborRateCentsPerHour < 0 ||
		p.PreparationSeconds < 0 || p.FinishingSeconds < 0 {
		return ErrInvalidCostInput
	}
	if p.PrinterPowerMilliwatts != nil && *p.PrinterPowerMilliwatts <= 0 {
		return ErrInvalidCostInput
	}
	if p.PrinterPowerMilliwatts == nil && strings.TrimSpace(p.PowerMeasurementNote) == "" {
		return ErrInvalidCostInput
	}
	return nil
}

type CostBreakdown struct {
	MaterialConsumedCents   int64  `json:"material_consumed_cents"`
	MaterialWasteCents      int64  `json:"material_waste_cents"`
	MaterialTotalCents      int64  `json:"material_total_cents"`
	EnergyCents             int64  `json:"energy_cents"`
	DepreciationCents       int64  `json:"depreciation_cents"`
	MaintenanceCents        int64  `json:"maintenance_cents"`
	PreparationCents        int64  `json:"preparation_cents"`
	FinishingCents          int64  `json:"finishing_cents"`
	LaborTotalCents         int64  `json:"labor_total_cents"`
	TotalCostCents          int64  `json:"total_cost_cents"`
	UnitCostCents           int64  `json:"unit_cost_cents"`
	RevenueCents            int64  `json:"revenue_cents"`
	ContributionMarginCents int64  `json:"contribution_margin_cents"`
	ContributionMarginBPS   *int64 `json:"contribution_margin_basis_points"`
	ProfitCents             int64  `json:"profit_cents"`
	ProfitMarginBPS         *int64 `json:"profit_margin_basis_points"`
}

// CalculateManufacturingCost is deterministic and uses integer rational
// arithmetic with half-up cent rounding. Contribution margin is revenue less
// variable material, energy, maintenance and labor costs; depreciation is
// included in profit/full cost but excluded from contribution margin.
func CalculateManufacturingCost(in CostInput, quantity int64) (CostBreakdown, error) {
	if quantity <= 0 || in.EnergyMilliWh < 0 || in.EnergyRateCentsKWh < 0 ||
		in.MachineAcquisitionCents < 0 || in.MachineUsefulLifeSeconds <= 0 ||
		in.MaintenanceRateCentsHour < 0 || in.LaborRateCentsHour < 0 ||
		in.RunSeconds < 0 || in.PreparationSeconds < 0 || in.FinishingSeconds < 0 ||
		in.PreparationOtherCostCents < 0 || in.FinishingOtherCostCents < 0 || in.RevenueCents < 0 {
		return CostBreakdown{}, ErrInvalidCostInput
	}
	var out CostBreakdown
	for _, m := range in.Materials {
		if m.ConsumedGrams < 0 || m.WastedGrams < 0 || m.ConsumedGrams > int64(^uint64(0)>>1)-m.WastedGrams || m.ConsumedGrams+m.WastedGrams <= 0 || m.CostPerKGCents < 0 {
			return CostBreakdown{}, ErrInvalidCostInput
		}
		all, err := roundProduct(m.ConsumedGrams+m.WastedGrams, m.CostPerKGCents, 1000)
		if err != nil {
			return CostBreakdown{}, err
		}
		consumed, err := roundProduct(m.ConsumedGrams, m.CostPerKGCents, 1000)
		if err != nil {
			return CostBreakdown{}, err
		}
		out.MaterialConsumedCents, err = addMoney(out.MaterialConsumedCents, consumed)
		if err != nil {
			return CostBreakdown{}, err
		}
		out.MaterialWasteCents, err = addMoney(out.MaterialWasteCents, all-consumed)
		if err != nil {
			return CostBreakdown{}, err
		}
	}
	var err error
	out.MaterialTotalCents, err = addMoney(out.MaterialConsumedCents, out.MaterialWasteCents)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.EnergyCents, err = roundProduct(in.EnergyMilliWh, in.EnergyRateCentsKWh, milliWhPerKWh)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.DepreciationCents, err = roundProduct(in.MachineAcquisitionCents, in.RunSeconds, in.MachineUsefulLifeSeconds)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.MaintenanceCents, err = roundProduct(in.MaintenanceRateCentsHour, in.RunSeconds, 3600)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.PreparationCents, err = roundProduct(in.LaborRateCentsHour, in.PreparationSeconds, 3600)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.FinishingCents, err = roundProduct(in.LaborRateCentsHour, in.FinishingSeconds, 3600)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.PreparationCents, err = addMoney(out.PreparationCents, in.PreparationOtherCostCents)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.FinishingCents, err = addMoney(out.FinishingCents, in.FinishingOtherCostCents)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.LaborTotalCents, err = addMoney(out.PreparationCents, out.FinishingCents)
	if err != nil {
		return CostBreakdown{}, err
	}
	for _, amount := range []int64{out.MaterialTotalCents, out.EnergyCents, out.DepreciationCents, out.MaintenanceCents, out.LaborTotalCents} {
		out.TotalCostCents, err = addMoney(out.TotalCostCents, amount)
		if err != nil {
			return CostBreakdown{}, err
		}
	}
	out.UnitCostCents = out.TotalCostCents / quantity
	if remainder := out.TotalCostCents % quantity; remainder >= quantity/2+quantity%2 {
		out.UnitCostCents++
	}
	out.RevenueCents = in.RevenueCents
	variableCost, err := addMoney(out.MaterialTotalCents, out.EnergyCents)
	if err != nil {
		return CostBreakdown{}, err
	}
	variableCost, err = addMoney(variableCost, out.MaintenanceCents)
	if err != nil {
		return CostBreakdown{}, err
	}
	variableCost, err = addMoney(variableCost, out.LaborTotalCents)
	if err != nil {
		return CostBreakdown{}, err
	}
	out.ContributionMarginCents = in.RevenueCents - variableCost
	out.ProfitCents = in.RevenueCents - out.TotalCostCents
	if in.RevenueCents > 0 {
		contributionMargin, ratioErr := roundSignedRatio(out.ContributionMarginCents, 10000, in.RevenueCents)
		err = ratioErr
		if err != nil {
			return CostBreakdown{}, err
		}
		out.ContributionMarginBPS = &contributionMargin
		profitMargin, ratioErr := roundSignedRatio(out.ProfitCents, 10000, in.RevenueCents)
		err = ratioErr
		if err != nil {
			return CostBreakdown{}, err
		}
		out.ProfitMarginBPS = &profitMargin
	}
	return out, nil
}

// EnergyMilliWhFromPower derives milli-watt-hours from an explicitly supplied
// power measurement and duration. It does not encode printer-specific values.
func EnergyMilliWhFromPower(powerMilliwatts, durationSeconds int64) (int64, error) {
	if powerMilliwatts <= 0 || durationSeconds < 0 {
		return 0, ErrInvalidCostInput
	}
	return roundProduct(powerMilliwatts, durationSeconds, 3600)
}

func roundProduct(a, b, denominator int64) (int64, error) {
	if a < 0 || b < 0 || denominator <= 0 {
		return 0, ErrInvalidCostInput
	}
	n := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	n.Add(n, big.NewInt(denominator/2))
	n.Quo(n, big.NewInt(denominator))
	if !n.IsInt64() {
		return 0, ErrCostOverflow
	}
	return n.Int64(), nil
}

func roundSignedRatio(value, multiplier, denominator int64) (int64, error) {
	n := new(big.Int).Mul(big.NewInt(value), big.NewInt(multiplier))
	abs := new(big.Int).Abs(new(big.Int).Set(n))
	abs.Add(abs, big.NewInt(denominator/2))
	abs.Quo(abs, big.NewInt(denominator))
	if n.Sign() < 0 {
		abs.Neg(abs)
	}
	if !abs.IsInt64() {
		return 0, ErrCostOverflow
	}
	return abs.Int64(), nil
}

func addMoney(a, b int64) (int64, error) {
	if b > 0 && a > int64(^uint64(0)>>1)-b {
		return 0, ErrCostOverflow
	}
	return a + b, nil
}
