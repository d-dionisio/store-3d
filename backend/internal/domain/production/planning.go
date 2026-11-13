package production

import (
	"sort"
	"time"
)

type RiskLevel string

const (
	RiskNormal    RiskLevel = "normal"
	RiskAttention RiskLevel = "attention"
	RiskOverdue   RiskLevel = "overdue"
	RiskBlocked   RiskLevel = "blocked"
)

type RiskReason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ForecastConfidence string

const (
	ConfidenceUnknown ForecastConfidence = "unknown"
	ConfidenceLow     ForecastConfidence = "low"
	ConfidenceMedium  ForecastConfidence = "medium"
	ConfidenceHigh    ForecastConfidence = "high"
)

type ProductionForecast struct {
	EstimatedMinutes *int               `json:"estimated_minutes"`
	RemainingMinutes *int               `json:"remaining_minutes"`
	EstimatedEndAt   *time.Time         `json:"estimated_end_at"`
	Source           string             `json:"source"`
	Confidence       ForecastConfidence `json:"confidence"`
	Samples          int                `json:"samples"`
}

type PrinterCapacity struct {
	PrinterID       string     `json:"printer_id"`
	Status          string     `json:"status"`
	AssignedJobs    int        `json:"assigned_jobs"`
	ActiveRuns      int        `json:"active_runs"`
	KnownMinutes    int        `json:"known_minutes"`
	Available       bool       `json:"available"`
	NextAvailableAt *time.Time `json:"next_available_at"`
}

type MaterialRequirement struct {
	MaterialID, ColorID string
	KnownGrams          int64
	UnknownJobs         int
}

type MaterialAvailability struct {
	MaterialID, ColorID string
	UsableGrams         int64
	MinimumStockGrams   int64
}

func Confidence(samples int) ForecastConfidence {
	switch {
	case samples >= 10:
		return ConfidenceHigh
	case samples >= 3:
		return ConfidenceMedium
	case samples >= 1:
		return ConfidenceLow
	default:
		return ConfidenceUnknown
	}
}

func MedianMinutes(values []int) (int, bool) {
	if len(values) == 0 {
		return 0, false
	}
	copyValues := append([]int(nil), values...)
	sort.Ints(copyValues)
	middle := len(copyValues) / 2
	if len(copyValues)%2 == 1 {
		return copyValues[middle], true
	}
	return (copyValues[middle-1] + copyValues[middle]) / 2, true
}

func RemainingMinutes(estimated int, startedAt *time.Time, now time.Time) int {
	if estimated <= 0 {
		return 0
	}
	if startedAt == nil {
		return estimated
	}
	elapsed := int(now.Sub(*startedAt).Minutes())
	if elapsed >= estimated {
		return 0
	}
	if elapsed < 0 {
		return estimated
	}
	return estimated - elapsed
}

// ClassifyRisk is deterministic. Missing planning data is attention, never a blockage.
func ClassifyRisk(now time.Time, dueAt, estimatedEnd *time.Time, blocked bool, missingData bool) (RiskLevel, []RiskReason) {
	if dueAt != nil && dueAt.Before(now) {
		return RiskOverdue, []RiskReason{{Code: "due_date_passed", Message: "O prazo informado já venceu."}}
	}
	if blocked {
		return RiskBlocked, []RiskReason{{Code: "confirmed_blocker", Message: "Existe um impedimento operacional confirmado."}}
	}
	if missingData || dueAt == nil || estimatedEnd == nil {
		return RiskAttention, []RiskReason{{Code: "insufficient_data", Message: "Faltam dados para uma previsão confiável."}}
	}
	remaining := estimatedEnd.Sub(now)
	buffered := time.Duration(float64(remaining) * 1.2)
	if now.Add(buffered).After(*dueAt) {
		return RiskAttention, []RiskReason{{Code: "forecast_near_due", Message: "A previsão com margem de segurança de 20% alcança o prazo."}}
	}
	return RiskNormal, []RiskReason{{Code: "within_forecast", Message: "A previsão permanece antes do prazo com margem de segurança."}}
}

func IsMaterialInsufficient(requirement MaterialRequirement, availability MaterialAvailability) bool {
	return requirement.KnownGrams > availability.UsableGrams-availability.MinimumStockGrams
}

func IsPrinterUnavailable(status string) bool {
	return status == "maintenance" || status == "offline" || status == "retired"
}
