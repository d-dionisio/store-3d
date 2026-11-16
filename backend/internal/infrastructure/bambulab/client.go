package bambulab

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"time"
)

type Client struct{ ConnectTimeout time.Duration }

func NewClient() *Client { return &Client{ConnectTimeout: 8 * time.Second} }
func tlsConfig(fingerprint string) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true, VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errors.New("printer supplied no certificate")
		}
		sum := sha256.Sum256(rawCerts[0])
		want, e := hex.DecodeString(fingerprint)
		if e != nil || len(want) != len(sum) || subtle.ConstantTimeCompare(sum[:], want) != 1 {
			return errors.New("printer certificate fingerprint mismatch")
		}
		return nil
	}}
}
func (c *Client) options(in app.PrinterMonitoringConfigInput, onMessage mqtt.MessageHandler) *mqtt.ClientOptions {
	topic := "device/" + in.SerialNumber + "/report"
	opts := mqtt.NewClientOptions().AddBroker(fmt.Sprintf("tls://%s:%d", in.Host, in.Port)).SetUsername("bblp").SetPassword(in.AccessCode).SetTLSConfig(tlsConfig(in.CertificateSHA256)).SetConnectTimeout(c.ConnectTimeout).SetAutoReconnect(true).SetConnectRetry(true).SetConnectRetryInterval(time.Second).SetMaxReconnectInterval(30 * time.Second).SetOrderMatters(false)
	opts.SetOnConnectHandler(func(client mqtt.Client) { client.Subscribe(topic, 1, onMessage) })
	return opts
}
func (c *Client) TestConnection(ctx context.Context, in app.PrinterMonitoringConfigInput) error {
	client := mqtt.NewClient(c.options(in, func(mqtt.Client, mqtt.Message) {}))
	token := client.Connect()
	done := make(chan struct{})
	go func() { token.Wait(); close(done) }()
	select {
	case <-ctx.Done():
		client.Disconnect(100)
		return ctx.Err()
	case <-done:
		if token.Error() != nil {
			return token.Error()
		}
		client.Disconnect(100)
		return nil
	}
}
func (c *Client) Monitor(ctx context.Context, in app.PrinterMonitoringConfigInput, sink func(domain.PrinterTelemetry) error) error {
	messages := make(chan []byte, 8)
	client := mqtt.NewClient(c.options(in, func(_ mqtt.Client, m mqtt.Message) {
		copyValue := append([]byte(nil), m.Payload()...)
		select {
		case messages <- copyValue:
		default:
		}
	}))
	token := client.Connect()
	if !token.WaitTimeout(c.ConnectTimeout) {
		return context.DeadlineExceeded
	}
	if token.Error() != nil {
		return token.Error()
	}
	defer client.Disconnect(500)
	for {
		select {
		case <-ctx.Done():
			return nil
		case payload := <-messages:
			t, err := ParseTelemetry(payload, time.Now().UTC())
			if err == nil {
				if err = sink(t); err != nil {
					return err
				}
			}
		}
	}
}
