package production

import (
	"context"
	"errors"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"testing"
	"time"
)

type optimizationStub struct {
	snapshot OptimizationSnapshot
	err      error
}

func (o optimizationStub) OptimizationSnapshot(context.Context, time.Time) (OptimizationSnapshot, error) {
	return o.snapshot, o.err
}
func TestOptimizationAuthorizationValidationAndConsultativeResult(t *testing.T) {
	service := New(nil)
	service.SetOptimizationRepository(optimizationStub{})
	if _, err := service.GetOptimization(context.Background(), admindomain.Session{Role: "quote_viewer"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("forbidden=%v", err)
	}
	if _, err := service.SimulateOptimization(context.Background(), admindomain.Session{Role: "admin"}, "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid=%v", err)
	}
	value, err := service.GetOptimization(context.Background(), admindomain.Session{Role: "production_operator"})
	if err != nil || !value.ConsultativeOnly || len(value.Strategies) != 5 {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}
func TestOptimizationPropagatesRepositoryFailure(t *testing.T) {
	want := errors.New("db")
	service := New(nil)
	service.SetOptimizationRepository(optimizationStub{err: want})
	_, err := service.GetOptimization(context.Background(), admindomain.Session{Role: "production_manager"})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
func TestOptimizationRecommendsBestPrinter(t *testing.T) {
	d := 10
	service := New(nil)
	service.SetOptimizationRepository(optimizationStub{snapshot: OptimizationSnapshot{Jobs: []domain.OptimizationJob{{ID: "j", Number: "J", WidthMM: &d, DepthMM: &d, HeightMM: &d}}, Printers: []domain.OptimizationPrinter{{ID: "p", Name: "P", Status: "available", BuildWidthMM: 20, BuildDepthMM: 20, BuildHeightMM: 20}}}})
	value, err := service.GetOptimization(context.Background(), admindomain.Session{Role: "admin"})
	if err != nil || value.Recommendations[0].Recommended == nil || value.Recommendations[0].Recommended.PrinterID != "p" {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}
