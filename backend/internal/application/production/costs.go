package production

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
)

var ErrCostDataUnavailable = errors.New("insufficient operational data to calculate manufacturing cost")
var ErrCostAuditUnavailable = errors.New("required manufacturing-cost audit could not be persisted")

type CostAuditContext struct {
	SessionID string
	IPAddress string
}

type CostProfileInput struct {
	ElectricityRateCentsPerKWh  int64  `json:"electricity_rate_cents_per_kwh"`
	PrinterPowerMilliwatts      *int64 `json:"printer_power_milliwatts"`
	PowerMeasurementNote        string `json:"power_measurement_note"`
	MachineAcquisitionCents     int64  `json:"machine_acquisition_cents"`
	MachineUsefulLifeSeconds    int64  `json:"machine_useful_life_seconds"`
	MaintenanceRateCentsPerHour int64  `json:"maintenance_rate_cents_per_hour"`
	LaborRateCentsPerHour       int64  `json:"labor_rate_cents_per_hour"`
	PreparationSeconds          int64  `json:"preparation_seconds"`
	FinishingSeconds            int64  `json:"finishing_seconds"`
	Notes                       string `json:"notes"`
	ExpectedVersion             int64  `json:"expected_version"`
}

type CostProfileView struct {
	ID                          string    `json:"id"`
	PrinterID                   string    `json:"printer_id"`
	Notes                       string    `json:"notes"`
	PowerMeasurementNote        string    `json:"power_measurement_note"`
	Version                     int64     `json:"version"`
	CreatedAt                   time.Time `json:"created_at"`
	ElectricityRateCentsPerKWh  int64     `json:"electricity_rate_cents_per_kwh"`
	PrinterPowerMilliwatts      *int64    `json:"printer_power_milliwatts"`
	MachineAcquisitionCents     int64     `json:"machine_acquisition_cents"`
	MachineUsefulLifeSeconds    int64     `json:"machine_useful_life_seconds"`
	MaintenanceRateCentsPerHour int64     `json:"maintenance_rate_cents_per_hour"`
	LaborRateCentsPerHour       int64     `json:"labor_rate_cents_per_hour"`
	PreparationSeconds          int64     `json:"preparation_seconds"`
	FinishingSeconds            int64     `json:"finishing_seconds"`
}

type CostSnapshotView struct {
	ID                          string          `json:"id"`
	RunID                       string          `json:"run_id"`
	SnapshotType                string          `json:"snapshot_type"`
	PrinterID                   string          `json:"printer_id"`
	CostProfileID               string          `json:"cost_profile_id"`
	CostProfileVersion          int64           `json:"cost_profile_version"`
	EnergySource                string          `json:"energy_source"`
	EnergyMilliWh               int64           `json:"energy_milliwh"`
	RunSeconds                  int64           `json:"run_seconds"`
	PreparationSeconds          int64           `json:"preparation_seconds"`
	FinishingSeconds            int64           `json:"finishing_seconds"`
	ElectricityRateCentsPerKWh  int64           `json:"electricity_rate_cents_per_kwh"`
	MachineAcquisitionCents     int64           `json:"machine_acquisition_cents"`
	MachineUsefulLifeSeconds    int64           `json:"machine_useful_life_seconds"`
	MaintenanceRateCentsPerHour int64           `json:"maintenance_rate_cents_per_hour"`
	LaborRateCentsPerHour       int64           `json:"labor_rate_cents_per_hour"`
	ConsumedGrams               int64           `json:"consumed_grams"`
	WastedGrams                 int64           `json:"wasted_grams"`
	MaterialBreakdown           json.RawMessage `json:"material_breakdown"`
	domain.CostBreakdown
}

type CostContext struct {
	Profile                                      domain.CostProfile
	Input                                        domain.CostInput
	RunID, PrinterID, SnapshotType, EnergySource string
	ProfileID                                    string
	RunVersion                                   int64
	Quantity                                     int64
	ConsumedGrams, WastedGrams                   int64
	MaterialBreakdown                            []byte
}

