package mail

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func VerifyWebhook(secret, id, timestamp, signature string, payload []byte, now time.Time) bool {
	if strings.HasPrefix(secret, "whsec_") {
		secret = strings.TrimPrefix(secret, "whsec_")
	}
	key, e := base64.StdEncoding.DecodeString(secret)
	if e != nil {
		return false
	}
	unix, e := strconv.ParseInt(timestamp, 10, 64)
	if e != nil || now.Sub(time.Unix(unix, 0)) > 5*time.Minute || time.Unix(unix, 0).Sub(now) > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(id + "." + timestamp + "." + string(payload)))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	for _, part := range strings.Fields(signature) {
		value := strings.TrimPrefix(part, "v1,")
		if hmac.Equal([]byte(value), []byte(expected)) {
			return true
		}
	}
	return false
}
func ResendWebhook(pool *pgxpool.Pool, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if e != nil {
			w.WriteHeader(400)
			return
		}
		id, stamp, sig := r.Header.Get("svix-id"), r.Header.Get("svix-timestamp"), r.Header.Get("svix-signature")
		if secret == "" || !VerifyWebhook(secret, id, stamp, sig, body, time.Now()) {
			w.WriteHeader(401)
			return
		}
		var event struct {
			Type      string `json:"type"`
			CreatedAt string `json:"created_at"`
			Data      struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &event) != nil || id == "" || event.Data.ID == "" {
			w.WriteHeader(400)
			return
		}
		sum := sha256.Sum256(body)
		tx, e := pool.Begin(context.Background())
		if e != nil {
			w.WriteHeader(503)
			return
		}
		defer tx.Rollback(context.Background())
		tag, e := tx.Exec(r.Context(), `INSERT INTO email_delivery_events(provider_event_id,provider_message_id,event_type,payload_hash) VALUES($1,$2,$3,$4) ON CONFLICT(provider_event_id) DO NOTHING`, id, event.Data.ID, event.Type, hex.EncodeToString(sum[:]))
		if e != nil {
			w.WriteHeader(500)
			return
		}
		if tag.RowsAffected() == 0 {
			w.WriteHeader(200)
			return
		}
		status := ""
		switch event.Type {
		case "email.delivered":
			status = "delivered"
		case "email.bounced":
			status = "bounced"
		case "email.complained":
			status = "complained"
		case "email.delivery_delayed":
		// The provider still owns this accepted delivery. Retrying the send here
		// could duplicate mail after Resend's 24-hour idempotency window.
		status = "sent"
		}
		if status != "" {
			_, e = tx.Exec(r.Context(), `UPDATE email_outbox SET status=$2,processed_at=CASE WHEN $2 IN('delivered','bounced','complained') THEN now() ELSE processed_at END WHERE provider_message_id=$1`, event.Data.ID, status)
		}
		if e == nil && (status == "bounced" || status == "complained") {
			_, e = tx.Exec(r.Context(), `INSERT INTO email_suppressions(recipient_hash,reason) SELECT recipient_hash,CASE WHEN $2='bounced' THEN 'bounce' ELSE 'complaint' END FROM email_outbox WHERE provider_message_id=$1 ON CONFLICT(recipient_hash) DO UPDATE SET reason=excluded.reason`, event.Data.ID, status)
		}
		if e != nil || tx.Commit(r.Context()) != nil {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}
}
