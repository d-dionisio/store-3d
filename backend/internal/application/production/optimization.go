package production

import (
	"context"
	"time"

	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
)

type OptimizationSnapshot struct {
	Jobs     []domain.OptimizationJob
	Printers []domain.OptimizationPrinter
}

type OptimizationRepository interface {
	OptimizationSnapshot(context.Context, time.Time) (OptimizationSnapshot, error)
}

type JobRecommendation struct {
	JobID        string                         `json:"job_id"`
	JobNumber    string                         `json:"job_number"`
	ProductName  string                         `json:"product_name"`
	Risk         domain.RiskLevel               `json:"risk"`
	Recommended  *domain.PrinterRecommendation  `json:"recommended"`
	Alternatives []domain.PrinterRecommendation `json:"alternatives"`
}

type OptimizationOverview struct {
	GeneratedAt      time.Time           `json:"generated_at"`
	ConsultativeOnly bool                `json:"consultative_only"`
	Jobs             int                 `json:"jobs"`
	Printers         int                 `json:"printers"`
	Recommendations  []JobRecommendation `json:"recommendations"`
	Strategies       []domain.Simulation `json:"strategies"`
	Limitations      []string            `json:"limitations"`
}

func (s *Service) GetOptimization(ctx context.Context, actor admindomain.Session) (OptimizationOverview, error) {
	if err := require(canRead(actor)); err != nil {
		return OptimizationOverview{}, err
	}
	if s.optimization == nil {
		return OptimizationOverview{}, ErrInvalid
	}
	now := time.Now().UTC()
	snapshot, err := s.optimization.OptimizationSnapshot(ctx, now)
	if err != nil {
		return OptimizationOverview{}, err
	}
	result := OptimizationOverview{GeneratedAt: now, ConsultativeOnly: true, Jobs: len(snapshot.Jobs), Printers: len(snapshot.Printers), Recommendations: []JobRecommendation{}, Strategies: []domain.Simulation{}, Limitations: []string{"Capacidades técnicas por material e temperatura ainda não estão modeladas.", "Trocas de material são inferidas da última carga ativa; não existe sensor de bobina carregada."}}
	for _, job := range snapshot.Jobs {
		ranked := domain.RankPrinters(job, snapshot.Printers)
		var estimatedEnd *time.Time
		if job.EstimatedMinutes != nil {
			end := now.Add(time.Duration(*job.EstimatedMinutes) * time.Minute)
			estimatedEnd = &end
		}
		risk, _ := domain.ClassifyRisk(now, job.DueAt, estimatedEnd, false, job.EstimatedMinutes == nil)
		rec := JobRecommendation{JobID: job.ID, JobNumber: job.Number, ProductName: job.ProductName, Risk: risk, Alternatives: ranked}
		if len(ranked) > 0 && ranked[0].Compatibility != domain.CompatibilityInvalid {
			copy := ranked[0]
			rec.Recommended = &copy
		}
		result.Recommendations = append(result.Recommendations, rec)
	}
	for _, strategy := range []domain.OptimizationStrategy{domain.StrategyFIFO, domain.StrategyDueDate, domain.StrategyShortest, domain.StrategyMaterial, domain.StrategyCombined} {
		result.Strategies = append(result.Strategies, domain.Simulate(now, snapshot.Jobs, snapshot.Printers, strategy))
	}
	return result, nil
}

func (s *Service) SimulateOptimization(ctx context.Context, actor admindomain.Session, strategy domain.OptimizationStrategy) (domain.Simulation, error) {
	if err := require(canRead(actor)); err != nil {
		return domain.Simulation{}, err
	}
	if s.optimization == nil || !domain.ValidStrategy(strategy) {
		return domain.Simulation{}, ErrInvalid
	}
	now := time.Now().UTC()
	snapshot, err := s.optimization.OptimizationSnapshot(ctx, now)
	if err != nil {
		return domain.Simulation{}, err
	}
	return domain.Simulate(now, snapshot.Jobs, snapshot.Printers, strategy), nil
}
