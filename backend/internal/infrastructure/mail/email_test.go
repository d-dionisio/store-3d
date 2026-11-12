package mail

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTemplatesEscapeDynamicContent(t *testing.T) {
	rendered, e := Render("quote_proposal", TemplateData{Reference: `<script>alert(1)</script>`, Link: "https://example.test/token?a=1&b=2"})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(rendered.HTML, "<script>") || !strings.Contains(rendered.HTML, "&lt;script&gt;") {
		t.Fatal("dynamic content was not escaped")
	}
	if rendered.Text == "" || rendered.HTML == "" {
		t.Fatal("missing multipart content")
	}
}
func TestResendSenderAndIdempotency(t *testing.T) {
	var key, auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("Idempotency-Key")
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
	}))
	defer server.Close()
	sender := ResendSender{BaseURL: server.URL, APIKey: "test-key", Client: server.Client()}
	id, e := sender.Send(context.Background(), Message{From: "Lume <a@example.com>", To: "b@example.com", Subject: "Test", HTML: "<p>x</p>", Text: "x", IdempotencyKey: "message/1"})
	if e != nil || id != "email_123" || key != "message/1" || auth != "Bearer test-key" {
		t.Fatalf("id=%s key=%s auth=%s err=%v", id, key, auth, e)
	}
}
func TestOutboxCipherRejectsCorruption(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	cipher, e := NewOutboxCipher(key, 1)
	if e != nil {
		t.Fatal(e)
	}
	value, e := cipher.encrypt([]byte("sensitive-link"))
	if e != nil {
		t.Fatal(e)
	}
	value[len(value)-1] ^= 1
	if _, e = cipher.decrypt(value, 1); e == nil {
		t.Fatal("corrupted payload accepted")
	}
}
func TestVerifyWebhook(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	secret := "whsec_" + base64.StdEncoding.EncodeToString(key)
	id, stamp, payload := "evt_1", time.Now().Unix(), []byte(`{"type":"email.delivered"}`)
	timestamp := time.Unix(stamp, 0).Format("150405")
	_ = timestamp
	stampText := string([]byte{})
	stampText = fmtInt(stamp)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(id + "." + stampText + "." + string(payload)))
	signature := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !VerifyWebhook(secret, id, stampText, signature, payload, time.Now()) {
		t.Fatal("valid webhook rejected")
	}
	payload[0] ^= 1
	if VerifyWebhook(secret, id, stampText, signature, payload, time.Now()) {
		t.Fatal("altered webhook accepted")
	}
}
func fmtInt(value int64) string { return strconv.FormatInt(value, 10) }
