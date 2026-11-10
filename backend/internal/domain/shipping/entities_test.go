package shipping

import "testing"

func TestNormalizePostalCode(t *testing.T) {
	v, e := NormalizePostalCode("01001-000")
	if e != nil || v != "01001000" {
		t.Fatalf("%q %v", v, e)
	}
	if _, e = NormalizePostalCode("123"); e == nil {
		t.Fatal("expected invalid CEP")
	}
}
func TestPackageValidation(t *testing.T) {
	if (Package{Quantity: 1, WeightGrams: 1, LengthMM: 10, WidthMM: 10, HeightMM: 10}).Validate() != nil {
		t.Fatal("valid package rejected")
	}
	if (Package{Quantity: 1, WeightGrams: 0, LengthMM: 10, WidthMM: 10, HeightMM: 10}).Validate() == nil {
		t.Fatal("invalid package accepted")
	}
}
