package production

import (
	"context"
	"errors"
	"testing"
	"time"

	admindomain "example.com/loja-3d/backend/internal/domain/admin"
)

type planningRepoStub struct {
	inputs []PlanningInput
	err    error
}

func (p planningRepoStub) ProductionDashboard(ctx context.Context, _ time.Time) (Dashboard, error) {
	if err := ctx.Err(); err != nil {
		return Dashboard{}, err
	}
	return Dashboard{AwaitingJobs: 101}, p.err
}
func (p planningRepoStub) PlanningInputs(ctx context.Context, _ PlanningFilter) ([]PlanningInput, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return p.inputs, int64(len(p.inputs)), p.err
}
func (p planningRepoStub) PrinterCapacities(context.Context, time.Time) ([]PrinterCapacityView, error) {
	return []PrinterCapacityView{{ID: "p1"}}, p.err
}
func (p planningRepoStub) MaterialPlanning(context.Context) ([]MaterialPlanningView, error) {
	return []MaterialPlanningView{{MaterialID: "m1"}}, p.err
}

func TestPlanningServiceAuthorizationAndDashboard(t *testing.T) {
	s := New(nil)
	s.SetPlanningRepository(planningRepoStub{})
	if _, err := s.GetProductionDashboard(context.Background(), admindomain.Session{Role: "quote_viewer"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer err=%v", err)
	}
	for _, role := range []string{"admin", "production_manager", "production_operator"} {
		got, err := s.GetProductionDashboard(context.Background(), admindomain.Session{Role: role})
		if err != nil || got.AwaitingJobs != 101 {
			t.Fatalf("role=%s dashboard=%+v err=%v", role, got, err)
		}
	}
}

func TestPlanningServiceRiskAndForecast(t *testing.T) {
	due := time.Now().UTC().Add(5 * time.Hour)
	estimated := 300
	s := New(nil)
	s.SetPlanningRepository(planningRepoStub{inputs: []PlanningInput{{ID: "j1", DueAt: &due, ExplicitEstimatedMinutes: &estimated}}})
	page, err := s.ListProductionPlanning(context.Background(), admindomain.Session{Role: "production_manager"}, PlanningFilter{Page: 1, Limit: 20})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].Forecast.RemainingMinutes == nil || page.Items[0].Risk != "attention" {
		t.Fatalf("item=%+v", page.Items[0])
	}
}

func TestPlanningServiceRepositoryFailureAndCancelledContext(t *testing.T) {
	want := errors.New("database unavailable")
	s := New(nil)
	s.SetPlanningRepository(planningRepoStub{err: want})
	if _, err := s.GetProductionDashboard(context.Background(), admindomain.Session{Role: "admin"}); !errors.Is(err, want) {
		t.Fatalf("repository err=%v", err)
	}
	s.SetPlanningRepository(planningRepoStub{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.GetProductionDashboard(ctx, admindomain.Session{Role: "admin"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
