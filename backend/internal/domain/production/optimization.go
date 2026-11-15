package production

import (
	"sort"
	"time"
)

type Compatibility string

const (
	CompatibilityCompatible Compatibility = "compatible"
	CompatibilityCaveat     Compatibility = "compatible_with_caveats"
	CompatibilityInvalid    Compatibility = "incompatible"
	CompatibilityUnknown    Compatibility = "insufficient_data"
)

type OptimizationStrategy string

const (
	StrategyFIFO     OptimizationStrategy = "fifo"
	StrategyShortest OptimizationStrategy = "shortest_processing_time"
	StrategyDueDate  OptimizationStrategy = "earliest_due_date"
	StrategyMaterial OptimizationStrategy = "material_color_grouping"
	StrategyCombined OptimizationStrategy = "combined"
)

func ValidStrategy(value OptimizationStrategy) bool {
	return value == StrategyFIFO || value == StrategyShortest || value == StrategyDueDate || value == StrategyMaterial || value == StrategyCombined
}

type OptimizationJob struct {
	ID, Number, ProductName, MaterialID, MaterialName, ColorID, ColorName string
	Priority, Quantity                                                    int
	DueAt                                                                 *time.Time
	CreatedAt                                                             time.Time
	WidthMM, DepthMM, HeightMM, EstimatedMinutes                          *int
	PrintTempC, BedTempC                                                  *int
	RequiresEnclosure                                                     *bool
	RequiredSurface                                                       string
	MaterialSpecsVerified                                                 bool
	Confidence                                                            ForecastConfidence
}

type OptimizationPrinter struct {
	ID, Name, Model, Status, CurrentMaterialID, CurrentColorID string
	BuildWidthMM, BuildDepthMM, BuildHeightMM, KnownMinutes    int
	MaxHotendTempC, MaxBedTempC, InstalledNozzleMicrons        *int
	EnclosedChamber                                            *bool
	InstalledNozzleType, InstalledSurface                      string
	TechnicalSpecsVerified, OperationalConfigReady             bool
	SupportedMaterialIDs                                       map[string]bool
	SupportedNozzles                                           map[int]bool
}

type CompatibilityResult struct {
	Status      Compatibility `json:"status"`
	Reasons     []RiskReason  `json:"reasons"`
	MissingData []string      `json:"missing_data"`
}

func EvaluateCompatibility(job OptimizationJob, printer OptimizationPrinter) CompatibilityResult {
	if IsPrinterUnavailable(printer.Status) {
		return CompatibilityResult{Status: CompatibilityInvalid, Reasons: []RiskReason{{Code: "printer_unavailable", Message: "A impressora não está operacionalmente disponível."}}}
	}
	if job.WidthMM == nil || job.DepthMM == nil || job.HeightMM == nil {
		return CompatibilityResult{Status: CompatibilityUnknown, Reasons: []RiskReason{{Code: "missing_dimensions", Message: "As dimensões da peça não estão completas."}}, MissingData: []string{"job_dimensions", "printer_material_capabilities"}}
	}
	if *job.WidthMM > printer.BuildWidthMM || *job.DepthMM > printer.BuildDepthMM || *job.HeightMM > printer.BuildHeightMM {
		return CompatibilityResult{Status: CompatibilityInvalid, Reasons: []RiskReason{{Code: "build_volume_exceeded", Message: "A peça excede o volume útil cadastrado da impressora."}}}
	}
	result := CompatibilityResult{Status: CompatibilityCompatible, Reasons: []RiskReason{{Code: "dimensions_fit", Message: "A peça cabe no volume útil cadastrado."}}, MissingData: []string{}}
	if !printer.TechnicalSpecsVerified || !printer.OperationalConfigReady || !job.MaterialSpecsVerified {
		return CompatibilityResult{Status: CompatibilityUnknown, Reasons: append(result.Reasons, RiskReason{Code: "technical_configuration_unknown", Message: "Especificações verificadas e configuração operacional são necessárias."}), MissingData: []string{"verified_printer_specs", "operational_configuration", "verified_material_specs"}}
	}
	if !printer.SupportedMaterialIDs[job.MaterialID] {
		return CompatibilityResult{Status: CompatibilityInvalid, Reasons: append(result.Reasons, RiskReason{Code: "material_not_supported", Message: "O material não consta entre os materiais suportados nesta especificação."})}
	}
	if printer.InstalledNozzleMicrons == nil || !printer.SupportedNozzles[*printer.InstalledNozzleMicrons] {
		return CompatibilityResult{Status: CompatibilityInvalid, Reasons: append(result.Reasons, RiskReason{Code: "nozzle_not_supported", Message: "O bico instalado não pertence às configurações suportadas."})}
	}
	if job.PrintTempC == nil || printer.MaxHotendTempC == nil || job.BedTempC == nil || printer.MaxBedTempC == nil {
		return CompatibilityResult{Status: CompatibilityUnknown, Reasons: append(result.Reasons, RiskReason{Code: "temperature_unknown", Message: "Faltam temperaturas recomendadas ou limites documentados."}), MissingData: []string{"temperature_limits"}}
	}
	if *job.PrintTempC > *printer.MaxHotendTempC || *job.BedTempC > *printer.MaxBedTempC {
		return CompatibilityResult{Status: CompatibilityInvalid, Reasons: append(result.Reasons, RiskReason{Code: "temperature_limit_exceeded", Message: "A temperatura recomendada excede o limite documentado do equipamento."})}
	}
	if job.RequiresEnclosure != nil && *job.RequiresEnclosure && (printer.EnclosedChamber == nil || !*printer.EnclosedChamber) {
		return CompatibilityResult{Status: CompatibilityInvalid, Reasons: append(result.Reasons, RiskReason{Code: "enclosure_required", Message: "O material exige ambiente fechado."})}
	}
	if job.RequiredSurface != "" && printer.InstalledSurface != job.RequiredSurface {
		result.Status = CompatibilityCaveat
		result.Reasons = append(result.Reasons, RiskReason{Code: "surface_change_required", Message: "A superfície instalada não corresponde à recomendada."})
	}
	result.Reasons = append(result.Reasons, RiskReason{Code: "technical_compatibility_confirmed", Message: "Limites verificados, material e bico são compatíveis."})
	return result
}

