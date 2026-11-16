package bambulab

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestParseTelemetryPartialAndUnknown(t *testing.T) {
	now := time.Now().UTC()
	value, err := ParseTelemetry([]byte(`{"print":{"gcode_state":"RUNNING","mc_percent":42,"nozzle_temper":215.5}}`), now)
	if err != nil || value.State != "printing" || value.ProgressPercent == nil || value.BedTemperatureC != nil {
		t.Fatalf("value=%+v err=%v", value, err)
	}
	unknown, err := ParseTelemetry([]byte(`{"print":{"future_field":true}}`), now)
	if err != nil || unknown.State != "unknown" {
		t.Fatalf("unknown=%+v err=%v", unknown, err)
	}
}
func TestCredentialCipherDoesNotExposeSecret(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	cipher, err := NewCipher(key, 1)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt([]byte("access-code"))
	if err != nil || strings.Contains(string(encrypted), "access-code") {
		t.Fatal("credential was not safely encrypted")
	}
	plain, err := cipher.Decrypt(encrypted, 1)
	if err != nil || string(plain) != "access-code" {
		t.Fatal("cipher roundtrip failed")
	}
}
