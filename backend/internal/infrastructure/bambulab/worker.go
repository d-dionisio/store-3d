package bambulab

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync"
	"time"
)

type storedConfig struct {
	Input      app.PrinterMonitoringConfigInput
	PrinterID  string
	Ciphertext []byte
	KeyVersion int16
}
type MonitorStore struct {
	pool   *pgxpool.Pool
	cipher *Cipher
}

func NewMonitorStore(pool *pgxpool.Pool, cipher *Cipher) *MonitorStore {
	return &MonitorStore{pool, cipher}
}
func (s *MonitorStore) list(ctx context.Context) ([]storedConfig, error) {
	rows, err := s.pool.Query(ctx, `SELECT printer_id,host::text,port,serial_number,certificate_sha256,credentials_ciphertext,key_version FROM printer_integrations WHERE enabled`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []storedConfig{}
	for rows.Next() {
		var c storedConfig
		if err = rows.Scan(&c.PrinterID, &c.Input.Host, &c.Input.Port, &c.Input.SerialNumber, &c.Input.CertificateSHA256, &c.Ciphertext, &c.KeyVersion); err != nil {
			return nil, err
		}
		plain, e := s.cipher.Decrypt(c.Ciphertext, c.KeyVersion)
		if e != nil {
			return nil, e
		}
		c.Input.AccessCode = string(plain)
		c.Input.Enabled = true
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *MonitorStore) save(ctx context.Context, id string, t domain.PrinterTelemetry) error {
	t.PrinterID = id
	_, err := s.pool.Exec(ctx, `INSERT INTO printer_current_telemetry(printer_id,connection_state,printer_state,progress_percent,nozzle_temperature_c,bed_temperature_c,speed_level,observed_job_name,elapsed_seconds,error_code,last_seen_at,payload_observed_at)VALUES($1,$2,$3,$4,$5,$6,$7,nullif($8,''),$9,nullif($10,''),$11,$12)ON CONFLICT(printer_id)DO UPDATE SET connection_state=excluded.connection_state,printer_state=excluded.printer_state,progress_percent=coalesce(excluded.progress_percent,printer_current_telemetry.progress_percent),nozzle_temperature_c=coalesce(excluded.nozzle_temperature_c,printer_current_telemetry.nozzle_temperature_c),bed_temperature_c=coalesce(excluded.bed_temperature_c,printer_current_telemetry.bed_temperature_c),speed_level=coalesce(excluded.speed_level,printer_current_telemetry.speed_level),observed_job_name=coalesce(excluded.observed_job_name,printer_current_telemetry.observed_job_name),elapsed_seconds=coalesce(excluded.elapsed_seconds,printer_current_telemetry.elapsed_seconds),error_code=excluded.error_code,last_seen_at=excluded.last_seen_at,payload_observed_at=excluded.payload_observed_at,updated_at=now()`, id, t.Connection, t.State, t.ProgressPercent, t.NozzleTemperatureC, t.BedTemperatureC, t.SpeedLevel, t.ObservedJobName, t.ElapsedSeconds, t.ErrorCode, t.LastSeenAt, t.PayloadObservedAt)
	if err != nil {
		return err
	}
	if t.ObservedJobName != "" {
		key := sha256.Sum256([]byte(t.ObservedJobName))
		_, err = s.pool.Exec(ctx, `INSERT INTO printer_observed_jobs(printer_id,provider_job_key,job_name,first_observed_at,last_observed_at,last_state,max_progress_percent)VALUES($1,$2,$3,now(),now(),$4,$5)ON CONFLICT(printer_id,provider_job_key)DO UPDATE SET last_observed_at=now(),last_state=excluded.last_state,max_progress_percent=greatest(printer_observed_jobs.max_progress_percent,excluded.max_progress_percent)`, id, hex.EncodeToString(key[:]), t.ObservedJobName, t.State, t.ProgressPercent)
	}
	return err
}
func (s *MonitorStore) event(ctx context.Context, id, kind, message string) error {
	bucket := time.Now().UTC().Truncate(time.Minute).Format(time.RFC3339)
	key := fmt.Sprintf("%s:%s:%s", id, kind, bucket)
	_, err := s.pool.Exec(ctx, `INSERT INTO printer_connection_events(printer_id,event_type,message,deduplication_key)VALUES($1,$2,$3,$4)ON CONFLICT(deduplication_key)DO NOTHING`, id, kind, message, key)
	return err
}
func (s *MonitorStore) markOffline(ctx context.Context, id string) {
	_, _ = s.pool.Exec(ctx, `INSERT INTO printer_current_telemetry(printer_id,connection_state,printer_state)VALUES($1,'offline','unknown')ON CONFLICT(printer_id)DO UPDATE SET connection_state='offline',updated_at=now()`, id)
}
func (s *MonitorStore) Cleanup(ctx context.Context, retention time.Duration) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM printer_connection_events WHERE occurred_at<now()-make_interval(secs=>$1)`, int64(retention.Seconds()))
	return err
}

type Worker struct {
	Store         *MonitorStore
	Provider      app.PrinterProvider
	MaxConcurrent int
	RetryMin      time.Duration
}

func (w *Worker) Run(ctx context.Context) error {
	configs, err := w.Store.list(ctx)
	if err != nil {
		return err
	}
	limit := w.MaxConcurrent
	if limit < 1 {
		limit = 4
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, cfg := range configs {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			backoff := w.RetryMin
			if backoff <= 0 {
				backoff = time.Second
			}
			for ctx.Err() == nil {
				_ = w.Store.event(ctx, cfg.PrinterID, "connecting", "")
				_ = w.Provider.Monitor(ctx, cfg.Input, func(t domain.PrinterTelemetry) error {
					_ = w.Store.event(ctx, cfg.PrinterID, "connected", "")
					return w.Store.save(ctx, cfg.PrinterID, t)
				})
				if ctx.Err() != nil {
					return
				}
				w.Store.markOffline(ctx, cfg.PrinterID)
				_ = w.Store.event(ctx, cfg.PrinterID, "disconnected", "monitoring connection ended")
				timer := time.NewTimer(backoff)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				if backoff < 30*time.Second {
					backoff *= 2
				}
			}
		}()
	}
	<-ctx.Done()
	wg.Wait()
	return nil
}
