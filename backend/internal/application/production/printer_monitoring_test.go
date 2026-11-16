package production

import "testing"

func TestMonitoringTargetRejectsSSRF(t *testing.T) {
	base := PrinterMonitoringConfigInput{Host: "192.168.1.10", Port: 8883, SerialNumber: "SERIAL123", CertificateSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if !validMonitoringTarget(base) {
		t.Fatal("private target rejected")
	}
	base.Host = "8.8.8.8"
	if validMonitoringTarget(base) {
		t.Fatal("public target accepted")
	}
	base.Host = "example.com"
	if validMonitoringTarget(base) {
		t.Fatal("hostname target accepted")
	}
}
