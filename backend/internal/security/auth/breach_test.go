package auth

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKAnonymityCheckerSendsOnlyPrefix(t *testing.T) {
	password := "uma senha comprometida de teste"
	sum := sha1.Sum([]byte(password))
	encoded := strings.ToUpper(hex.EncodeToString(sum[:]))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, password) || !strings.HasSuffix(r.URL.Path, encoded[:5]) {
			t.Error("password exposed or wrong prefix")
		}
		_, _ = w.Write([]byte(encoded[5:] + ":42\n"))
	}))
	defer server.Close()
	compromised, err := (KAnonymityChecker{BaseURL: server.URL, Client: server.Client()}).Compromised(context.Background(), password)
	if err != nil || !compromised {
		t.Fatalf("compromised=%v err=%v", compromised, err)
	}
}

func TestKAnonymityFailurePolicy(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })}
	if compromised, err := (KAnonymityChecker{BaseURL: "https://invalid", Client: client}).Compromised(context.Background(), "long enough password"); err != nil || compromised {
		t.Fatal("fail-open policy failed")
	}
	if _, err := (KAnonymityChecker{BaseURL: "https://invalid", Client: client, FailClosed: true}).Compromised(context.Background(), "long enough password"); err == nil {
		t.Fatal("fail-closed policy failed")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
