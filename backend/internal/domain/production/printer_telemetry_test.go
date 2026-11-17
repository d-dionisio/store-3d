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

func TestTelemetryAvailabilityRequiresOperatorRelease(t *testing.T) {
	now := time.Now()
	seen := now.Add(-time.Second)
	cases := []struct {
		state             ObservedPrinterState
		operational, want string
	}{
		{"idle", "available", "available"}, {"idle", "maintenance", "unknown"}, {"finished", "available", "busy"}, {"failed", "available", "unavailable"},
	}
	for _, tc := range cases {
		value := PrinterTelemetry{Connection: ConnectionOnline, State: tc.state, OperationalStatus: tc.operational, LastSeenAt: &seen}
		if got := value.EffectiveAvailability(now, 45*time.Second); got != tc.want {
			t.Errorf("state %s operational %s: got %s want %s", tc.state, tc.operational, got, tc.want)
		}
	}
	offline := PrinterTelemetry{Connection: ConnectionOffline, State: "idle", OperationalStatus: "available", LastSeenAt: &seen}
	if got := offline.EffectiveAvailability(now, 45*time.Second); got != "unavailable" {
		t.Fatalf("offline got %s", got)
	}
}
