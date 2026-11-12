package production

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	ErrInvalidQuantity   = errors.New("production quantity must be positive")
	ErrInvalidTransition = errors.New("invalid production state transition")
	ErrInvalidCost       = errors.New("production cost cannot be negative")
	ErrInsufficientStock = errors.New("insufficient material spool balance")
	ErrInvalidReprint    = errors.New("reprint requires a failed or rejected run")
	ErrInvalidInspection = errors.New("invalid quality inspection")
	ErrFinancialGate     = errors.New("order is not financially eligible for production")
)

type JobStatus string

const (
	JobAwaitingProduction JobStatus = "awaiting_production"
	JobQueued             JobStatus = "queued"
	JobPreparing          JobStatus = "preparing"
	JobPrinting           JobStatus = "printing"
	JobPostProcessing     JobStatus = "post_processing"
	JobQualityCheck       JobStatus = "quality_check"
	JobCompleted          JobStatus = "completed"
	JobFailed             JobStatus = "failed"
	JobCancelled          JobStatus = "cancelled"
)

type ProductionJob struct {
	ID, Number, OrderItemID, ParentJobID string
	Status                               JobStatus
	Quantity, Priority                   int
	Version                              int64
	DueAt                                *time.Time
}

func NewProductionJob(id, number, orderItemID string, quantity int) (ProductionJob, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(number) == "" || strings.TrimSpace(orderItemID) == "" {
		return ProductionJob{}, errors.New("production job identity is required")
	}
	if quantity <= 0 {
		return ProductionJob{}, ErrInvalidQuantity
	}
	return ProductionJob{ID: id, Number: number, OrderItemID: orderItemID, Quantity: quantity, Priority: 50, Version: 1, Status: JobAwaitingProduction}, nil
}

func (j *ProductionJob) Transition(to JobStatus) error {
	if !validJobTransition(j.Status, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, j.Status, to)
	}
	j.Status = to
	j.Version++
	return nil
}

func validJobTransition(from, to JobStatus) bool {
	allowed := map[JobStatus]map[JobStatus]bool{
		JobAwaitingProduction: {JobQueued: true, JobCancelled: true},
		JobQueued:             {JobPreparing: true, JobFailed: true, JobCancelled: true},
		JobPreparing:          {JobQueued: true, JobPrinting: true, JobFailed: true, JobCancelled: true},
		JobPrinting:           {JobPostProcessing: true, JobQualityCheck: true, JobFailed: true, JobCancelled: true},
		JobPostProcessing:     {JobQualityCheck: true, JobFailed: true, JobCancelled: true},
		JobQualityCheck:       {JobQueued: true, JobCompleted: true, JobFailed: true, JobCancelled: true},
	}
	return allowed[from][to]
}

type FinancialSnapshot struct {
	OrderStatus, OrderCurrency, PaymentStatus, PaymentCurrency string
	OrderTotalCents, PaymentAmountCents, PaidAmountCents       int64
	RefundedAmountCents                                        int64
}

func (f FinancialSnapshot) ValidateForQueue() error {
	if (f.OrderStatus != "confirmed" && f.OrderStatus != "in_production") ||
		f.PaymentStatus != "paid" || f.OrderTotalCents <= 0 ||
		f.PaymentAmountCents != f.OrderTotalCents || f.PaidAmountCents != f.OrderTotalCents ||
		f.RefundedAmountCents != 0 || f.OrderCurrency == "" || f.PaymentCurrency != f.OrderCurrency {
		return ErrFinancialGate
	}
	return nil
}

type RunStatus string

const (
	RunQueued         RunStatus = "queued"
	RunPreparing      RunStatus = "preparing"
	RunPrinting       RunStatus = "printing"
	RunPostProcessing RunStatus = "post_processing"
	RunQualityCheck   RunStatus = "quality_check"
	RunCompleted      RunStatus = "completed"
	RunFailed         RunStatus = "failed"
	RunCancelled      RunStatus = "cancelled"
)

type ProductionRun struct {
	ID, JobID, ReprintOfRunID, PrinterID string
	AttemptNumber, Quantity              int
	Status                               RunStatus
	MaterialCostCents, EnergyCostCents   int64
	OtherCostCents                       int64
	Version                              int64
}

func NewProductionRun(id, jobID string, attempt, quantity int) (ProductionRun, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(jobID) == "" || attempt <= 0 {
		return ProductionRun{}, errors.New("production run identity is invalid")
	}
	if quantity <= 0 {
		return ProductionRun{}, ErrInvalidQuantity
	}
	return ProductionRun{ID: id, JobID: jobID, AttemptNumber: attempt, Quantity: quantity, Status: RunQueued, Version: 1}, nil
}

func NewReprint(id string, previous ProductionRun, latestInspection InspectionResult) (ProductionRun, error) {
	if previous.Status != RunFailed && latestInspection != InspectionRejected {
		return ProductionRun{}, ErrInvalidReprint
	}
	run, err := NewProductionRun(id, previous.JobID, previous.AttemptNumber+1, previous.Quantity)
	if err != nil {
		return ProductionRun{}, err
	}
	run.ReprintOfRunID = previous.ID
	return run, nil
}

