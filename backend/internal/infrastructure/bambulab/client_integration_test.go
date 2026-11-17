package bambulab

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	mqttserver "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

type testAuthHook struct {
	mqttserver.HookBase
	writes atomic.Int32
}

func (h *testAuthHook) ID() string { return "bambu-test-auth" }
func (h *testAuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{mqttserver.OnConnectAuthenticate, mqttserver.OnACLCheck}, []byte{b})
}
func (h *testAuthHook) OnConnectAuthenticate(_ *mqttserver.Client, pk packets.Packet) bool {
	return string(pk.Connect.Username) == "bblp" && string(pk.Connect.Password) == "test-code"
}
func (h *testAuthHook) OnACLCheck(_ *mqttserver.Client, _ string, write bool) bool {
	if write {
		h.writes.Add(1)
		return false
	}
	return true
}

func testCertificate(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "bambu-test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(der)
	return cert, hex.EncodeToString(sum[:])
}

func startTestBroker(t *testing.T) (*mqttserver.Server, string, string, *testAuthHook) {
	t.Helper()
	cert, fingerprint := testCertificate(t)
	server := mqttserver.New(&mqttserver.Options{InlineClient: true})
	hook := new(testAuthHook)
	if err := server.AddHook(hook, nil); err != nil {
		t.Fatal(err)
	}
	listener := listeners.NewTCP(listeners.Config{ID: "tls", Address: "127.0.0.1:0", TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}})
	if err := server.AddListener(listener); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	return server, listener.Address(), fingerprint, hook
}

func startTestBrokerAt(t *testing.T, cert tls.Certificate, address string) (*mqttserver.Server, *testAuthHook) {
	t.Helper()
	server := mqttserver.New(&mqttserver.Options{InlineClient: true})
	hook := new(testAuthHook)
	if err := server.AddHook(hook, nil); err != nil {
		t.Fatal(err)
	}
	listener := listeners.NewTCP(listeners.Config{ID: "tls", Address: address, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}})
	if err := server.AddListener(listener); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	time.Sleep(30 * time.Millisecond)
	return server, hook
}

func monitoringInput(t *testing.T, address, fingerprint string) app.PrinterMonitoringConfigInput {
	t.Helper()
	_, portText, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return app.PrinterMonitoringConfigInput{Host: "127.0.0.1", Port: port, SerialNumber: "SERIAL_TEST", CertificateSHA256: fingerprint, AccessCode: "test-code", Enabled: true}
}

func TestMQTTSTrustAuthenticationSubscriptionAndReadOnly(t *testing.T) {
	server, address, fingerprint, hook := startTestBroker(t)
	client := NewClient()
	client.ConnectTimeout = time.Second
	input := monitoringInput(t, address, fingerprint)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.TestConnection(ctx, input); err != nil {
		t.Fatalf("valid connection: %v", err)
	}
	wrongFingerprint := input
	wrongFingerprint.CertificateSHA256 = hex.EncodeToString(make([]byte, 32))
	if err := client.TestConnection(ctx, wrongFingerprint); err == nil {
		t.Fatal("different certificate was accepted")
	}
	wrongCredentials := input
	wrongCredentials.AccessCode = "wrong"
	if err := client.TestConnection(ctx, wrongCredentials); err == nil {
		t.Fatal("invalid credentials were accepted")
	}
	monitorCtx, stop := context.WithCancel(context.Background())
	monitorCtx2, stop2 := context.WithCancel(context.Background())
	received := make(chan domain.PrinterTelemetry, 4)
	received2 := make(chan domain.PrinterTelemetry, 1)
	done := make(chan error, 1)
	done2 := make(chan error, 1)
	go func() {
		done <- client.Monitor(monitorCtx, input, func(value domain.PrinterTelemetry) error { received <- value; return nil })
	}()
	input2 := input
	input2.SerialNumber = "SERIAL_TEST_2"
	go func() {
		done2 <- client.Monitor(monitorCtx2, input2, func(value domain.PrinterTelemetry) error { received2 <- value; return nil })
	}()
	topic := "device/" + input.SerialNumber + "/report"
	deadline := time.Now().Add(2 * time.Second)
	for server.Clients.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(250 * time.Millisecond)
	if err := server.Publish(topic, []byte(`{"print":{"gcode_state":"RUNNING","mc_percent":31}}`), false, 1); err != nil {
		t.Fatal(err)
	}
	if err := server.Publish(topic, []byte(`{"print":{"gcode_state":"RUNNING","mc_percent":31}}`), false, 1); err != nil {
		t.Fatal(err)
	}
	if err := server.Publish(topic, []byte(`{"print":{"future_field":true}}`), false, 1); err != nil {
		t.Fatal(err)
	}
	if err := server.Publish("device/"+input2.SerialNumber+"/report", []byte(`{"print":{"gcode_state":"IDLE"}}`), false, 1); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		select {
		case <-received:
		case <-time.After(2 * time.Second):
			t.Fatal("telemetry not delivered")
		}
	}
	select {
	case value := <-received2:
		if value.State != "idle" {
			t.Fatalf("second equipment state=%s", value.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second equipment telemetry not delivered")
	}
	stop()
	stop2()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("monitor did not shut down gracefully")
	}
	select {
	case err := <-done2:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second monitor did not shut down gracefully")
	}
	if hook.writes.Load() != 0 {
		t.Fatalf("client attempted %d operational publish(es)", hook.writes.Load())
	}
}

func TestMQTTSConnectTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, e := listener.Accept()
		if e == nil {
			defer connection.Close()
			time.Sleep(time.Second)
		}
	}()
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	client := NewClient()
	client.ConnectTimeout = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	input := app.PrinterMonitoringConfigInput{Host: "127.0.0.1", Port: port, SerialNumber: "TIMEOUT", CertificateSHA256: hex.EncodeToString(make([]byte, 32)), AccessCode: "test-code"}
	if err = client.TestConnection(ctx, input); err == nil {
		t.Fatal("unresponsive TLS peer did not time out")
	}
}

func TestMQTTSReconnectsAfterNetworkInterruption(t *testing.T) {
	cert, fingerprint := testCertificate(t)
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	reservation.Close()
	server, _ := startTestBrokerAt(t, cert, address)
	input := monitoringInput(t, address, fingerprint)
	client := NewClient()
	client.ConnectTimeout = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	received := make(chan domain.PrinterTelemetry, 2)
	go func() {
		done <- client.Monitor(ctx, input, func(value domain.PrinterTelemetry) error { received <- value; return nil })
	}()
	topic := "device/" + input.SerialNumber + "/report"
	time.Sleep(200 * time.Millisecond)
	if err = server.Publish(topic, []byte(`{"print":{"gcode_state":"IDLE"}}`), false, 1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("initial telemetry not received")
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	replacement, _ := startTestBrokerAt(t, cert, address)
	defer replacement.Close()
	deadline := time.Now().Add(5 * time.Second)
	recovered := false
	for time.Now().Before(deadline) && !recovered {
		_ = replacement.Publish(topic, []byte(`{"print":{"gcode_state":"RUNNING"}}`), false, 1)
		select {
		case value := <-received:
			recovered = value.State == "printing"
		case <-time.After(150 * time.Millisecond):
		}
	}
	if !recovered {
		t.Fatal("monitor did not recover after broker restart")
	}
	cancel()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reconnected monitor did not shut down")
	}
}
