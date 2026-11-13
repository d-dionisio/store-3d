package production

import (
	"context"
	"errors"
	"strings"
	"time"

	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
)

var (
	ErrForbidden = errors.New("production access forbidden")
	ErrInvalid   = errors.New("invalid production input")
	ErrNotFound  = errors.New("production resource not found")
	ErrConflict  = errors.New("production conflict")
)

type Page struct {
	Items []JobView `json:"items"`
	Page  int       `json:"page"`
	Limit int       `json:"limit"`
	Total int64     `json:"total"`
}
type JobView struct {
	ID          string                   `json:"id"`
	Number      string                   `json:"job_number"`
	OrderNumber string                   `json:"order_number"`
	ProductName string                   `json:"product_name"`
	Status      string                   `json:"status"`
	Quantity    int                      `json:"quantity"`
	Priority    int                      `json:"priority"`
	Version     int64                    `json:"version"`
	DueAt       *time.Time               `json:"due_at"`
	Runs        []RunView                `json:"runs,omitempty"`
	Events      []domain.ProductionEvent `json:"events,omitempty"`
}
type RunView struct {
	ID                string `json:"id"`
	JobID             string `json:"job_id"`
	PrinterID         string `json:"printer_id,omitempty"`
	Status            string `json:"status"`
	Attempt           int    `json:"attempt_number"`
	Quantity          int    `json:"quantity"`
	Version           int64  `json:"version"`
	MaterialCostCents int64  `json:"material_cost_cents"`
	EnergyCostCents   int64  `json:"energy_cost_cents"`
	OtherCostCents    int64  `json:"other_cost_cents"`
}
type PrinterInput struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Manufacturer  string `json:"manufacturer"`
	Model         string `json:"model"`
	BuildWidthMM  int    `json:"build_width_mm"`
	BuildDepthMM  int    `json:"build_depth_mm"`
	BuildHeightMM int    `json:"build_height_mm"`
	NozzleMicrons int    `json:"nozzle_diameter_microns"`
}
type MaintenanceInput struct {
	EventType   string     `json:"event_type"`
	Description string     `json:"description"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	CostCents   int64      `json:"cost_cents"`
}
type SpoolInput struct {
	Code               string `json:"code"`
	MaterialID         string `json:"material_id"`
	ColorID            string `json:"color_id"`
	Manufacturer       string `json:"manufacturer"`
	ProductLine        string `json:"product_line"`
	LotNumber          string `json:"lot_number"`
	InitialWeightGrams int    `json:"initial_weight_grams"`
	CurrentWeightGrams int    `json:"current_weight_grams"`
	MinimumStockGrams  int    `json:"minimum_stock_grams"`
	CostPerKGCents     int64  `json:"cost_per_kg_cents"`
}
type InspectionInput struct {
	Result                domain.InspectionResult `json:"result"`
	DimensionalAccuracyOK bool                    `json:"dimensional_accuracy_ok"`
	VisualQualityOK       bool                    `json:"visual_quality_ok"`
	FunctionalTestOK      *bool                   `json:"functional_test_ok"`
	Notes                 string                  `json:"notes"`
}
type PrinterView struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Manufacturer  string    `json:"manufacturer"`
	Model         string    `json:"model"`
	Status        string    `json:"status"`
	BuildWidthMM  int       `json:"build_width_mm"`
	BuildDepthMM  int       `json:"build_depth_mm"`
	BuildHeightMM int       `json:"build_height_mm"`
	NozzleMicrons int       `json:"nozzle_diameter_microns"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type SpoolView struct {
	ID                 string    `json:"id"`
	Code               string    `json:"code"`
	MaterialID         string    `json:"material_id"`
	ColorID            string    `json:"color_id"`
	Manufacturer       string    `json:"manufacturer"`
	Status             string    `json:"status"`
	InitialWeightGrams int       `json:"initial_weight_grams"`
	CurrentWeightGrams int       `json:"current_weight_grams"`
	CostPerKGCents     int64     `json:"cost_per_kg_cents"`
	Version            int64     `json:"version"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Repository interface {
	CreateJobsForPaidOrder(context.Context, string) (int, error)
	ListJobs(context.Context, string, int, int) (Page, error)
	GetJob(context.Context, string) (JobView, error)
	TransitionJob(context.Context, string, domain.JobStatus, string, string, int64) (JobView, error)
	StartPreparation(context.Context, string, string, string, int64) (JobView, error)
	TransitionRun(context.Context, string, domain.RunStatus, string, string, int64) (RunView, error)
	CreateReprint(context.Context, string, string, string) (RunView, error)
	CreatePrinter(context.Context, PrinterInput) (PrinterView, error)
	UpdatePrinter(context.Context, string, PrinterInput) (PrinterView, error)
	ListPrinters(context.Context) ([]PrinterView, error)
	ChangePrinterStatus(context.Context, string, string) (PrinterView, error)
	RegisterMaintenance(context.Context, string, string, MaintenanceInput) (string, error)
	RegisterSpool(context.Context, SpoolInput) (SpoolView, error)
	UpdateSpool(context.Context, string, SpoolInput, int64) (SpoolView, error)
	ListSpools(context.Context, string) ([]SpoolView, error)
	GetSpool(context.Context, string) (SpoolView, error)
	AdjustSpool(context.Context, string, int, string, string, int64) (SpoolView, error)
	ConsumeMaterial(context.Context, string, string, int, int, string) error
	QuarantineSpool(context.Context, string, string, int64) (SpoolView, error)
	Inspect(context.Context, string, string, InspectionInput) (string, error)
	ApproveRun(context.Context, string, string, string, InspectionInput, int64) (RunView, error)
	RejectRun(context.Context, string, string, InspectionInput) (string, error)
}

type Service struct {
	repo     Repository
	planning PlanningRepository
}

func New(repo Repository) *Service                               { return &Service{repo: repo} }
func (s *Service) SetPlanningRepository(repo PlanningRepository) { s.planning = repo }
func canRead(s admindomain.Session) bool {
	return s.Role == "admin" || s.Role == "production_manager" || s.Role == "production_operator"
}
func canManage(s admindomain.Session) bool {
	return s.Role == "admin" || s.Role == "production_manager"
}
func require(ok bool) error {
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Service) CreateProductionJobsForPaidOrder(ctx context.Context, a admindomain.Session, order string) (int, error) {
	if err := require(canManage(a)); err != nil {
		return 0, err
	}
	return s.repo.CreateJobsForPaidOrder(ctx, order)
}
func (s *Service) ListProductionQueue(ctx context.Context, a admindomain.Session, status string, page, limit int) (Page, error) {
	if err := require(canRead(a)); err != nil {
		return Page{}, err
	}
	if page < 1 || limit < 1 || limit > 100 {
		return Page{}, ErrInvalid
	}
	return s.repo.ListJobs(ctx, status, page, limit)
}
func (s *Service) GetProductionJob(ctx context.Context, a admindomain.Session, id string) (JobView, error) {
	if err := require(canRead(a)); err != nil {
		return JobView{}, err
	}
	return s.repo.GetJob(ctx, id)
}
func (s *Service) QueueProductionJob(ctx context.Context, a admindomain.Session, id, description string, version int64) (JobView, error) {
	if err := require(canManage(a)); err != nil {
		return JobView{}, err
	}
	return s.repo.TransitionJob(ctx, id, domain.JobQueued, a.UserID, description, version)
}
func (s *Service) StartPreparation(ctx context.Context, a admindomain.Session, job, printer string, version int64) (JobView, error) {
	if err := require(canRead(a)); err != nil {
		return JobView{}, err
	}
	return s.repo.StartPreparation(ctx, job, printer, a.UserID, version)
}
func (s *Service) StartProductionRun(ctx context.Context, a admindomain.Session, id string, version int64) (RunView, error) {
	if err := require(canRead(a)); err != nil {
		return RunView{}, err
	}
	return s.repo.TransitionRun(ctx, id, domain.RunPrinting, a.UserID, "Impressão iniciada.", version)
}
func (s *Service) FinishProductionRun(ctx context.Context, a admindomain.Session, id string, version int64) (RunView, error) {
	if err := require(canRead(a)); err != nil {
		return RunView{}, err
	}
	return s.repo.TransitionRun(ctx, id, domain.RunQualityCheck, a.UserID, "Execução enviada ao controle de qualidade.", version)
}
func (s *Service) FailProductionRun(ctx context.Context, a admindomain.Session, id, reason string, version int64) (RunView, error) {
	if err := require(canRead(a)); err != nil {
		return RunView{}, err
	}
	if strings.TrimSpace(reason) == "" {
		return RunView{}, ErrInvalid
	}
	return s.repo.TransitionRun(ctx, id, domain.RunFailed, a.UserID, reason, version)
}
func (s *Service) CreateReprint(ctx context.Context, a admindomain.Session, run, printer string) (RunView, error) {
	if err := require(canManage(a)); err != nil {
		return RunView{}, err
	}
	return s.repo.CreateReprint(ctx, run, printer, a.UserID)
}
func (s *Service) CompleteProductionJob(ctx context.Context, a admindomain.Session, id string, version int64) (JobView, error) {
	if err := require(canManage(a)); err != nil {
		return JobView{}, err
	}
	return s.repo.TransitionJob(ctx, id, domain.JobCompleted, a.UserID, "Produção concluída.", version)
}
func (s *Service) CancelProductionJob(ctx context.Context, a admindomain.Session, id, reason string, version int64) (JobView, error) {
	if err := require(canManage(a)); err != nil {
		return JobView{}, err
	}
	if strings.TrimSpace(reason) == "" {
		return JobView{}, ErrInvalid
	}
	return s.repo.TransitionJob(ctx, id, domain.JobCancelled, a.UserID, reason, version)
}

func (s *Service) CreatePrinter(ctx context.Context, a admindomain.Session, in PrinterInput) (PrinterView, error) {
	if err := require(canManage(a)); err != nil {
		return PrinterView{}, err
	}
	if in.NozzleMicrons == 0 {
		in.NozzleMicrons = 400
	}
	return s.repo.CreatePrinter(ctx, in)
}
func (s *Service) UpdatePrinter(ctx context.Context, a admindomain.Session, id string, in PrinterInput) (PrinterView, error) {
	if err := require(canManage(a)); err != nil {
		return PrinterView{}, err
	}
	return s.repo.UpdatePrinter(ctx, id, in)
}
func (s *Service) ListPrinters(ctx context.Context, a admindomain.Session) ([]PrinterView, error) {
	if err := require(canRead(a)); err != nil {
		return nil, err
	}
	return s.repo.ListPrinters(ctx)
}
func (s *Service) ChangePrinterStatus(ctx context.Context, a admindomain.Session, id, status string) (PrinterView, error) {
	if err := require(canManage(a)); err != nil {
		return PrinterView{}, err
	}
	return s.repo.ChangePrinterStatus(ctx, id, status)
}
func (s *Service) RegisterPrinterMaintenance(ctx context.Context, a admindomain.Session, printer string, in MaintenanceInput) (string, error) {
	if err := require(canManage(a)); err != nil {
		return "", err
	}
	return s.repo.RegisterMaintenance(ctx, printer, a.UserID, in)
}
func (s *Service) RegisterMaterialSpool(ctx context.Context, a admindomain.Session, in SpoolInput) (SpoolView, error) {
	if err := require(canManage(a)); err != nil {
		return SpoolView{}, err
	}
	return s.repo.RegisterSpool(ctx, in)
}
func (s *Service) UpdateMaterialSpool(ctx context.Context, a admindomain.Session, id string, in SpoolInput, version int64) (SpoolView, error) {
	if err := require(canManage(a)); err != nil {
		return SpoolView{}, err
	}
	return s.repo.UpdateSpool(ctx, id, in, version)
}
func (s *Service) ListMaterialSpools(ctx context.Context, a admindomain.Session, status string) ([]SpoolView, error) {
	if err := require(canRead(a)); err != nil {
		return nil, err
	}
	return s.repo.ListSpools(ctx, status)
}
func (s *Service) GetMaterialSpool(ctx context.Context, a admindomain.Session, id string) (SpoolView, error) {
	if err := require(canRead(a)); err != nil {
		return SpoolView{}, err
	}
	return s.repo.GetSpool(ctx, id)
}
func (s *Service) AdjustSpoolBalance(ctx context.Context, a admindomain.Session, id string, delta int, reason string, version int64) (SpoolView, error) {
	if err := require(canManage(a)); err != nil {
		return SpoolView{}, err
	}
	return s.repo.AdjustSpool(ctx, id, delta, reason, a.UserID, version)
}
func (s *Service) RecordMaterialConsumption(ctx context.Context, a admindomain.Session, run, spool string, consumed, wasted int) error {
	if err := require(canRead(a)); err != nil {
		return err
	}
	return s.repo.ConsumeMaterial(ctx, run, spool, consumed, wasted, a.UserID)
}
func (s *Service) QuarantineMaterialSpool(ctx context.Context, a admindomain.Session, id, reason string, version int64) (SpoolView, error) {
	if err := require(canManage(a)); err != nil {
		return SpoolView{}, err
	}
	return s.repo.QuarantineSpool(ctx, id, reason, version)
}
func (s *Service) CreateQualityInspection(ctx context.Context, a admindomain.Session, run string, in InspectionInput) (string, error) {
	if err := require(canRead(a)); err != nil {
		return "", err
	}
	return s.repo.Inspect(ctx, run, a.UserID, in)
}
func (s *Service) ApproveProductionRun(ctx context.Context, a admindomain.Session, run string, in InspectionInput, version int64) (RunView, error) {
	if err := require(canManage(a)); err != nil {
		return RunView{}, err
	}
	in.Result = domain.InspectionApproved
	return s.repo.ApproveRun(ctx, run, a.UserID, "Qualidade aprovada.", in, version)
}
func (s *Service) RejectProductionRun(ctx context.Context, a admindomain.Session, run string, in InspectionInput) (string, error) {
	if err := require(canManage(a)); err != nil {
		return "", err
	}
	in.Result = domain.InspectionRejected
	return s.repo.RejectRun(ctx, run, a.UserID, in)
}
