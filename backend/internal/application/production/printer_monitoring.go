package production

import (
	"context"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"net"
	"strings"
	"time"
)

type PrinterMonitoringConfigInput struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	SerialNumber      string `json:"serial_number"`
	CertificateSHA256 string `json:"certificate_sha256"`
	AccessCode        string `json:"access_code"`
	Enabled           bool   `json:"enabled"`
	ExpectedVersion   int64  `json:"expected_version"`
}
type PrinterMonitoringConfigView struct {
	PrinterID         string `json:"printer_id"`
	Provider          string `json:"provider"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	SerialNumber      string `json:"serial_number"`
	CertificateSHA256 string `json:"certificate_sha256"`
	Enabled           bool   `json:"enabled"`
	Version           int64  `json:"version"`
}
type PrinterMonitoringView struct {
	Telemetry             domain.PrinterTelemetry `json:"telemetry"`
	Fresh                 bool                    `json:"fresh"`
	EffectiveAvailability string                  `json:"effective_availability"`
	StaleAfterSeconds     int                     `json:"stale_after_seconds"`
}
type ConnectionEventView struct {
	ID         string    `json:"id"`
	EventType  string    `json:"event_type"`
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurred_at"`
}
type ObservedJobView struct {
	ID                 string    `json:"id"`
	ProviderJobKey     string    `json:"provider_job_key"`
	JobName            string    `json:"job_name"`
	LastState          string    `json:"last_state"`
	FirstObservedAt    time.Time `json:"first_observed_at"`
	LastObservedAt     time.Time `json:"last_observed_at"`
	MaxProgressPercent *float64  `json:"max_progress_percent"`
}

type MonitoringRepository interface {
	SaveMonitoringConfig(context.Context, string, PrinterMonitoringConfigInput, []byte, int16, string) (PrinterMonitoringConfigView, error)
	GetMonitoringConfig(context.Context, string) (PrinterMonitoringConfigView, error)
	GetTelemetry(context.Context, string) (domain.PrinterTelemetry, error)
	ListConnectionEvents(context.Context, string, int) ([]ConnectionEventView, error)
	ListObservedJobs(context.Context, string, int) ([]ObservedJobView, error)
}
type PrinterProvider interface {
	TestConnection(context.Context, PrinterMonitoringConfigInput) error
	Monitor(context.Context, PrinterMonitoringConfigInput, func(domain.PrinterTelemetry) error) error
}
type CredentialEncryptor interface {
	Encrypt([]byte) ([]byte, error)
	Version() int16
}

func validMonitoringTarget(in PrinterMonitoringConfigInput) bool {
	ip := net.ParseIP(strings.TrimSpace(in.Host))
	if ip == nil || (!ip.IsPrivate() && !ip.IsLoopback()) {
		return false
	}
	return in.Port > 0 && in.Port <= 65535 && len(strings.TrimSpace(in.SerialNumber)) >= 5 && len(in.CertificateSHA256) == 64
}
func (s *Service) SetMonitoring(repo MonitoringRepository, provider PrinterProvider, cipher CredentialEncryptor) {
	s.monitoring, s.connectionTester, s.monitoringCipher = repo, provider, cipher
}
func (s *Service) SaveMonitoringConfig(ctx context.Context, a admindomain.Session, id string, in PrinterMonitoringConfigInput) (PrinterMonitoringConfigView, error) {
	if err := require(canManage(a)); err != nil {
		return PrinterMonitoringConfigView{}, err
	}
	if !validMonitoringTarget(in) || in.AccessCode == "" || s.monitoringCipher == nil {
		return PrinterMonitoringConfigView{}, ErrInvalid
	}
	encrypted, err := s.monitoringCipher.Encrypt([]byte(in.AccessCode))
	if err != nil {
		return PrinterMonitoringConfigView{}, err
	}
	return s.monitoring.SaveMonitoringConfig(ctx, id, in, encrypted, s.monitoringCipher.Version(), a.UserID)
}
func (s *Service) GetMonitoring(ctx context.Context, a admindomain.Session, id string) (PrinterMonitoringView, error) {
	if err := require(canRead(a)); err != nil {
		return PrinterMonitoringView{}, err
	}
	t, err := s.monitoring.GetTelemetry(ctx, id)
	if err != nil {
		return PrinterMonitoringView{}, err
	}
	const age = 45 * time.Second
	return PrinterMonitoringView{Telemetry: t, Fresh: t.Fresh(time.Now(), age), EffectiveAvailability: t.EffectiveAvailability(time.Now(), age), StaleAfterSeconds: int(age.Seconds())}, nil
}
func (s *Service) GetMonitoringConfig(ctx context.Context, a admindomain.Session, id string) (PrinterMonitoringConfigView, error) {
	if err := require(canRead(a)); err != nil {
		return PrinterMonitoringConfigView{}, err
	}
	return s.monitoring.GetMonitoringConfig(ctx, id)
}
func (s *Service) TestMonitoringConnection(ctx context.Context, a admindomain.Session, in PrinterMonitoringConfigInput) error {
	if err := require(canManage(a)); err != nil {
		return err
	}
	if !validMonitoringTarget(in) || in.AccessCode == "" {
		return ErrInvalid
	}
	return s.connectionTester.TestConnection(ctx, in)
}
func (s *Service) ListConnectionEvents(ctx context.Context, a admindomain.Session, id string) ([]ConnectionEventView, error) {
	if err := require(canRead(a)); err != nil {
		return nil, err
	}
	return s.monitoring.ListConnectionEvents(ctx, id, 100)
}
func (s *Service) ListObservedJobs(ctx context.Context, a admindomain.Session, id string) ([]ObservedJobView, error) {
	if err := require(canRead(a)); err != nil {
		return nil, err
	}
	return s.monitoring.ListObservedJobs(ctx, id, 100)
}
