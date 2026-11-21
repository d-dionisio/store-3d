package mail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseOutboxKeyring(t *testing.T) {
	key1 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	key2 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	cipher, err := ParseOutboxKeyring("2", key2, "1:"+key1)
	if err != nil || cipher.Version != 2 {
		t.Fatal("versioned keyring rejected")
	}
	for _, tc := range []struct{ version, active, old string }{
		{"0", key1, ""}, {"x", key1, ""}, {"1", "bad", ""}, {"1", key1, "broken"}, {"1", key1, "1:" + key1},
	} {
		if _, err := ParseOutboxKeyring(tc.version, tc.active, tc.old); err == nil {
			t.Fatal("invalid keyring accepted")
		}
	}
}

type workerOutboxFake struct {
	items                                        []Item
	message                                      Message
	messageErr                                   error
	cancel                                       context.CancelFunc
	claimErr, expireErr, suppressErr, cleanupErr error
	failedErr, expiredErr, sentErr               error
	claimed                                      int
	failed                                       []string
	expired                                      int
	sent                                         int
}

func (f *workerOutboxFake) Claim(context.Context, int) ([]Item, error) {
	f.claimed++
	items := f.items
	f.items = nil
	if f.cancel != nil {
		f.cancel()
	}
	return items, f.claimErr
}
func (f *workerOutboxFake) Cleanup(context.Context, time.Duration, time.Duration) (int64, int64, error) {
	return 0, 0, f.cleanupErr
}
func (f *workerOutboxFake) ExpireDue(context.Context) (int64, error) { return 0, f.expireErr }
func (f *workerOutboxFake) Expired(context.Context, string) error    { f.expired++; return f.expiredErr }
func (f *workerOutboxFake) Failed(_ context.Context, _ Item, code string, _ bool) error {
	f.failed = append(f.failed, code)
	return f.failedErr
}
func (f *workerOutboxFake) Message(Item, string, string) (Message, error) {
	return f.message, f.messageErr
}
func (f *workerOutboxFake) Sent(context.Context, string, string) error     { f.sent++; return f.sentErr }
func (f *workerOutboxFake) SuppressPending(context.Context) (int64, error) { return 0, f.suppressErr }

type workerSenderFake struct {
	err    error
	sends  int
	cancel context.CancelFunc
}

func (f *workerSenderFake) Send(context.Context, Message) (string, error) {
	f.sends++
	if f.cancel != nil {
		f.cancel()
	}
	return "provider-id", f.err
}

func runWorkerOnce(t *testing.T, outbox *workerOutboxFake, sender *workerSenderFake) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	outbox.cancel, sender.cancel = cancel, cancel
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	worker := Worker{Outbox: outbox, Sender: sender, From: "studio@example.test", Batch: 1, Rate: time.Millisecond, Logger: logger}
	if err := worker.Run(ctx); err != nil {
		t.Fatal("worker returned error")
	}
	return logs.String()
}

func TestWorkerProcessesMessageAndDoesNotSendExpiredOrUndecryptable(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		outbox := &workerOutboxFake{items: []Item{{ID: "one", Kind: "verify_email", Attempts: 1, ExpiresAt: time.Now().Add(time.Minute)}}}
		sender := &workerSenderFake{}
		logs := runWorkerOnce(t, outbox, sender)
		if sender.sends != 1 || outbox.sent != 1 || !strings.Contains(logs, "email outbox claim completed") || !strings.Contains(logs, `"count":1`) {
			t.Fatal("valid message was not processed")
		}
	})
	t.Run("expired", func(t *testing.T) {
		outbox := &workerOutboxFake{items: []Item{{ID: "expired", Kind: "verify_email", ExpiresAt: time.Now().Add(-time.Second)}}}
		sender := &workerSenderFake{}
		runWorkerOnce(t, outbox, sender)
		if sender.sends != 0 || outbox.expired != 1 {
			t.Fatal("expired message reached sender")
		}
	})
	t.Run("bad decryption or render", func(t *testing.T) {
		outbox := &workerOutboxFake{items: []Item{{ID: "bad", Kind: "verify_email", KeyVersion: 99, ExpiresAt: time.Now().Add(time.Minute)}}, messageErr: errors.New("secret-bearing test error")}
		sender := &workerSenderFake{}
		logs := runWorkerOnce(t, outbox, sender)
		if sender.sends != 0 || len(outbox.failed) != 1 || outbox.failed[0] != "decrypt_or_render" || strings.Contains(logs, "secret-bearing") {
			t.Fatal("invalid payload handling or log redaction failed")
		}
	})
}