type PrinterRecommendation struct {
	PrinterID     string             `json:"printer_id"`
	PrinterName   string             `json:"printer_name"`
	Score         int                `json:"score"`
	Compatibility Compatibility      `json:"compatibility"`
	Confidence    ForecastConfidence `json:"confidence"`
	Reasons       []RiskReason       `json:"reasons"`
	MissingData   []string           `json:"missing_data"`
}

func RankPrinters(job OptimizationJob, printers []OptimizationPrinter) []PrinterRecommendation {
	result := make([]PrinterRecommendation, 0, len(printers))
	for _, printer := range printers {
		compatibility := EvaluateCompatibility(job, printer)
		score := 0
		switch compatibility.Status {
		case CompatibilityCompatible:
			score = 50
		case CompatibilityCaveat:
			score = 35
		case CompatibilityUnknown:
			score = 20
		default:
			result = append(result, PrinterRecommendation{PrinterID: printer.ID, PrinterName: printer.Name, Compatibility: compatibility.Status, Reasons: compatibility.Reasons, MissingData: compatibility.MissingData, Confidence: ConfidenceUnknown})
			continue
		}
		if printer.Status == string(PrinterAvailable) {
			score += 20
		} else if printer.Status == string(PrinterBusy) {
			score += 5
		}
		loadScore := 20 - printer.KnownMinutes/60
		if loadScore < 0 {
			loadScore = 0
		}
		score += loadScore
		if printer.CurrentMaterialID != "" && printer.CurrentMaterialID == job.MaterialID && printer.CurrentColorID == job.ColorID {
			score += 5
			compatibility.Reasons = append(compatibility.Reasons, RiskReason{Code: "same_material_color", Message: "A carga ativa usa o mesmo material e cor."})
		}
		switch job.Confidence {
		case ConfidenceHigh:
			score += 10
		case ConfidenceMedium:
			score += 7
		case ConfidenceLow:
			score += 4
		}
		result = append(result, PrinterRecommendation{PrinterID: printer.ID, PrinterName: printer.Name, Score: score, Compatibility: compatibility.Status, Confidence: job.Confidence, Reasons: compatibility.Reasons, MissingData: compatibility.MissingData})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].PrinterID < result[j].PrinterID
		}
		return result[i].Score > result[j].Score
	})
	return result
}

func SequenceJobs(jobs []OptimizationJob, strategy OptimizationStrategy) []OptimizationJob {
	ordered := append([]OptimizationJob(nil), jobs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		switch strategy {
		case StrategyShortest:
			if a.EstimatedMinutes != nil && b.EstimatedMinutes != nil && *a.EstimatedMinutes != *b.EstimatedMinutes {
				return *a.EstimatedMinutes < *b.EstimatedMinutes
			}
		case StrategyDueDate:
			if a.DueAt != nil && b.DueAt != nil && !a.DueAt.Equal(*b.DueAt) {
				return a.DueAt.Before(*b.DueAt)
			}
			if a.DueAt != nil && b.DueAt == nil {
				return true
			}
		case StrategyMaterial:
			if a.MaterialID != b.MaterialID {
				return a.MaterialID < b.MaterialID
			}
			if a.ColorID != b.ColorID {
				return a.ColorID < b.ColorID
			}
		case StrategyCombined:
			if a.DueAt != nil && b.DueAt != nil && !a.DueAt.Equal(*b.DueAt) {
				return a.DueAt.Before(*b.DueAt)
			}
			if a.Priority != b.Priority {
				return a.Priority > b.Priority
			}
			if a.MaterialID != b.MaterialID {
				return a.MaterialID < b.MaterialID
			}
			if a.ColorID != b.ColorID {
				return a.ColorID < b.ColorID
			}
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})
	return ordered
}

