package production

import (
	"context"
	"time"

	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
)

type Dashboard struct {
	PeriodDays          int `json:"period_days"`
	AwaitingJobs        int `json:"awaiting_jobs"`
	QueuedJobs          int `json:"queued_jobs"`
	PrintingRuns        int `json:"printing_runs"`
	QualityJobs         int `json:"quality_jobs"`
	OverdueJobs         int `json:"overdue_jobs"`
	CompletedJobs       int `json:"completed_jobs"`
	AvailablePrinters   int `json:"available_printers"`
	BusyPrinters        int `json:"busy_printers"`
	MaintenancePrinters int `json:"maintenance_printers"`
	LowStockMaterials   int `json:"low_stock_materials"`
}

type PlanningFilter struct {
	Status, Risk, PrinterID string
	DueFrom, DueTo          *time.Time
	Page, Limit             int
}

type PlanningInput struct {
	ID, Number, OrderNumber, ProductName, VariantName string
	Status, PrinterID, PrinterName, PrinterStatus     string
	MaterialID, MaterialName, ColorID, ColorName      string
	Quantity, Priority                                int
	DueAt, StartedAt                                  *time.Time
	ExplicitEstimatedMinutes, HistoricalMedianMinutes *int
	EstimatedWeightGrams                              *int
	HistoricalSamples                                 int
	MaterialInsufficient                              bool
}

type PlanningJob struct {
	ID                   string                    `json:"id"`
	Number               string                    `json:"job_number"`
	OrderNumber          string                    `json:"order_number"`
	ProductName          string                    `json:"product_name"`
	VariantName          string                    `json:"variant_name,omitempty"`
	Status               string                    `json:"status"`
	PrinterID            string                    `json:"printer_id,omitempty"`
	PrinterName          string                    `json:"printer_name,omitempty"`
	MaterialName         string                    `json:"material_name"`
	ColorName            string                    `json:"color_name"`
	Quantity             int                       `json:"quantity"`
	Priority             int                       `json:"priority"`
	DueAt                *time.Time                `json:"due_at"`
	EstimatedWeightGrams *int                      `json:"estimated_weight_grams"`
	Risk                 domain.RiskLevel          `json:"risk"`
	Reasons              []domain.RiskReason       `json:"reasons"`
	Forecast             domain.ProductionForecast `json:"forecast"`
}

type PlanningPage struct {
	Items      []PlanningJob            `json:"items"`
	Page       int                      `json:"page"`
	Limit      int                      `json:"limit"`
	Total      int64                    `json:"total"`
	RiskCounts map[domain.RiskLevel]int `json:"risk_counts"`
}

type PrinterCapacityView struct {
	ID                string     `json:"id"`
	Code              string     `json:"code"`
	Name              string     `json:"name"`
	Manufacturer      string     `json:"manufacturer"`
	Model             string     `json:"model"`
	Status            string     `json:"status"`
	AssignedJobs      int        `json:"assigned_jobs"`
	ActiveRuns        int        `json:"active_runs"`
	KnownMinutes      int        `json:"known_minutes"`
	Available         bool       `json:"available"`
	NextAvailableAt   *time.Time `json:"next_available_at"`
	MaintenanceEvents int        `json:"maintenance_events"`
	LastMaintenanceAt *time.Time `json:"last_maintenance_at"`
}

type MaterialPlanningView struct {
	MaterialID         string `json:"material_id"`
	MaterialName       string `json:"material_name"`
	ColorID            string `json:"color_id"`
	ColorName          string `json:"color_name"`
	UsableGrams        int64  `json:"usable_grams"`
	MinimumStockGrams  int64  `json:"minimum_stock_grams"`
	KnownRequiredGrams int64  `json:"known_required_grams"`
	UnknownDemandJobs  int    `json:"unknown_demand_jobs"`
	SpoolCount         int    `json:"spool_count"`
	Insufficient       bool   `json:"insufficient"`
}

type PlanningRepository interface {
	ProductionDashboard(context.Context, time.Time) (Dashboard, error)
	PlanningInputs(context.Context, PlanningFilter) ([]PlanningInput, int64, error)
	PrinterCapacities(context.Context, time.Time) ([]PrinterCapacityView, error)
	MaterialPlanning(context.Context) ([]MaterialPlanningView, error)
}

func (s *Service) GetProductionDashboard(ctx context.Context, actor admindomain.Session) (Dashboard, error) {
	if err := require(canRead(actor)); err != nil {
		return Dashboard{}, err
	}
	if s.planning == nil {
		return Dashboard{}, ErrInvalid
	}
	return s.planning.ProductionDashboard(ctx, time.Now().UTC().Add(-30*24*time.Hour))
}

