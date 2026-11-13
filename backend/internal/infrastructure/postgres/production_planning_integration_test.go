package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionPlanningQueriesIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") || strings.EqualFold(config.ConnConfig.Database, "loja_3d") {
		t.Fatal("planning integration requires a dedicated *_test database")
	}
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == dsn {
		t.Fatal("TEST_DATABASE_URL must differ from DATABASE_URL")
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repository := NewProductionRepository(pool)
	if _, err = repository.ProductionDashboard(ctx, time.Now().Add(-30*24*time.Hour)); err != nil {
		t.Fatalf("dashboard query: %v", err)
	}
	if _, _, err = repository.PlanningInputs(ctx, app.PlanningFilter{Page: 1, Limit: 20}); err != nil {
		t.Fatalf("planning query: %v", err)
	}
	if _, err = repository.PrinterCapacities(ctx, time.Now()); err != nil {
		t.Fatalf("capacity query: %v", err)
	}
	if _, err = repository.MaterialPlanning(ctx); err != nil {
		t.Fatalf("material query: %v", err)
	}
}
