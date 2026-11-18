package production

import "math"

type TimeComparison struct {
	EstimatedMinutes  int      `json:"estimated_minutes"`
	ActualMinutes     int      `json:"actual_minutes"`
	DifferenceMinutes int      `json:"difference_minutes"`
	DifferencePercent *float64 `json:"difference_percent"`
	Source            string   `json:"source"`
}

func CompareRecordedTime(estimated, actual int) (TimeComparison, bool) {
	if estimated <= 0 || actual < 0 {
		return TimeComparison{}, false
	}
	difference := actual - estimated
	percent := math.Round((float64(difference)/float64(estimated))*10000) / 100
	return TimeComparison{EstimatedMinutes: estimated, ActualMinutes: actual, DifferenceMinutes: difference, DifferencePercent: &percent, Source: "manual_run"}, true
}

func MaterialSnapshotCost(usedGrams int, costPerKGCents int64) (int64, bool) {
	if usedGrams < 0 || costPerKGCents < 0 {
		return 0, false
	}
	return int64(math.Round(float64(usedGrams) * float64(costPerKGCents) / 1000)), true
}

func Percentage(part, total int64) *float64 {
	if total <= 0 {
		return nil
	}
	value := math.Round(float64(part)*10000/float64(total)) / 100
	return &value
}
