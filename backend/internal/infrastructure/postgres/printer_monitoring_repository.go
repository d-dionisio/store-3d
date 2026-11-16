package postgres

import (
	"context"
	"errors"
	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"github.com/jackc/pgx/v5"
)

func scanMonitoringConfig(row pgx.Row) (app.PrinterMonitoringConfigView, error) {
	var v app.PrinterMonitoringConfigView
	err := row.Scan(&v.PrinterID, &v.Provider, &v.Host, &v.Port, &v.SerialNumber, &v.CertificateSHA256, &v.Enabled, &v.Version)
	return v, productionError(err)
}

const monitoringConfigSelect = `SELECT printer_id,provider,host::text,port,serial_number,certificate_sha256,enabled,version FROM printer_integrations`

func (r *ProductionRepository) SaveMonitoringConfig(ctx context.Context, id string, in app.PrinterMonitoringConfigInput, ciphertext []byte, keyVersion int16, actor string) (app.PrinterMonitoringConfigView, error) {
	tag, err := r.pool.Exec(ctx, `INSERT INTO printer_integrations(printer_id,provider,host,port,serial_number,certificate_sha256,credentials_ciphertext,key_version,enabled,created_by_admin_user_id,updated_by_admin_user_id) VALUES($1,'bambulab_lan',$2::inet,$3,$4,lower($5),$6,$7,$8,$9,$9) ON CONFLICT(printer_id) DO UPDATE SET host=excluded.host,port=excluded.port,serial_number=excluded.serial_number,certificate_sha256=excluded.certificate_sha256,credentials_ciphertext=excluded.credentials_ciphertext,key_version=excluded.key_version,enabled=excluded.enabled,updated_by_admin_user_id=excluded.updated_by_admin_user_id,version=printer_integrations.version+1,updated_at=now() WHERE printer_integrations.version=$10`, id, in.Host, in.Port, in.SerialNumber, in.CertificateSHA256, ciphertext, keyVersion, in.Enabled, actor, in.ExpectedVersion)
	if err != nil {
		return app.PrinterMonitoringConfigView{}, productionError(err)
	}
	if tag.RowsAffected() != 1 {
		return app.PrinterMonitoringConfigView{}, app.ErrConflict
	}
	return r.GetMonitoringConfig(ctx, id)
}
func (r *ProductionRepository) GetMonitoringConfig(ctx context.Context, id string) (app.PrinterMonitoringConfigView, error) {
	return scanMonitoringConfig(r.pool.QueryRow(ctx, monitoringConfigSelect+` WHERE printer_id=$1`, id))
}
func (r *ProductionRepository) GetTelemetry(ctx context.Context, id string) (domain.PrinterTelemetry, error) {
	var t domain.PrinterTelemetry
	var state string
	err := r.pool.QueryRow(ctx, `SELECT printer_id,connection_state,printer_state,progress_percent::float8,nozzle_temperature_c::float8,bed_temperature_c::float8,speed_level,coalesce(observed_job_name,''),elapsed_seconds,coalesce(error_code,''),last_seen_at,payload_observed_at FROM printer_current_telemetry WHERE printer_id=$1`, id).Scan(&t.PrinterID, &t.Connection, &state, &t.ProgressPercent, &t.NozzleTemperatureC, &t.BedTemperatureC, &t.SpeedLevel, &t.ObservedJobName, &t.ElapsedSeconds, &t.ErrorCode, &t.LastSeenAt, &t.PayloadObservedAt)
	t.State = domain.ObservedPrinterState(state)
	if errors.Is(err, pgx.ErrNoRows) {
		t.PrinterID = id
		t.Connection = domain.ConnectionUnknown
		t.State = "unknown"
		return t, nil
	}
	return t, productionError(err)
}
func (r *ProductionRepository) ListConnectionEvents(ctx context.Context, id string, limit int) ([]app.ConnectionEventView, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,event_type,coalesce(message,''),occurred_at FROM printer_connection_events WHERE printer_id=$1 ORDER BY occurred_at DESC LIMIT $2`, id, limit)
	if err != nil {
		return nil, productionError(err)
	}
	defer rows.Close()
	out := []app.ConnectionEventView{}
	for rows.Next() {
		var v app.ConnectionEventView
		if err = rows.Scan(&v.ID, &v.EventType, &v.Message, &v.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ProductionRepository) ListObservedJobs(ctx context.Context, id string, limit int) ([]app.ObservedJobView, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,provider_job_key,coalesce(job_name,''),last_state,first_observed_at,last_observed_at,max_progress_percent::float8 FROM printer_observed_jobs WHERE printer_id=$1 ORDER BY last_observed_at DESC LIMIT $2`, id, limit)
	if err != nil {
		return nil, productionError(err)
	}
	defer rows.Close()
	out := []app.ObservedJobView{}
	for rows.Next() {
		var v app.ObservedJobView
		if err = rows.Scan(&v.ID, &v.ProviderJobKey, &v.JobName, &v.LastState, &v.FirstObservedAt, &v.LastObservedAt, &v.MaxProgressPercent); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
