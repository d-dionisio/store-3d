package production

import (
	"context"
	"errors"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	"strings"
	"testing"
	"time"
)

type analyticsRepoStub struct {
	page      HistoryPage
	detail    HistoryDetail
	analytics ProductivityAnalytics
	err       error
}

func (s analyticsRepoStub) ListHistory(context.Context, AnalyticsFilter) (HistoryPage, error) {
	return s.page, s.err
}
func (s analyticsRepoStub) GetHistory(context.Context, string) (HistoryDetail, error) {
	return s.detail, s.err
}
func (s analyticsRepoStub) Productivity(context.Context, AnalyticsFilter) (ProductivityAnalytics, error) {
	return s.analytics, s.err
}
func TestAnalyticsAuthorizationPaginationAndFilters(t *testing.T) {
	service := New(nil)
	service.SetAnalyticsRepository(analyticsRepoStub{page: HistoryPage{Total: 2}, analytics: ProductivityAnalytics{Summary: ProductivitySummary{TotalRuns: 2}}})
	viewer := admindomain.Session{Role: "quote_viewer"}
	if _, err := service.ListProductionHistory(context.Background(), viewer, AnalyticsFilter{Page: 1, Limit: 20}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer err=%v", err)
	}
	operator := admindomain.Session{Role: "production_operator"}
	if _, err := service.ListProductionHistory(context.Background(), operator, AnalyticsFilter{Page: 0, Limit: 20}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid page err=%v", err)
	}
	if _, err := service.ListProductionHistory(context.Background(), operator, AnalyticsFilter{PrinterID: "not-a-uuid", Page: 1, Limit: 20}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid identifier err=%v", err)
	}
	if _, err := service.ListProductionHistory(context.Background(), operator, AnalyticsFilter{ProductQuery: strings.Repeat("a", 101), Page: 1, Limit: 20}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized text filter err=%v", err)
	}
	from := time.Now()
	to := from.Add(-time.Hour)
	if _, err := service.GetProductivityAnalytics(context.Background(), operator, AnalyticsFilter{From: &from, To: &to, Page: 1, Limit: 20}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid dates err=%v", err)
	}
	value, err := service.GetProductivityAnalytics(context.Background(), operator, AnalyticsFilter{})
	if err != nil || value.Summary.TotalRuns != 2 {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}
func TestAnalyticsRepositoryFailure(t *testing.T) {
	want := errors.New("database unavailable")
	service := New(nil)
	service.SetAnalyticsRepository(analyticsRepoStub{err: want})
	_, err := service.GetProductionHistory(context.Background(), admindomain.Session{Role: "production_manager"}, "00000000-0000-4000-8000-000000000001")
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
