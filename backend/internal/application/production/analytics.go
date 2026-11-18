package production

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	admindomain "example.com/loja-3d/backend/internal/domain/admin"
)

type AnalyticsFilter struct {
	From, To                                          *time.Time
	PrinterID, ProductID, MaterialID, Status, Quality string
	ProductQuery, MaterialQuery                       string
	Page, Limit                                       int
}
type HistoryItem struct {
	ID                string     `json:"id"`
	JobID             string     `json:"job_id"`
	JobNumber         string     `json:"job_number"`
	OrderNumber       string     `json:"order_number"`
	ProductID         string     `json:"product_id"`
	ProductName       string     `json:"product_name"`
	VariantID         string     `json:"variant_id"`
	VariantName       string     `json:"variant_name"`
	PrinterID         string     `json:"printer_id"`
	PrinterName       string     `json:"printer_name"`
	MaterialID        string     `json:"material_id"`
	MaterialName      string     `json:"material_name"`
	ColorName         string     `json:"color_name"`
	Status            string     `json:"status"`
	QualityResult     string     `json:"quality_result"`
	FailureReason     string     `json:"failure_reason"`
	DurationSource    string     `json:"duration_source"`
	Attempt           int        `json:"attempt_number"`
	Quantity          int        `json:"quantity"`
	EstimatedMinutes  *int       `json:"estimated_minutes"`
	ActualMinutes     *int       `json:"actual_minutes"`
	DifferenceMinutes *int       `json:"difference_minutes"`
	DifferencePercent *float64   `json:"difference_percent"`
	ConsumedGrams     int64      `json:"consumed_grams"`
	WastedGrams       int64      `json:"wasted_grams"`
	MaterialCostCents int64      `json:"material_cost_cents"`
	StartedAt         *time.Time `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	IsReprint         bool       `json:"is_reprint"`
}

type HistoryPage struct {
	Items []HistoryItem `json:"items"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
	Total int64         `json:"total"`
}
type RunMaterialView struct {
	SpoolID        string `json:"spool_id"`
	SpoolCode      string `json:"spool_code"`
	MaterialName   string `json:"material_name"`
	ColorName      string `json:"color_name"`
	ConsumedGrams  int    `json:"consumed_grams"`
	WastedGrams    int    `json:"wasted_grams"`
	CostPerKGCents int64  `json:"cost_per_kg_snapshot_cents"`
	TotalCostCents int64  `json:"total_cost_cents"`
}
type InspectionView struct {
	ID                    string    `json:"id"`
	Result                string    `json:"result"`
	Notes                 string    `json:"notes"`
	DimensionalAccuracyOK bool      `json:"dimensional_accuracy_ok"`
	VisualQualityOK       bool      `json:"visual_quality_ok"`
	FunctionalTestOK      *bool     `json:"functional_test_ok"`
	InspectedAt           time.Time `json:"inspected_at"`
}
type AnalyticsEventView struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	PreviousStatus string    `json:"previous_status"`
	NewStatus      string    `json:"new_status"`
	Description    string    `json:"description"`
	OccurredAt     time.Time `json:"occurred_at"`
}
type ObservedRunCandidate struct {
	ID                 string    `json:"id"`
	JobName            string    `json:"job_name"`
	LastState          string    `json:"last_state"`
	FirstObservedAt    time.Time `json:"first_observed_at"`
	LastObservedAt     time.Time `json:"last_observed_at"`
	MaxProgressPercent *float64  `json:"max_progress_percent"`
	Association        string    `json:"association"`
}
type HistoryDetail struct {
	Run                HistoryItem            `json:"run"`
	Materials          []RunMaterialView      `json:"materials"`
	Inspections        []InspectionView       `json:"inspections"`
	Events             []AnalyticsEventView   `json:"events"`
	ObservedCandidates []ObservedRunCandidate `json:"observed_candidates"`
}
type ProductivitySummary struct {
	TotalRuns             int64    `json:"total_runs"`
	CompletedRuns         int64    `json:"completed_runs"`
	FailedRuns            int64    `json:"failed_runs"`
	Reprints              int64    `json:"reprints"`
	SuccessRate           *float64 `json:"success_rate"`
	FailureRate           *float64 `json:"failure_rate"`
	RecordedMinutes       int64    `json:"recorded_minutes"`
	ConsumedGrams         int64    `json:"consumed_grams"`
	WastedGrams           int64    `json:"wasted_grams"`
	MaterialCostCents     int64    `json:"material_cost_cents"`
	UtilizationPercent    *float64 `json:"utilization_percent"`
	UtilizationLimitation string   `json:"utilization_limitation"`
}
type PeriodPoint struct {
	Period            string `json:"period"`
	Runs              int64  `json:"runs"`
	Completed         int64  `json:"completed"`
	Failed            int64  `json:"failed"`
	RecordedMinutes   int64  `json:"recorded_minutes"`
	ConsumedGrams     int64  `json:"consumed_grams"`
	WastedGrams       int64  `json:"wasted_grams"`
	MaterialCostCents int64  `json:"material_cost_cents"`
}
type PrinterProductivity struct {
	PrinterID             string   `json:"printer_id"`
	PrinterName           string   `json:"printer_name"`
	TotalRuns             int64    `json:"total_runs"`
	CompletedRuns         int64    `json:"completed_runs"`
	FailedRuns            int64    `json:"failed_runs"`
	Reprints              int64    `json:"reprints"`
	RecordedMinutes       int64    `json:"recorded_minutes"`
	ConsumedGrams         int64    `json:"consumed_grams"`
	WastedGrams           int64    `json:"wasted_grams"`
	MaterialCostCents     int64    `json:"material_cost_cents"`
	SuccessRate           *float64 `json:"success_rate"`
	FailureRate           *float64 `json:"failure_rate"`
	AverageMinutes        *float64 `json:"average_minutes"`
	MedianMinutes         *float64 `json:"median_minutes"`
	UtilizationPercent    *float64 `json:"utilization_percent"`
	UtilizationLimitation string   `json:"utilization_limitation"`
}
type ProductProductivity struct {
	ProductID         string   `json:"product_id"`
	ProductName       string   `json:"product_name"`
	VariantID         string   `json:"variant_id"`
	VariantName       string   `json:"variant_name"`
	TotalRuns         int64    `json:"total_runs"`
	CompletedRuns     int64    `json:"completed_runs"`
	FailedRuns        int64    `json:"failed_runs"`
	RecordedMinutes   int64    `json:"recorded_minutes"`
	ConsumedGrams     int64    `json:"consumed_grams"`
	MaterialCostCents int64    `json:"material_cost_cents"`
	MedianMinutes     *float64 `json:"median_minutes"`
}
type MaterialProductivity struct {
	MaterialID        string `json:"material_id"`
	MaterialName      string `json:"material_name"`
	ConsumedGrams     int64  `json:"consumed_grams"`
	WastedGrams       int64  `json:"wasted_grams"`
	MaterialCostCents int64  `json:"material_cost_cents"`
}
type TimeAccuracyPoint struct {
	ProductID               string   `json:"product_id"`
	ProductName             string   `json:"product_name"`
	VariantID               string   `json:"variant_id"`
	VariantName             string   `json:"variant_name"`
	Samples                 int64    `json:"samples"`
	MedianEstimatedMinutes  *float64 `json:"median_estimated_minutes"`
	MedianActualMinutes     *float64 `json:"median_actual_minutes"`
	MedianDifferenceMinutes *float64 `json:"median_difference_minutes"`
	MedianDifferencePercent *float64 `json:"median_difference_percent"`
}
type ProductivityAnalytics struct {
	Summary      ProductivitySummary    `json:"summary"`
	Timeline     []PeriodPoint          `json:"timeline"`
	Printers     []PrinterProductivity  `json:"printers"`
	Products     []ProductProductivity  `json:"products"`
	Materials    []MaterialProductivity `json:"materials"`
	TimeAccuracy []TimeAccuracyPoint    `json:"time_accuracy"`
	GeneratedAt  time.Time              `json:"generated_at"`
	Limitations  []string               `json:"limitations"`
}

