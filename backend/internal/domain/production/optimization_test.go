package production

import (
	"reflect"
	"testing"
	"time"
)

func intPtr(v int) *int { return &v }

func TestCompatibilityDoesNotInventCapabilities(t *testing.T) {
	printer := OptimizationPrinter{ID: "p", Status: "available", BuildWidthMM: 256, BuildDepthMM: 256, BuildHeightMM: 256}
	missing := EvaluateCompatibility(OptimizationJob{}, printer)
	if missing.Status != CompatibilityUnknown {
		t.Fatalf("missing=%s", missing.Status)
	}
	tooLarge := EvaluateCompatibility(OptimizationJob{WidthMM: intPtr(300), DepthMM: intPtr(10), HeightMM: intPtr(10)}, printer)
	if tooLarge.Status != CompatibilityInvalid {
		t.Fatalf("large=%s", tooLarge.Status)
	}
	fit := EvaluateCompatibility(OptimizationJob{WidthMM: intPtr(100), DepthMM: intPtr(100), HeightMM: intPtr(100)}, printer)
	if fit.Status != CompatibilityUnknown || len(fit.MissingData) == 0 {
		t.Fatalf("fit=%+v", fit)
	}
	offline := printer
	offline.Status = "maintenance"
	if got := EvaluateCompatibility(OptimizationJob{WidthMM: intPtr(10), DepthMM: intPtr(10), HeightMM: intPtr(10)}, offline); got.Status != CompatibilityInvalid {
		t.Fatalf("maintenance=%s", got.Status)
	}
}

func TestRankPrintersIsDeterministicAndExplainsTie(t *testing.T) {
	job := OptimizationJob{MaterialID: "m", ColorID: "c", WidthMM: intPtr(10), DepthMM: intPtr(10), HeightMM: intPtr(10), Confidence: ConfidenceHigh}
	printers := []OptimizationPrinter{{ID: "b", Name: "B", Status: "available", BuildWidthMM: 20, BuildDepthMM: 20, BuildHeightMM: 20}, {ID: "a", Name: "A", Status: "available", BuildWidthMM: 20, BuildDepthMM: 20, BuildHeightMM: 20}}
	first := RankPrinters(job, printers)
	second := RankPrinters(job, printers)
	if !reflect.DeepEqual(first, second) || first[0].PrinterID != "a" {
		t.Fatalf("ranking=%+v", first)
	}
}

func TestSequenceStrategiesAndConflictingDueDates(t *testing.T) {
	now := time.Now()
	jobs := []OptimizationJob{{ID: "a", MaterialID: "z", CreatedAt: now, Priority: 1, EstimatedMinutes: intPtr(50)}, {ID: "b", MaterialID: "a", CreatedAt: now.Add(time.Minute), Priority: 90, EstimatedMinutes: intPtr(10), DueAt: timePtr(now.Add(time.Hour))}}
	if got := SequenceJobs(jobs, StrategyFIFO); got[0].ID != "a" {
		t.Fatal("fifo")
	}
	if got := SequenceJobs(jobs, StrategyShortest); got[0].ID != "b" {
		t.Fatal("shortest")
	}
	if got := SequenceJobs(jobs, StrategyDueDate); got[0].ID != "b" {
		t.Fatal("due")
	}
	if got := SequenceJobs(jobs, StrategyMaterial); got[0].ID != "b" {
		t.Fatal("material")
	}
	if got := SequenceJobs(jobs, StrategyCombined); got[0].ID != "b" {
		t.Fatal("combined")
	}
}

func TestSimulationCapacityMissingHistoryAndNoMutation(t *testing.T) {
	now := time.Now().UTC()
	job := OptimizationJob{ID: "j", Number: "J", MaterialID: "m", ColorID: "c", CreatedAt: now, WidthMM: intPtr(10), DepthMM: intPtr(10), HeightMM: intPtr(10)}
	original := job
	none := Simulate(now, []OptimizationJob{job}, nil, StrategyFIFO)
	if none.UnassignedJobs != 1 {
		t.Fatalf("none=%+v", none)
	}
	printer := OptimizationPrinter{ID: "p", Name: "P", Status: "available", BuildWidthMM: 20, BuildDepthMM: 20, BuildHeightMM: 20}
	result := Simulate(now, []OptimizationJob{job}, []OptimizationPrinter{printer}, StrategyFIFO)
	if result.MissingEstimateJobs != 1 || result.Confidence != ConfidenceUnknown || !reflect.DeepEqual(job, original) {
		t.Fatalf("result=%+v mutated=%v", result, !reflect.DeepEqual(job, original))
	}
}

func timePtr(v time.Time) *time.Time { return &v }
