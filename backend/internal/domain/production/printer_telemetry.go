package production

import "time"

type ConnectionState string
type ObservedPrinterState string

const (
	ConnectionUnknown    ConnectionState = "unknown"
	ConnectionConnecting ConnectionState = "connecting"
	ConnectionOnline     ConnectionState = "online"
	ConnectionOffline    ConnectionState = "offline"
	ConnectionError      ConnectionState = "error"
)

type PrinterTelemetry struct {
	PrinterID          string               `json:"printer_id"`
	Connection         ConnectionState      `json:"connection_state"`
	State              ObservedPrinterState `json:"printer_state"`
	ProgressPercent    *float64             `json:"progress_percent"`
	NozzleTemperatureC *float64             `json:"nozzle_temperature_c"`
	BedTemperatureC    *float64             `json:"bed_temperature_c"`
	SpeedLevel         *int                 `json:"speed_level"`
	ObservedJobName    string               `json:"observed_job_name"`
	ElapsedSeconds     *int64               `json:"elapsed_seconds"`
	ErrorCode          string               `json:"error_code"`
	LastSeenAt         *time.Time           `json:"last_seen_at"`
	PayloadObservedAt  *time.Time           `json:"payload_observed_at"`
	OperationalStatus  string               `json:"operational_status"`
}

func (t PrinterTelemetry) Fresh(now time.Time, maximumAge time.Duration) bool {
	return t.Connection == ConnectionOnline && t.LastSeenAt != nil && now.Sub(*t.LastSeenAt) <= maximumAge
}

func (t PrinterTelemetry) EffectiveAvailability(now time.Time, maximumAge time.Duration) string {
	if t.Connection == ConnectionOffline || t.Connection == ConnectionError {
		return "unavailable"
	}
	if !t.Fresh(now, maximumAge) {
		return "unknown"
	}
	if t.State == "printing" || t.State == "heating" || t.State == "paused" || t.State == "finished" {
		return "busy"
	}
	if t.State == "idle" && t.OperationalStatus == "available" {
		return "available"
	}
	if t.State == "failed" {
		return "unavailable"
	}
	return "unknown"
}
