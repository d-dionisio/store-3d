package production

import "testing"

func TestTechnicalSpecificationValidation(t *testing.T) {
	w, d, h := 100, 100, 100
	valid := PrinterTechnicalSpec{PrinterID: "p", Technology: "fdm", MaxWidthMM: &w, MaxDepthMM: &d, MaxHeightMM: &h, SupportedNozzles: []NozzleOption{{DiameterMicrons: 400, Type: "hardened_steel"}}}
	if valid.Validate() != nil {
		t.Fatal("valid printer spec rejected")
	}
	invalid := valid
	invalid.MaxHeightMM = nil
	if invalid.Validate() == nil {
		t.Fatal("partial volume accepted")
	}
	min, max, rec := 220, 200, 210
	if (MaterialTechnicalSpec{MaterialID: "m", PrintTempMinC: &min, PrintTempMaxC: &max, PrintTempRecommendedC: &rec}).Validate() == nil {
		t.Fatal("invalid temperature range accepted")
	}
}
func TestVerifiedTechnicalCompatibility(t *testing.T) {
	ten, hotend, bed, nozzle, temp, bedTemp := 10, 300, 110, 400, 220, 60
	enclosure := false
	job := OptimizationJob{MaterialID: "pla", WidthMM: &ten, DepthMM: &ten, HeightMM: &ten, PrintTempC: &temp, BedTempC: &bedTemp, RequiresEnclosure: &enclosure, MaterialSpecsVerified: true}
	printer := OptimizationPrinter{ID: "p", Status: "available", BuildWidthMM: 100, BuildDepthMM: 100, BuildHeightMM: 100, MaxHotendTempC: &hotend, MaxBedTempC: &bed, InstalledNozzleMicrons: &nozzle, TechnicalSpecsVerified: true, OperationalConfigReady: true, SupportedMaterialIDs: map[string]bool{"pla": true}, SupportedNozzles: map[int]bool{400: true}}
	if got := EvaluateCompatibility(job, printer); got.Status != CompatibilityCompatible {
		t.Fatalf("got=%+v", got)
	}
	tooHot := 350
	job.PrintTempC = &tooHot
	if got := EvaluateCompatibility(job, printer); got.Status != CompatibilityInvalid {
		t.Fatalf("temperature got=%+v", got)
	}
}
