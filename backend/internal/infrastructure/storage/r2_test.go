package storage

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func TestR2PresignPutBindsChecksumAndExpiry(t *testing.T) {
	r := &R2{account: "account", bucket: "private-bucket", keyID: "key-id", secret: "secret"}
	checksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	signed, headers, err := r.PresignPut(context.Background(), "quotes/pending/file.stl", checksum, 84, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("X-Amz-Expires") != "900" {
		t.Fatalf("unexpected expiry: %s", parsed.Query().Get("X-Amz-Expires"))
	}
	if headers["x-amz-meta-sha256"] != checksum || headers["x-amz-checksum-sha256"] == "" {
		t.Fatalf("checksum headers missing: %#v", headers)
	}
	if parsed.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("signature missing")
	}
	if parsed.Host != "account.r2.cloudflarestorage.com" {
		t.Fatalf("unexpected host: %s", parsed.Host)
	}
}