type RunCostRequest struct {
	Kind                  string
	MeasuredEnergyMilliWh *int64
	PreparationSeconds    *int64
	FinishingSeconds      *int64
}

type CostRepository interface {
	SaveCostProfile(context.Context, string, CostProfileInput, string, CostAuditContext) (CostProfileView, error)
	GetCurrentCostProfile(context.Context, string) (CostProfileView, error)
	ListCostProfileHistory(context.Context, string) ([]CostProfileView, error)
	LoadRunCostContext(context.Context, string, string, *int64) (CostContext, error)
	SaveRunCostSnapshot(context.Context, CostContext, domain.CostBreakdown, string, CostAuditContext) (CostSnapshotView, error)
	ListRunCostSnapshots(context.Context, string) ([]CostSnapshotView, error)
	CostProfitability(context.Context, string, string) ([]ProductCostProfitability, error)
	CostProfitabilityByPrinter(context.Context, string, string) ([]PrinterCostProfitability, error)
	CostEstimateComparison(context.Context, string, string) ([]CostEstimateComparison, error)
}

type ProductCostProfitability struct {
	ProductID               string `json:"product_id"`
	ProductName             string `json:"product_name"`
	Runs                    int64  `json:"runs"`
	Units                   int64  `json:"units"`
	RevenueCents            int64  `json:"revenue_cents"`
	ManufacturingCostCents  int64  `json:"manufacturing_cost_cents"`
	ContributionMarginCents int64  `json:"contribution_margin_cents"`
	ProfitCents             int64  `json:"profit_cents"`
	ContributionMarginBPS   *int64 `json:"contribution_margin_basis_points"`
	ProfitMarginBPS         *int64 `json:"profit_margin_basis_points"`
}

type PrinterCostProfitability struct {
	PrinterID               string `json:"printer_id"`
	PrinterName             string `json:"printer_name"`
	Runs                    int64  `json:"runs"`
	RevenueCents            int64  `json:"revenue_cents"`
	ManufacturingCostCents  int64  `json:"manufacturing_cost_cents"`
	ContributionMarginCents int64  `json:"contribution_margin_cents"`
	ProfitCents             int64  `json:"profit_cents"`
	ContributionMarginBPS   *int64 `json:"contribution_margin_basis_points"`
	ProfitMarginBPS         *int64 `json:"profit_margin_basis_points"`
}

type CostEstimateComparison struct {
	RunID                 string `json:"run_id"`
	EstimatedCostCents    int64  `json:"estimated_cost_cents"`
	ActualCostCents       int64  `json:"actual_cost_cents"`
	DifferenceCents       int64  `json:"difference_cents"`
	DifferenceBasisPoints *int64 `json:"difference_basis_points"`
}

func (s *Service) SetCostRepository(r CostRepository) { s.costs = r }

func (s *Service) AuthorizeProductionCostManagement(a admindomain.Session) error {
	return require(canManage(a))
}

func (s *Service) SaveProductionCostProfile(ctx context.Context, a admindomain.Session, printerID string, in CostProfileInput, audit CostAuditContext) (CostProfileView, error) {
	if err := require(canManage(a)); err != nil {
		return CostProfileView{}, err
	}
	if printerID == "" || in.ExpectedVersion < 0 {
		return CostProfileView{}, ErrInvalid
	}
	p := domain.CostProfile{PrinterID: printerID, Version: in.ExpectedVersion, ElectricityRateCentsPerKWh: in.ElectricityRateCentsPerKWh, PrinterPowerMilliwatts: in.PrinterPowerMilliwatts, PowerMeasurementNote: in.PowerMeasurementNote, MachineAcquisitionCents: in.MachineAcquisitionCents, MachineUsefulLifeSeconds: in.MachineUsefulLifeSeconds, MaintenanceRateCentsPerHour: in.MaintenanceRateCentsPerHour, LaborRateCentsPerHour: in.LaborRateCentsPerHour, PreparationSeconds: in.PreparationSeconds, FinishingSeconds: in.FinishingSeconds}
	if err := p.Validate(); err != nil {
		return CostProfileView{}, ErrInvalid
	}
	return s.costs.SaveCostProfile(ctx, printerID, in, a.UserID, audit)
}

