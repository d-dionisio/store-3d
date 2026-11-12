package mail

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	mathrand "math/rand/v2"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Payload struct {
	To, Kind string
	Data     TemplateData
}
type OutboxCipher struct {
	key     []byte
	Version int16
}

func NewOutboxCipher(encoded string, version int16) (*OutboxCipher, error) {
	key, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || len(key) != 32 || version < 1 {
		return nil, errors.New("email outbox key must contain 32 base64 bytes")
	}
	return &OutboxCipher{key, version}, nil
}
func (c *OutboxCipher) encrypt(value []byte) ([]byte, error) {
	block, e := aes.NewCipher(c.key)
	if e != nil {
		return nil, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	return gcm.Seal(nonce, nonce, value, []byte(fmt.Sprintf("loja3d:email:v%d", c.Version))), nil
}
func (c *OutboxCipher) decrypt(value []byte, version int16) ([]byte, error) {
	if version != c.Version {
		return nil, errors.New("email outbox key version unavailable")
	}
	block, e := aes.NewCipher(c.key)
	if e != nil {
		return nil, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil || len(value) < gcm.NonceSize() {
		return nil, errors.New("invalid email ciphertext")
	}
	return gcm.Open(nil, value[:gcm.NonceSize()], value[gcm.NonceSize():], []byte(fmt.Sprintf("loja3d:email:v%d", version)))
}

type Outbox struct {
	pool   *pgxpool.Pool
	cipher *OutboxCipher
}

func NewOutbox(pool *pgxpool.Pool, cipher *OutboxCipher) *Outbox { return &Outbox{pool, cipher} }
func hash(value string) string {
	s := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(s[:])
}
func (o *Outbox) enqueue(ctx context.Context, to, kind string, data TemplateData) error {
	return o.EnqueueTx(ctx, nil, to, kind, data)
}
func (o *Outbox) EnqueueTx(ctx context.Context, tx pgx.Tx, to, kind string, data TemplateData) error {
	if ValidateAddress(to) != nil {
		return errors.New("invalid recipient")
	}
	payload, _ := json.Marshal(Payload{to, kind, data})
	encrypted, e := o.cipher.encrypt(payload)
	for i := range payload {
		payload[i] = 0
	}
	if e != nil {
		return e
	}
	idempotency := kind + "/" + hash(to+"|"+data.Link+"|"+data.Reference+"|"+data.Status+"|"+data.Decision)
	query := `INSERT INTO email_outbox(message_type,recipient_hash,payload_ciphertext,key_version,idempotency_key,expires_at) VALUES($1,$2,$3,$4,$5,now()+make_interval(secs=>$6)) ON CONFLICT(idempotency_key) DO NOTHING`
	if tx != nil {
		_, e = tx.Exec(ctx, query, kind, hash(to), encrypted, o.cipher.Version, idempotency, int64(LinkExpiry(kind).Seconds()))
	} else {
		_, e = o.pool.Exec(ctx, query, kind, hash(to), encrypted, o.cipher.Version, idempotency, int64(LinkExpiry(kind).Seconds()))
	}
	return e
}
func (o *Outbox) SendVerification(ctx context.Context, to, link string) error {
	return o.enqueue(ctx, to, "verify_email", TemplateData{Link: link})
}
func (o *Outbox) SendPasswordReset(ctx context.Context, to, link string) error {
	return o.enqueue(ctx, to, "password_reset", TemplateData{Link: link})
}
func (o *Outbox) SendMagicLink(ctx context.Context, to, link string) error {
	return o.enqueue(ctx, to, "magic_link", TemplateData{Link: link})
}
func (o *Outbox) SendDecision(ctx context.Context, to, reference, decision string) error {
	return o.enqueue(ctx, to, "quote_decision", TemplateData{Reference: reference, Decision: decision})
}
func (o *Outbox) SendSecurityEvent(ctx context.Context, to, kind string) error {
	return o.enqueue(ctx, to, kind, TemplateData{})
}

type Item struct {
	ID, Kind, Idempotency string
	Ciphertext            []byte
	KeyVersion            int16
	Attempts              int
	ExpiresAt             time.Time
}

func (o *Outbox) Claim(ctx context.Context, limit int) ([]Item, error) {
	rows, e := o.pool.Query(ctx, `WITH candidates AS (SELECT e.id FROM email_outbox e WHERE ((e.status IN('pending','retry') AND e.next_attempt_at<=now()) OR (e.status='processing' AND e.locked_at<now()-interval '5 minutes')) AND NOT EXISTS(SELECT 1 FROM email_suppressions s WHERE s.recipient_hash=e.recipient_hash) AND (SELECT count(*) FROM email_outbox WHERE provider_message_id IS NOT NULL AND processed_at>=date_trunc('day',now()))<100 ORDER BY e.created_at FOR UPDATE SKIP LOCKED LIMIT $1) UPDATE email_outbox e SET status='processing',locked_at=now(),attempts=attempts+1 FROM candidates c WHERE e.id=c.id RETURNING e.id,e.message_type,e.idempotency_key,e.payload_ciphertext,e.key_version,e.attempts,e.expires_at`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var result []Item
	for rows.Next() {
		var i Item
		if e = rows.Scan(&i.ID, &i.Kind, &i.Idempotency, &i.Ciphertext, &i.KeyVersion, &i.Attempts, &i.ExpiresAt); e != nil {
			return nil, e
		}
		result = append(result, i)
	}
	return result, rows.Err()
}
func (o *Outbox) Message(item Item, from, reply string) (Message, error) {
	plain, e := o.cipher.decrypt(item.Ciphertext, item.KeyVersion)
	if e != nil {
		return Message{}, e
	}
	defer func() {
		for i := range plain {
			plain[i] = 0
		}
	}()
	var p Payload
	if e = json.Unmarshal(plain, &p); e != nil {
		return Message{}, e
	}
	rendered, e := Render(p.Kind, p.Data)
	if e != nil {
		return Message{}, e
	}
	return Message{From: from, ReplyTo: reply, To: p.To, Subject: rendered.Subject, HTML: rendered.HTML, Text: rendered.Text, IdempotencyKey: item.Idempotency}, nil
}
func (o *Outbox) Sent(ctx context.Context, id, provider string) error {
	_, e := o.pool.Exec(ctx, `UPDATE email_outbox SET status='sent',provider_message_id=$2,processed_at=now(),locked_at=NULL,last_error_code=NULL WHERE id=$1 AND status='processing'`, id, provider)
	return e
}
func (o *Outbox) Expired(ctx context.Context, id string) error {
	_, e := o.pool.Exec(ctx, `UPDATE email_outbox SET status='expired',processed_at=now(),locked_at=NULL,last_error_code='expired' WHERE id=$1`, id)
	return e
}
func (o *Outbox) Failed(ctx context.Context, item Item, code string, permanent bool) error {
	status := "retry"
	if permanent || item.Attempts >= 8 {
		status = "failed"
	}
	delay := time.Duration(math.Pow(2, float64(item.Attempts)))*time.Minute + time.Duration(mathrand.IntN(30))*time.Second
	_, e := o.pool.Exec(ctx, `UPDATE email_outbox SET status=$2,next_attempt_at=now()+make_interval(secs=>$3),locked_at=NULL,last_error_code=$4,processed_at=CASE WHEN $2='failed' THEN now() ELSE NULL END WHERE id=$1`, item.ID, status, int64(delay.Seconds()), code)
	return e
}
func (o *Outbox) Pending(ctx context.Context) (int64, error) {
	var n int64
	e := o.pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE status IN('pending','retry','processing')`).Scan(&n)
	return n, e
}
func IsPermanent(err error) bool { var target PermanentError; return errors.As(err, &target) }

type Worker struct {
	Outbox        *Outbox
	Sender        EmailSender
	From, ReplyTo string
	Batch         int
	Rate          time.Duration
}

func (w Worker) Run(ctx context.Context) error {
	if w.Batch <= 0 {
		w.Batch = 5
	}
	if w.Rate <= 0 {
		w.Rate = time.Second
	}
	ticker := time.NewTicker(w.Rate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			items, e := w.Outbox.Claim(ctx, w.Batch)
			if e != nil {
				continue
			}
			for _, item := range items {
				if time.Now().After(item.ExpiresAt) {
					_ = w.Outbox.Expired(ctx, item.ID)
					continue
				}
				message, e := w.Outbox.Message(item, w.From, w.ReplyTo)
				if e != nil {
					_ = w.Outbox.Failed(ctx, item, "decrypt_or_render", true)
					continue
				}
				sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				provider, e := w.Sender.Send(sendCtx, message)
				cancel()
				if e != nil {
					_ = w.Outbox.Failed(ctx, item, "provider_error", IsPermanent(e))
					continue
				}
				_ = w.Outbox.Sent(ctx, item.ID, provider)
			}
		}
	}
}
