package production

import (
	"testing"
	"time"
)

func TestPlanningMedianConfidenceAndRemaining(t *testing.T) {
	if got, ok := MedianMinutes([]int{40, 10, 30, 20}); !ok || got != 25 {
		t.Fatalf("median=%d ok=%v", got, ok)
	}
	if Confidence(2) != ConfidenceLow || Confidence(5) != ConfidenceMedium || Confidence(10) != ConfidenceHigh {
		t.Fatal("unexpected confidence")
	}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	started := now.Add(-20 * time.Minute)
	if got := RemainingMinutes(60, &started, now); got != 40 {
		t.Fatalf("remaining=%d", got)
	}
}

func TestPlanningRiskPrecedenceAndMissingData(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	overdue := now.Add(-time.Minute)
	if risk, _ := ClassifyRisk(now, &overdue, nil, true, true); risk != RiskOverdue {
		t.Fatalf("risk=%s", risk)
	}
	if risk, _ := ClassifyRisk(now, nil, nil, false, true); risk != RiskAttention {
		t.Fatalf("risk=%s", risk)
	}
	if risk, _ := ClassifyRisk(now, nil, nil, true, false); risk != RiskBlocked {
		t.Fatalf("risk=%s", risk)
	}
}

func TestPlanningMaterialAndPrinter(t *testing.T) {
	req := MaterialRequirement{KnownGrams: 900}
	stock := MaterialAvailability{UsableGrams: 1000, MinimumStockGrams: 200}
	if !IsMaterialInsufficient(req, stock) {
		t.Fatal("expected insufficient material")
	}
	if !IsPrinterUnavailable("maintenance") || IsPrinterUnavailable("busy") {
		t.Fatal("unexpected printer availability")
	}
}