func (s *Service) GetProductionCostProfile(ctx context.Context, a admindomain.Session, printerID string) (CostProfileView, error) {
	if err := require(canManage(a)); err != nil {
		return CostProfileView{}, err
	}
	return s.costs.GetCurrentCostProfile(ctx, printerID)
}

func (s *Service) ProductionCostProfileHistory(ctx context.Context, a admindomain.Session, printerID string) ([]CostProfileView, error) {
	if err := require(canManage(a)); err != nil {
		return nil, err
	}
	return s.costs.ListCostProfileHistory(ctx, printerID)
}

func (s *Service) CalculateRunCost(ctx context.Context, a admindomain.Session, runID string, request RunCostRequest, audit CostAuditContext) (CostSnapshotView, error) {
	if err := require(canManage(a)); err != nil {
		return CostSnapshotView{}, err
	}
	if request.Kind != "estimate" && request.Kind != "actual" {
		return CostSnapshotView{}, ErrInvalid
	}
	if request.MeasuredEnergyMilliWh != nil && (*request.MeasuredEnergyMilliWh < 0 || request.Kind != "actual") {
		return CostSnapshotView{}, ErrInvalid
	}
	if request.Kind == "actual" && (request.PreparationSeconds == nil || request.FinishingSeconds == nil || *request.PreparationSeconds < 0 || *request.FinishingSeconds < 0) {
		return CostSnapshotView{}, ErrInvalid
	}
	c, err := s.costs.LoadRunCostContext(ctx, runID, request.Kind, request.MeasuredEnergyMilliWh)
	if err != nil {
		return CostSnapshotView{}, err
	}
	if c.Profile.PrinterPowerMilliwatts == nil && request.MeasuredEnergyMilliWh == nil {
		return CostSnapshotView{}, ErrCostDataUnavailable
	}
	if request.Kind == "actual" {
		c.Input.PreparationSeconds = *request.PreparationSeconds
		c.Input.FinishingSeconds = *request.FinishingSeconds
	}
	result, err := domain.CalculateManufacturingCost(c.Input, c.Quantity)
	if err != nil {
		return CostSnapshotView{}, ErrInvalid
	}
	return s.costs.SaveRunCostSnapshot(ctx, c, result, a.UserID, audit)
}

func (s *Service) GetRunCostSnapshots(ctx context.Context, a admindomain.Session, runID string) ([]CostSnapshotView, error) {
	if err := require(canManage(a)); err != nil {
		return nil, err
	}
	return s.costs.ListRunCostSnapshots(ctx, runID)
}

func (s *Service) GetCostProfitability(ctx context.Context, a admindomain.Session, from, to string) ([]ProductCostProfitability, error) {
	if err := require(canManage(a)); err != nil {
		return nil, err
	}
	return s.costs.CostProfitability(ctx, from, to)
}

func (s *Service) GetPrinterCostProfitability(ctx context.Context, a admindomain.Session, from, to string) ([]PrinterCostProfitability, error) {
	if err := require(canManage(a)); err != nil {
		return nil, err
	}
	return s.costs.CostProfitabilityByPrinter(ctx, from, to)
}

func (s *Service) GetCostEstimateComparison(ctx context.Context, a admindomain.Session, from, to string) ([]CostEstimateComparison, error) {
	if err := require(canManage(a)); err != nil {
		return nil, err
	}
	return s.costs.CostEstimateComparison(ctx, from, to)
}