type AnalyticsRepository interface {
	ListHistory(context.Context, AnalyticsFilter) (HistoryPage, error)
	GetHistory(context.Context, string) (HistoryDetail, error)
	Productivity(context.Context, AnalyticsFilter) (ProductivityAnalytics, error)
}

func (s *Service) SetAnalyticsRepository(repo AnalyticsRepository) { s.analytics = repo }
func validAnalyticsUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	return len(strings.ReplaceAll(value, "-", "")) == 32 && func() bool {
		_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
		return err == nil
	}()
}
func validAnalyticsFilter(f AnalyticsFilter) bool {
	statuses := map[string]bool{"": true, "queued": true, "preparing": true, "printing": true, "post_processing": true, "quality_check": true, "completed": true, "failed": true, "cancelled": true}
	qualities := map[string]bool{"": true, "approved": true, "rejected": true, "conditional": true}
	return f.Page > 0 && f.Limit > 0 && f.Limit <= 100 && (f.From == nil || f.To == nil || !f.From.After(*f.To)) && statuses[f.Status] && qualities[f.Quality] && len(f.ProductQuery) <= 100 && len(f.MaterialQuery) <= 100 &&
		(f.PrinterID == "" || validAnalyticsUUID(f.PrinterID)) && (f.ProductID == "" || validAnalyticsUUID(f.ProductID)) && (f.MaterialID == "" || validAnalyticsUUID(f.MaterialID))
}
func (s *Service) ListProductionHistory(ctx context.Context, a admindomain.Session, f AnalyticsFilter) (HistoryPage, error) {
	if err := require(canRead(a)); err != nil {
		return HistoryPage{}, err
	}
	if !validAnalyticsFilter(f) {
		return HistoryPage{}, ErrInvalid
	}
	return s.analytics.ListHistory(ctx, f)
}
func (s *Service) GetProductionHistory(ctx context.Context, a admindomain.Session, id string) (HistoryDetail, error) {
	if err := require(canRead(a)); err != nil {
		return HistoryDetail{}, err
	}
	if !validAnalyticsUUID(id) {
		return HistoryDetail{}, ErrInvalid
	}
	return s.analytics.GetHistory(ctx, id)
}
func (s *Service) GetProductivityAnalytics(ctx context.Context, a admindomain.Session, f AnalyticsFilter) (ProductivityAnalytics, error) {
	if err := require(canRead(a)); err != nil {
		return ProductivityAnalytics{}, err
	}
	if f.Page == 0 {
		f.Page = 1
	}
	if f.Limit == 0 {
		f.Limit = 100
	}
	if !validAnalyticsFilter(f) {
		return ProductivityAnalytics{}, ErrInvalid
	}
	return s.analytics.Productivity(ctx, f)
}