func (s *Service) ListProductionPlanning(ctx context.Context, actor admindomain.Session, filter PlanningFilter) (PlanningPage, error) {
	if err := require(canRead(actor)); err != nil {
		return PlanningPage{}, err
	}
	validRisk := filter.Risk == "" || filter.Risk == string(domain.RiskNormal) || filter.Risk == string(domain.RiskAttention) || filter.Risk == string(domain.RiskOverdue) || filter.Risk == string(domain.RiskBlocked)
	validStatus := filter.Status == "" || filter.Status == "awaiting_production" || filter.Status == "queued" || filter.Status == "preparing" || filter.Status == "printing" || filter.Status == "post_processing" || filter.Status == "quality_check" || filter.Status == "completed" || filter.Status == "failed" || filter.Status == "cancelled"
	validDates := filter.DueFrom == nil || filter.DueTo == nil || !filter.DueFrom.After(*filter.DueTo)
	if s.planning == nil || filter.Page < 1 || filter.Limit < 1 || filter.Limit > 100 || !validRisk || !validStatus || !validDates {
		return PlanningPage{}, ErrInvalid
	}
	repositoryFilter := filter
	repositoryFilter.Page = 1
	repositoryFilter.Limit = 1000000
	inputs, total, err := s.planning.PlanningInputs(ctx, repositoryFilter)
	if err != nil {
		return PlanningPage{}, err
	}
	now := time.Now().UTC()
	result := PlanningPage{Items: []PlanningJob{}, Page: filter.Page, Limit: filter.Limit, Total: total, RiskCounts: map[domain.RiskLevel]int{domain.RiskNormal: 0, domain.RiskAttention: 0, domain.RiskOverdue: 0, domain.RiskBlocked: 0}}
	for _, input := range inputs {
		forecast := buildForecast(input, now)
		blocked := domain.IsPrinterUnavailable(input.PrinterStatus) || input.MaterialInsufficient
		risk, reasons := domain.ClassifyRisk(now, input.DueAt, forecast.EstimatedEndAt, blocked, forecast.EstimatedMinutes == nil)
		result.RiskCounts[risk]++
		if filter.Risk != "" && string(risk) != filter.Risk {
			continue
		}
		result.Items = append(result.Items, PlanningJob{ID: input.ID, Number: input.Number, OrderNumber: input.OrderNumber, ProductName: input.ProductName, VariantName: input.VariantName, Status: input.Status, PrinterID: input.PrinterID, PrinterName: input.PrinterName, MaterialName: input.MaterialName, ColorName: input.ColorName, Quantity: input.Quantity, Priority: input.Priority, DueAt: input.DueAt, EstimatedWeightGrams: input.EstimatedWeightGrams, Risk: risk, Reasons: reasons, Forecast: forecast})
	}
	result.Total = int64(len(result.Items))
	start := (filter.Page - 1) * filter.Limit
	if start >= len(result.Items) {
		result.Items = []PlanningJob{}
	} else {
		end := start + filter.Limit
		if end > len(result.Items) {
			end = len(result.Items)
		}
		result.Items = result.Items[start:end]
	}
	return result, nil
}

func buildForecast(input PlanningInput, now time.Time) domain.ProductionForecast {
	var estimated *int
	source := "unknown"
	samples := 0
	if input.ExplicitEstimatedMinutes != nil {
		estimated = input.ExplicitEstimatedMinutes
		source = "explicit"
	} else if input.HistoricalMedianMinutes != nil {
		estimated = input.HistoricalMedianMinutes
		source = "historical_median"
		samples = input.HistoricalSamples
	}
	f := domain.ProductionForecast{EstimatedMinutes: estimated, Source: source, Samples: samples, Confidence: domain.Confidence(samples)}
	if source == "explicit" {
		f.Confidence = domain.ConfidenceUnknown
	}
	if estimated != nil {
		remaining := domain.RemainingMinutes(*estimated, input.StartedAt, now)
		f.RemainingMinutes = &remaining
		end := now.Add(time.Duration(remaining) * time.Minute)
		f.EstimatedEndAt = &end
	}
	return f
}

func (s *Service) GetPrinterCapacity(ctx context.Context, actor admindomain.Session) ([]PrinterCapacityView, error) {
	if err := require(canRead(actor)); err != nil {
		return nil, err
	}
	if s.planning == nil {
		return nil, ErrInvalid
	}
	return s.planning.PrinterCapacities(ctx, time.Now().UTC())
}
func (s *Service) GetMaterialPlanning(ctx context.Context, actor admindomain.Session) ([]MaterialPlanningView, error) {
	if err := require(canRead(actor)); err != nil {
		return nil, err
	}
	if s.planning == nil {
		return nil, ErrInvalid
	}
	return s.planning.MaterialPlanning(ctx)
}