func TestWorkerClassifiesProviderFailuresAndLogsPersistenceFailures(t *testing.T) {
	for _, permanent := range []bool{false, true} {
		outbox := &workerOutboxFake{items: []Item{{ID: "provider", Kind: "verify_email", Attempts: 2, ExpiresAt: time.Now().Add(time.Minute)}}}
		sender := &workerSenderFake{}
		if permanent {
			sender.err = PermanentError{Code: "resend_422"}
		} else {
			sender.err = errors.New("temporary transport")
		}
		logs := runWorkerOnce(t, outbox, sender)
		if len(outbox.failed) != 1 || outbox.failed[0] != "provider_error" || !strings.Contains(logs, `"permanent":`) {
			t.Fatal("provider error was not recorded")
		}
	}
	outbox := &workerOutboxFake{items: []Item{{ID: "sent-db-error", Kind: "verify_email", Attempts: 1, ExpiresAt: time.Now().Add(time.Minute)}, {ID: "claimed-error", Kind: "verify_email", ExpiresAt: time.Now().Add(time.Minute)}}}
	// A state persistence error is logged; no recipient or provider message is logged.
	outbox.sentErr = errors.New("write failed")
	sender := &workerSenderFake{}
	logs := runWorkerOnce(t, outbox, sender)
	if !strings.Contains(logs, "email sent state update failed") {
		t.Fatal("sent persistence failure was invisible")
	}
}

func TestWorkerLogsClaimFailureWithoutRawError(t *testing.T) {
	outbox := &workerOutboxFake{claimErr: errors.New("private SQL details or DSN")}
	sender := &workerSenderFake{}
	logs := runWorkerOnce(t, outbox, sender)
	if !strings.Contains(logs, "email outbox claim failed") || !strings.Contains(logs, "error_class") || strings.Contains(logs, "private SQL details") || strings.Contains(logs, "DSN") {
		t.Fatal("claim error was not observable or raw error leaked")
	}
}

func TestOutboxMessageWrongKeyFailsWithoutExposingPayload(t *testing.T) {
	good, _ := NewOutboxCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), 1)
	wrong, _ := NewOutboxCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)), 1)
	payload, _ := json.Marshal(Payload{To: "private@example.test", Kind: "verify_email", Data: TemplateData{Link: "private-token"}})
	ciphertext, err := good.Encrypt(payload)
	if err != nil {
		t.Fatal(err)
	}
	for i := range payload {
		payload[i] = 0
	}
	if _, err = (&Outbox{cipher: wrong}).Message(Item{Kind: "verify_email", Ciphertext: ciphertext, KeyVersion: 1}, "noreply@example.test", ""); err == nil {
		t.Fatal("wrong key unexpectedly decrypted payload")
	}
}

func TestResendSenderClassifiesLocalMockResponses(t *testing.T) {
	for _, tc := range []struct {
		code      int
		permanent bool
	}{{http.StatusServiceUnavailable, false}, {http.StatusUnprocessableEntity, true}} {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/emails" || r.Header.Get("Idempotency-Key") == "" {
					t.Error("unexpected local mock request")
				}
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(`{"name":"mock_error"}`))
			}))
			defer server.Close()
			sender := ResendSender{BaseURL: server.URL, APIKey: "unit-test-only", Client: server.Client()}
			_, err := sender.Send(context.Background(), Message{From: "noreply@example.test", To: "test@example.test", Subject: "test", IdempotencyKey: "fixture"})
			if err == nil || IsPermanent(err) != tc.permanent {
				t.Fatal("local provider response classified incorrectly")
			}
		})
	}
}

func TestOutboxRecipientHashIsStable(t *testing.T) {
	want := sha256.Sum256([]byte("test@example.test"))
	if hash(" Test@example.test ") != fmt.Sprintf("%x", want) {
		t.Fatal("recipient hashing behavior changed")
	}
}