type ScheduledJob struct {
	JobID            string        `json:"job_id"`
	JobNumber        string        `json:"job_number"`
	PrinterID        string        `json:"printer_id"`
	PrinterName      string        `json:"printer_name"`
	Position         int           `json:"position"`
	StartsAt         *time.Time    `json:"starts_at"`
	EndsAt           *time.Time    `json:"ends_at"`
	EstimatedMinutes *int          `json:"estimated_minutes"`
	Compatibility    Compatibility `json:"compatibility"`
}
type Simulation struct {
	Strategy              OptimizationStrategy `json:"strategy"`
	Schedule              []ScheduledJob       `json:"schedule"`
	TotalEstimatedMinutes *int                 `json:"total_estimated_minutes"`
	PrinterUtilization    map[string]float64   `json:"printer_utilization"`
	PotentiallyLateJobs   int                  `json:"potentially_late_jobs"`
	MaterialChanges       int                  `json:"material_changes"`
	UnassignedJobs        int                  `json:"unassigned_jobs"`
	MissingEstimateJobs   int                  `json:"missing_estimate_jobs"`
	Confidence            ForecastConfidence   `json:"confidence"`
	ResourceWarnings      []string             `json:"resource_warnings"`
}

func Simulate(now time.Time, jobs []OptimizationJob, printers []OptimizationPrinter, strategy OptimizationStrategy) Simulation {
	result := Simulation{Strategy: strategy, Schedule: []ScheduledJob{}, PrinterUtilization: map[string]float64{}, ResourceWarnings: []string{}}
	if !ValidStrategy(strategy) {
		return result
	}
	type state struct {
		printer         OptimizationPrinter
		free            time.Time
		minutes         int
		material, color string
	}
	states := make([]state, 0, len(printers))
	for _, p := range printers {
		if !IsPrinterUnavailable(p.Status) {
			states = append(states, state{printer: p, free: now.Add(time.Duration(p.KnownMinutes) * time.Minute), minutes: p.KnownMinutes, material: p.CurrentMaterialID, color: p.CurrentColorID})
		}
	}
	if len(states) == 0 {
		result.UnassignedJobs = len(jobs)
		result.ResourceWarnings = append(result.ResourceWarnings, "Nenhuma impressora operacional disponível.")
		return result
	}
	known := 0
	confidence := ConfidenceHigh
	hasEstimate := false
	for position, job := range SequenceJobs(jobs, strategy) {
		ranked := RankPrinters(job, printers)
		chosen := -1
		for _, rec := range ranked {
			if rec.Compatibility == CompatibilityInvalid {
				continue
			}
			for i := range states {
				if states[i].printer.ID == rec.PrinterID && (chosen < 0 || states[i].free.Before(states[chosen].free)) {
					chosen = i
				}
			}
		}
		if chosen < 0 {
			result.UnassignedJobs++
			continue
		}
		compat := EvaluateCompatibility(job, states[chosen].printer)
		item := ScheduledJob{JobID: job.ID, JobNumber: job.Number, PrinterID: states[chosen].printer.ID, PrinterName: states[chosen].printer.Name, Position: position + 1, EstimatedMinutes: job.EstimatedMinutes, Compatibility: compat.Status}
		if job.EstimatedMinutes == nil {
			result.MissingEstimateJobs++
			confidence = ConfidenceUnknown
			result.Schedule = append(result.Schedule, item)
			continue
		}
		hasEstimate = true
		if confidenceRank(job.Confidence) < confidenceRank(confidence) {
			confidence = job.Confidence
		}
		start := states[chosen].free
		end := start.Add(time.Duration(*job.EstimatedMinutes) * time.Minute)
		item.StartsAt = &start
		item.EndsAt = &end
		states[chosen].free = end
		states[chosen].minutes += *job.EstimatedMinutes
		known += *job.EstimatedMinutes
		if job.DueAt != nil && end.After(*job.DueAt) {
			result.PotentiallyLateJobs++
		}
		if states[chosen].material != "" && (states[chosen].material != job.MaterialID || states[chosen].color != job.ColorID) {
			result.MaterialChanges++
		}
		states[chosen].material = job.MaterialID
		states[chosen].color = job.ColorID
		result.Schedule = append(result.Schedule, item)
	}
	if hasEstimate {
		result.TotalEstimatedMinutes = &known
	}
	horizon := 0
	for _, s := range states {
		if s.minutes > horizon {
			horizon = s.minutes
		}
	}
	if horizon > 0 {
		for _, s := range states {
			result.PrinterUtilization[s.printer.ID] = float64(s.minutes) / float64(horizon)
		}
	}
	if !hasEstimate {
		confidence = ConfidenceUnknown
	}
	result.Confidence = confidence
	if result.MissingEstimateJobs > 0 {
		result.ResourceWarnings = append(result.ResourceWarnings, "Existem jobs sem estimativa de duração.")
	}
	return result
}

func confidenceRank(value ForecastConfidence) int {
	switch value {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}
