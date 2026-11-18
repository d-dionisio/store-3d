package production

import (
	"context"
	"errors"
	"testing"

	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
)

type costRepoFake struct {
	context CostContext
	stored  domain.CostBreakdown
	profile CostProfileView
	saveErr error
}

func (f *costRepoFake) SaveCostProfile(_ context.Context, printer string, in CostProfileInput, _ string) (CostProfileView, error) {
	if f.saveErr != nil {
		return CostProfileView{}, f.saveErr
	}
	f.profile = CostProfileView{PrinterID: printer, Version: in.ExpectedVersion + 1}
	return f.profile, nil
}
func (f *costRepoFake) GetCurrentCostProfile(context.Context, string) (CostProfileView, error) {
	return f.profile, nil
}
func (f *costRepoFake) ListCostProfileHistory(context.Context, string) ([]CostProfileView, error) {
	return []CostProfileView{f.profile}, nil
}
func (f *costRepoFake) LoadRunCostContext(_ context.Context, run, kind string, _ *int64) (CostContext, error) {
	c := f.context
	c.RunID = run
	c.SnapshotType = kind
	return c, nil
}
func (f *costRepoFake) SaveRunCostSnapshot(_ context.Context, c CostContext, b domain.CostBreakdown, _ string) (CostSnapshotView, error) {
	f.stored = b
	return CostSnapshotView{RunID: c.RunID, CostBreakdown: b}, nil
}
func (f *costRepoFake) ListRunCostSnapshots(context.Context, string) ([]CostSnapshotView, error) {
	return nil, nil
}
func (f *costRepoFake) CostProfitability(context.Context, string, string) ([]ProductCostProfitability, error) {
	return nil, nil
}
func (f *costRepoFake) CostProfitabilityByPrinter(context.Context, string, string) ([]PrinterCostProfitability, error) {
	return nil, nil
}
func (f *costRepoFake) CostEstimateComparison(context.Context, string, string) ([]CostEstimateComparison, error) {
	return nil, nil
}

func TestCostConfigurationIsManagerOnlyAndVersioned(t *testing.T) {
	repo := &costRepoFake{}
	service := New(nil)
	service.SetCostRepository(repo)
	input := CostProfileInput{ElectricityRateCentsPerKWh: 100, MachineUsefulLifeSeconds: 1000, ExpectedVersion: 0, PowerMeasurementNote: "medição local"}
	if _, err := service.SaveProductionCostProfile(context.Background(), admindomain.Session{Role: "production_operator"}, "printer", input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator error=%v", err)
	}
	if _, err := service.SaveProductionCostProfile(context.Background(), admindomain.Session{Role: "quote_viewer"}, "printer", input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("quote viewer error=%v", err)
	}
	got, err := service.SaveProductionCostProfile(context.Background(), admindomain.Session{Role: "production_manager", UserID: "admin"}, "printer", input)
	if err != nil || got.Version != 1 {
		t.Fatalf("profile=%+v err=%v", got, err)
	}
}

func TestActualCostRequiresObservedLaborTimes(t *testing.T) {
	power := int64(3600)
	repo := &costRepoFake{context: CostContext{Quantity: 1, Profile: domain.CostProfile{PrinterID: "printer", PrinterPowerMilliwatts: &power}, Input: domain.CostInput{MachineUsefulLifeSeconds: 3600, RunSeconds: 3600, LaborRateCentsHour: 3600, EnergyRateCentsKWh: 100}}}
	service := New(nil)
	service.SetCostRepository(repo)
	_, err := service.CalculateRunCost(context.Background(), admindomain.Session{Role: "admin", UserID: "admin"}, "run", RunCostRequest{Kind: "actual"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing measured labor times error=%v", err)
	}
	prep, finish := int64(60), int64(30)
	got, err := service.CalculateRunCost(context.Background(), admindomain.Session{Role: "admin", UserID: "admin"}, "run", RunCostRequest{Kind: "actual", PreparationSeconds: &prep, FinishingSeconds: &finish})
	if err != nil || got.TotalCostCents == 0 {
		t.Fatalf("snapshot=%+v err=%v", got, err)
	}
}
