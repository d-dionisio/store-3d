package postgres

import (
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
)

func TestProductionPlanningQueriesIntegration(t *testing.T) {
	pool, ctx := newProductionIntegrationPool(t, "planning_queries")
	defer pool.Close()
	repository := NewProductionRepository(pool)
	if _, err := repository.ProductionDashboard(ctx, time.Now().Add(-30*24*time.Hour)); err != nil {
		t.Fatalf("dashboard query: %v", err)
	}
	if _, _, err := repository.PlanningInputs(ctx, app.PlanningFilter{Page: 1, Limit: 20}); err != nil {
		t.Fatalf("planning query: %v", err)
	}
	if _, err := repository.PrinterCapacities(ctx, time.Now()); err != nil {
		t.Fatalf("capacity query: %v", err)
	}
	if _, err := repository.MaterialPlanning(ctx); err != nil {
		t.Fatalf("material query: %v", err)
	}
}
