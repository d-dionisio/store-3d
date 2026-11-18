package production

import "testing"

func TestCompareRecordedTime(t *testing.T) {
	value, ok := CompareRecordedTime(100, 125)
	if !ok || value.DifferenceMinutes != 25 || value.DifferencePercent == nil || *value.DifferencePercent != 25 {
		t.Fatalf("unexpected comparison: %+v", value)
	}
	if _, ok = CompareRecordedTime(0, 10); ok {
		t.Fatal("missing estimate must not create comparison")
	}
}
func TestMaterialSnapshotCost(t *testing.T) {
	value, ok := MaterialSnapshotCost(250, 8000)
	if !ok || value != 2000 {
		t.Fatalf("cost=%d", value)
	}
	if _, ok = MaterialSnapshotCost(-1, 8000); ok {
		t.Fatal("negative consumption accepted")
	}
}
func TestPercentage(t *testing.T) {
	if Percentage(1, 0) != nil {
		t.Fatal("zero total must be unknown")
	}
	value := Percentage(1, 3)
	if value == nil || *value != 33.33 {
		t.Fatalf("percentage=%v", value)
	}
}
