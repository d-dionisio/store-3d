package production

import (
	"testing"
	"time"
)

func TestTelemetryFreshnessAndAvailability(t *testing.T) {
	now := time.Now()
	seen := now.Add(-10 * time.Second)
	v := PrinterTelemetry{Connection: ConnectionOnline, State: "printing", LastSeenAt: &seen}
	if !v.Fresh(now, 45*time.Second) || v.EffectiveAvailability(now, 45*time.Second) != "busy" {
		t.Fatal("fresh print should be busy")
	}
	if v.Fresh(now.Add(time.Minute), 45*time.Second) || v.EffectiveAvailability(now.Add(time.Minute), 45*time.Second) != "unknown" {
		t.Fatal("stale telemetry must be unknown")
	}
}
