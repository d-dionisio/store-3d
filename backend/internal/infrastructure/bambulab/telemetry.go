package bambulab

import (
	"encoding/json"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"strconv"
	"strings"
	"time"
)

type reportEnvelope struct {
	Print struct {
		GCodeState *string  `json:"gcode_state"`
		Percent    *float64 `json:"mc_percent"`
		Nozzle     *float64 `json:"nozzle_temper"`
		Bed        *float64 `json:"bed_temper"`
		Speed      *int     `json:"spd_lvl"`
		Job        *string  `json:"subtask_name"`
		Remaining  *int64   `json:"mc_remaining_time"`
		Error      *int64   `json:"print_error"`
	} `json:"print"`
}

func ParseTelemetry(payload []byte, now time.Time) (domain.PrinterTelemetry, error) {
	var r reportEnvelope
	if err := json.Unmarshal(payload, &r); err != nil {
		return domain.PrinterTelemetry{}, err
	}
	t := domain.PrinterTelemetry{Connection: domain.ConnectionOnline, State: "unknown", ProgressPercent: r.Print.Percent, NozzleTemperatureC: r.Print.Nozzle, BedTemperatureC: r.Print.Bed, SpeedLevel: r.Print.Speed, LastSeenAt: &now, PayloadObservedAt: &now}
	if r.Print.Job != nil {
		t.ObservedJobName = *r.Print.Job
	}
	if r.Print.Error != nil && *r.Print.Error != 0 {
		t.ErrorCode = strconv.FormatInt(*r.Print.Error, 10)
	}
	if r.Print.GCodeState != nil {
		switch strings.ToUpper(*r.Print.GCodeState) {
		case "IDLE":
			t.State = "idle"
		case "PREPARE":
			t.State = "heating"
		case "RUNNING":
			t.State = "printing"
		case "PAUSE":
			t.State = "paused"
		case "FINISH":
			t.State = "finished"
		case "FAILED":
			t.State = "failed"
		}
	}
	return t, nil
}
