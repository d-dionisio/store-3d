package production

import (
	"errors"
	"testing"
	"time"
)

func TestProductionJobTransitions(t *testing.T) {
	job, err := NewProductionJob("job", "PJ-1", "item", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []JobStatus{JobQueued, JobPreparing, JobPrinting, JobPostProcessing, JobQualityCheck, JobCompleted} {
		if err = job.Transition(status); err != nil {
			t.Fatalf("transition to %s: %v", status, err)
		}
	}
	if job.Version != 7 {
		t.Fatalf("version=%d", job.Version)
	}
	if err = job.Transition(JobQueued); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid terminal transition, got %v", err)
	}
}

func TestInvalidProductionQuantity(t *testing.T) {
	if _, err := NewProductionJob("job", "PJ", "item", 0); !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("got %v", err)
	}
	if _, err := NewProductionRun("run", "job", 1, -1); !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("got %v", err)
	}
}

func TestFinancialGate(t *testing.T) {
	valid := FinancialSnapshot{OrderStatus: "confirmed", OrderCurrency: "BRL", OrderTotalCents: 1000, PaymentStatus: "paid", PaymentCurrency: "BRL", PaymentAmountCents: 1000, PaidAmountCents: 1000}
	if err := valid.ValidateForQueue(); err != nil {
		t.Fatal(err)
	}
	cases := []FinancialSnapshot{valid, valid, valid, valid}
	cases[0].PaymentStatus = "partially_refunded"
	cases[1].RefundedAmountCents = 1
	cases[2].PaidAmountCents = 999
	cases[3].PaymentCurrency = "USD"
	for i, current := range cases {
		if !errors.Is(current.ValidateForQueue(), ErrFinancialGate) {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestMaterialConsumptionAndSnapshot(t *testing.T) {
	spool := MaterialSpool{CurrentWeightGrams: 1000, InitialWeightGrams: 1000, CostPerKGCents: 12000, Version: 1, Status: "available"}
	usage, err := spool.Consume(100, 25)
	if err != nil {
		t.Fatal(err)
	}
	if spool.CurrentWeightGrams != 875 || spool.Version != 2 || usage.TotalCostCents != 1500 || usage.CostPerKGCents != 12000 {
		t.Fatalf("spool=%+v usage=%+v", spool, usage)
	}
	if _, err = spool.Consume(876, 0); !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("got %v", err)
	}
	if spool.CurrentWeightGrams != 875 {
		t.Fatal("failed consumption mutated balance")
	}
}

func TestNegativeCosts(t *testing.T) {
	run := ProductionRun{MaterialCostCents: -1}
	if !errors.Is(run.ValidateCosts(), ErrInvalidCost) {
		t.Fatal("negative cost accepted")
	}
}

func TestQualityInspection(t *testing.T) {
	falseValue := false
	inspection := QualityInspection{ID: "i", RunID: "r", InspectorAdminID: "a", Result: InspectionApproved, DimensionalAccuracyOK: true, VisualQualityOK: true, FunctionalTestOK: &falseValue, InspectedAt: time.Now()}
	if !errors.Is(inspection.Validate(), ErrInvalidInspection) {
		t.Fatal("invalid approval accepted")
	}
	inspection.FunctionalTestOK = nil
	if err := inspection.Validate(); err != nil {
		t.Fatal(err)
	}
	inspection.Result, inspection.Notes = InspectionRejected, ""
	if !errors.Is(inspection.Validate(), ErrInvalidInspection) {
		t.Fatal("rejection without reason accepted")
	}
}

func TestReprintRules(t *testing.T) {
	previous := ProductionRun{ID: "r1", JobID: "j", AttemptNumber: 1, Quantity: 2, Status: RunCompleted}
	if _, err := NewReprint("r2", previous, InspectionApproved); !errors.Is(err, ErrInvalidReprint) {
		t.Fatalf("got %v", err)
	}
	reprint, err := NewReprint("r2", previous, InspectionRejected)
	if err != nil {
		t.Fatal(err)
	}
	if reprint.AttemptNumber != 2 || reprint.ReprintOfRunID != "r1" {
		t.Fatalf("%+v", reprint)
	}
	previous.Status = RunFailed
	if _, err = NewReprint("r3", previous, ""); err != nil {
		t.Fatal(err)
	}
}

func TestRunRequiresPrinter(t *testing.T) {
	run, _ := NewProductionRun("r", "j", 1, 1)
	if err := run.Transition(RunPreparing); err == nil {
		t.Fatal("run without printer accepted")
	}
	run.PrinterID = "p"
	if err := run.Transition(RunPreparing); err != nil {
		t.Fatal(err)
	}
}
