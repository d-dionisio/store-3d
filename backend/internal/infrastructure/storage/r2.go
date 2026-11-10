package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type R2 struct {
	account, bucket, keyID, secret string
	client                         *http.Client
}

func NewR2FromEnv() (*R2, error) {
	r := &R2{account: os.Getenv("R2_ACCOUNT_ID"), bucket: os.Getenv("R2_BUCKET"), keyID: os.Getenv("R2_ACCESS_KEY_ID"), secret: os.Getenv("R2_SECRET_ACCESS_KEY"), client: &http.Client{Timeout: 10 * time.Second}}
	if r.account == "" || r.bucket == "" || r.keyID == "" || r.secret == "" {
		return nil, errors.New("R2 configuration is incomplete")
	}
	return r, nil
}
func hmacSum(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value))
	return h.Sum(nil)
}
func (r *R2) presign(method, key string, ttl time.Duration, extra map[string]string) (string, error) {
	now := time.Now().UTC()
	date := now.Format("20060102")
	stamp := now.Format("20060102T150405Z")
	host := r.account + ".r2.cloudflarestorage.com"
	path := "/" + r.bucket + "/" + strings.TrimLeft(key, "/")
	headers := map[string]string{"host": host}
	for k, v := range extra {
		headers[strings.ToLower(k)] = v
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	signed := strings.Join(names, ";")
	canonicalHeaders := ""
	for _, k := range names {
		canonicalHeaders += k + ":" + strings.TrimSpace(headers[k]) + "\n"
	}
	q := url.Values{}
	scope := date + "/auto/s3/aws4_request"
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", r.keyID+"/"+scope)
	q.Set("X-Amz-Date", stamp)
	q.Set("X-Amz-Expires", fmt.Sprint(int(ttl.Seconds())))
	q.Set("X-Amz-SignedHeaders", signed)
	canonical := method + "\n" + path + "\n" + q.Encode() + "\n" + canonicalHeaders + "\n" + signed + "\nUNSIGNED-PAYLOAD"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	kd := hmacSum([]byte("AWS4"+r.secret), date)
	kr := hmacSum(kd, "auto")
	ks := hmacSum(kr, "s3")
	signing := hmacSum(ks, "aws4_request")
	sig := hex.EncodeToString(hmacSum(signing, toSign))
	q.Set("X-Amz-Signature", sig)
	return "https://" + host + path + "?" + q.Encode(), nil
}
func (r *R2) PresignPut(ctx context.Context, key, checksum string, size int64, ttl time.Duration) (string, map[string]string, error) {
	_ = ctx
	raw, err := hex.DecodeString(checksum)
	if err != nil {
		return "", nil, errors.New("invalid sha256 checksum")
	}
	headers := map[string]string{"x-amz-meta-sha256": checksum, "x-amz-checksum-sha256": base64.StdEncoding.EncodeToString(raw)}
	u, err := r.presign(http.MethodPut, key, ttl, headers)
	return u, headers, err
}
func (r *R2) Verify(ctx context.Context, key string, size int64, checksum, contentType string) error {
	u, err := r.presign(http.MethodHead, key, time.Minute, nil)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("storage object unavailable")
	}
	rawChecksum, err := hex.DecodeString(checksum)
	if err != nil {
		return errors.New("invalid expected checksum")
	}
	expectedStorageChecksum := base64.StdEncoding.EncodeToString(rawChecksum)
	if resp.ContentLength != size || resp.Header.Get("x-amz-meta-sha256") != checksum || resp.Header.Get("x-amz-checksum-sha256") != expectedStorageChecksum || resp.Header.Get("Content-Type") != contentType {
		return errors.New("storage metadata mismatch")
	}
	getURL, err := r.presign(http.MethodGet, key, time.Minute, nil)
	if err != nil {
		return err
	}
	getRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return err
	}
	getRequest.Header.Set("Range", "bytes=0-83")
	first, err := r.client.Do(getRequest)
	if err != nil {
		return err
	}
	defer first.Body.Close()
	header, err := io.ReadAll(io.LimitReader(first.Body, 84))
	if err != nil {
		return err
	}
	if contentType == "application/vnd.ms-package.3dmanufacturing-3dmodel+xml" {
		if len(header) < 4 || string(header[:4]) != "PK\x03\x04" {
			return errors.New("invalid 3mf signature")
		}
	} else {
		ascii := len(header) >= 5 && strings.EqualFold(string(header[:5]), "solid")
		binarySTL := len(header) == 84 && int64(84)+int64(binary.LittleEndian.Uint32(header[80:84]))*50 == size
		if !ascii && !binarySTL {
			return errors.New("invalid stl structure")
		}
	}
	return nil
}
func (r *R2) Bucket() string { return r.bucket }

type Unavailable struct{}

func (Unavailable) PresignPut(context.Context, string, string, int64, time.Duration) (string, map[string]string, error) {
	return "", nil, errors.New("object storage unavailable")
}
func (Unavailable) Verify(context.Context, string, int64, string, string) error {
	return errors.New("object storage unavailable")
}