func (r *ProductionRun) Transition(to RunStatus) error {
	allowed := map[RunStatus]map[RunStatus]bool{
		RunQueued:         {RunPreparing: true, RunCancelled: true},
		RunPreparing:      {RunPrinting: true, RunFailed: true, RunCancelled: true},
		RunPrinting:       {RunPostProcessing: true, RunQualityCheck: true, RunFailed: true, RunCancelled: true},
		RunPostProcessing: {RunQualityCheck: true, RunFailed: true, RunCancelled: true},
		RunQualityCheck:   {RunCompleted: true, RunFailed: true, RunCancelled: true},
	}
	if !allowed[r.Status][to] {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, r.Status, to)
	}
	if (to == RunPreparing || to == RunPrinting || to == RunPostProcessing || to == RunQualityCheck || to == RunCompleted) && strings.TrimSpace(r.PrinterID) == "" {
		return errors.New("printer is required for execution")
	}
	r.Status = to
	r.Version++
	return nil
}

func (r ProductionRun) ValidateCosts() error {
	if r.MaterialCostCents < 0 || r.EnergyCostCents < 0 || r.OtherCostCents < 0 {
		return ErrInvalidCost
	}
	return nil
}

type PrinterStatus string

const (
	PrinterAvailable   PrinterStatus = "available"
	PrinterBusy        PrinterStatus = "busy"
	PrinterMaintenance PrinterStatus = "maintenance"
	PrinterOffline     PrinterStatus = "offline"
	PrinterRetired     PrinterStatus = "retired"
)

type Printer struct {
	ID, Code, Name, Manufacturer, Model string
	Status                              PrinterStatus
	BuildWidthMM, BuildDepthMM          int
	BuildHeightMM, NozzleMicrons        int
}

func (p Printer) Validate() error {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Code) == "" || strings.TrimSpace(p.Name) == "" ||
		strings.TrimSpace(p.Manufacturer) == "" || strings.TrimSpace(p.Model) == "" ||
		p.BuildWidthMM <= 0 || p.BuildDepthMM <= 0 || p.BuildHeightMM <= 0 || p.NozzleMicrons <= 0 {
		return errors.New("invalid printer")
	}
	return nil
}

type MaterialSpool struct {
	ID, Code, MaterialID, ColorID string
	Status                        string
	InitialWeightGrams            int64
	CurrentWeightGrams            int64
	CostPerKGCents                int64
	Version                       int64
}

type MaterialConsumption struct {
	ConsumedGrams, WastedGrams int64
	CostPerKGCents             int64
	TotalCostCents             int64
}

func (s *MaterialSpool) Consume(consumedGrams, wastedGrams int64) (MaterialConsumption, error) {
	if consumedGrams < 0 || wastedGrams < 0 || consumedGrams+wastedGrams <= 0 {
		return MaterialConsumption{}, ErrInvalidQuantity
	}
	if s.CostPerKGCents < 0 {
		return MaterialConsumption{}, ErrInvalidCost
	}
	total := consumedGrams + wastedGrams
	if total > s.CurrentWeightGrams {
		return MaterialConsumption{}, ErrInsufficientStock
	}
	if total > 0 && s.CostPerKGCents > math.MaxInt64/total {
		return MaterialConsumption{}, ErrInvalidCost
	}
	cost := (total*s.CostPerKGCents + 500) / 1000
	s.CurrentWeightGrams -= total
	s.Version++
	if s.CurrentWeightGrams == 0 {
		s.Status = "depleted"
	} else {
		s.Status = "in_use"
	}
	return MaterialConsumption{ConsumedGrams: consumedGrams, WastedGrams: wastedGrams, CostPerKGCents: s.CostPerKGCents, TotalCostCents: cost}, nil
}

type InspectionResult string

const (
	InspectionApproved    InspectionResult = "approved"
	InspectionRejected    InspectionResult = "rejected"
	InspectionConditional InspectionResult = "conditional"
)

type QualityInspection struct {
	ID, RunID, InspectorAdminID string
	Result                      InspectionResult
	DimensionalAccuracyOK       bool
	VisualQualityOK             bool
	FunctionalTestOK            *bool
	Notes                       string
	InspectedAt                 time.Time
}

func (q QualityInspection) Validate() error {
	if strings.TrimSpace(q.ID) == "" || strings.TrimSpace(q.RunID) == "" || strings.TrimSpace(q.InspectorAdminID) == "" || q.InspectedAt.IsZero() {
		return ErrInvalidInspection
	}
	if q.Result != InspectionApproved && q.Result != InspectionRejected && q.Result != InspectionConditional {
		return ErrInvalidInspection
	}
	if q.Result == InspectionApproved && (!q.DimensionalAccuracyOK || !q.VisualQualityOK || (q.FunctionalTestOK != nil && !*q.FunctionalTestOK)) {
		return ErrInvalidInspection
	}
	if q.Result == InspectionRejected && strings.TrimSpace(q.Notes) == "" {
		return ErrInvalidInspection
	}
	return nil
}

type ProductionEvent struct {
	ID             string    `json:"id"`
	JobID          string    `json:"job_id"`
	RunID          string    `json:"run_id,omitempty"`
	AdminUserID    string    `json:"admin_user_id,omitempty"`
	PreviousStatus string    `json:"previous_status,omitempty"`
	NewStatus      string    `json:"new_status,omitempty"`
	Description    string    `json:"description"`
	OccurredAt     time.Time `json:"occurred_at"`
}

func (e ProductionEvent) Validate() error {
	if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.JobID) == "" || strings.TrimSpace(e.Description) == "" || e.OccurredAt.IsZero() {
		return errors.New("invalid production event")
	}
	if (e.PreviousStatus == "") != (e.NewStatus == "") {
		return errors.New("production event transition must contain both states")
	}
	return nil
}
