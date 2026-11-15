package production

import (
	"context"
	"encoding/json"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"time"
)

func marshalTechnical(value any) ([]byte, error) { return json.Marshal(value) }

type PrinterSpecInput struct {
	Status               string                `json:"status"`
	Technology           string                `json:"technology"`
	TechnicalNotes       string                `json:"technical_notes"`
	SourceReference      string                `json:"source_reference"`
	MaxWidthMM           *int                  `json:"max_width_mm"`
	MaxDepthMM           *int                  `json:"max_depth_mm"`
	MaxHeightMM          *int                  `json:"max_height_mm"`
	MaxHotendTempC       *int                  `json:"max_hotend_temp_c"`
	MaxBedTempC          *int                  `json:"max_bed_temp_c"`
	EnclosedChamber      *bool                 `json:"enclosed_chamber"`
	ExpectedVersion      int64                 `json:"expected_version"`
	SupportedMaterialIDs []string              `json:"supported_material_ids"`
	SupportedNozzles     []domain.NozzleOption `json:"supported_nozzles"`
	Surfaces             []string              `json:"surfaces"`
	Features             []string              `json:"features"`
}
type PrinterSpecView struct {
	ID, PrinterID, Status, Technology, TechnicalNotes, SourceReference string                `json:"-"`
	Version                                                            int64                 `json:"version"`
	OperationalConfigVersion                                           int64                 `json:"operational_config_version"`
	MaxWidthMM                                                         *int                  `json:"max_width_mm"`
	MaxDepthMM                                                         *int                  `json:"max_depth_mm"`
	MaxHeightMM                                                        *int                  `json:"max_height_mm"`
	MaxHotendTempC                                                     *int                  `json:"max_hotend_temp_c"`
	MaxBedTempC                                                        *int                  `json:"max_bed_temp_c"`
	EnclosedChamber                                                    *bool                 `json:"enclosed_chamber"`
	SupportedMaterialIDs                                               []string              `json:"supported_material_ids"`
	SupportedNozzles                                                   []domain.NozzleOption `json:"supported_nozzles"`
	Surfaces                                                           []string              `json:"surfaces"`
	Features                                                           []string              `json:"features"`
	CreatedAt                                                          time.Time             `json:"created_at"`
}

func (v PrinterSpecView) MarshalJSON() ([]byte, error) {
	type alias PrinterSpecView
	return marshalTechnical(map[string]any{"id": v.ID, "printer_id": v.PrinterID, "status": v.Status, "technology": v.Technology, "technical_notes": v.TechnicalNotes, "source_reference": v.SourceReference, "version": v.Version, "operational_config_version": v.OperationalConfigVersion, "max_width_mm": v.MaxWidthMM, "max_depth_mm": v.MaxDepthMM, "max_height_mm": v.MaxHeightMM, "max_hotend_temp_c": v.MaxHotendTempC, "max_bed_temp_c": v.MaxBedTempC, "enclosed_chamber": v.EnclosedChamber, "supported_material_ids": v.SupportedMaterialIDs, "supported_nozzles": v.SupportedNozzles, "surfaces": v.Surfaces, "features": v.Features, "created_at": v.CreatedAt})
}

type MaterialSpecInput struct {
	Manufacturer          string `json:"manufacturer"`
	CommercialName        string `json:"commercial_name"`
	PolymerType           string `json:"polymer_type"`
	MoistureSensitivity   string `json:"moisture_sensitivity"`
	RequiredSurface       string `json:"required_surface"`
	Notes                 string `json:"notes"`
	Status                string `json:"status"`
	SourceReference       string `json:"source_reference"`
	PrintTempMinC         *int   `json:"print_temp_min_c"`
	PrintTempMaxC         *int   `json:"print_temp_max_c"`
	PrintTempRecommendedC *int   `json:"print_temp_recommended_c"`
	BedTempMinC           *int   `json:"bed_temp_min_c"`
	BedTempMaxC           *int   `json:"bed_temp_max_c"`
	BedTempRecommendedC   *int   `json:"bed_temp_recommended_c"`
	RecommendedSpeedMMS   *int   `json:"recommended_speed_mm_s"`
	RequiresEnclosure     *bool  `json:"requires_enclosure"`
	RequiresDrying        *bool  `json:"requires_drying"`
	ExpectedVersion       int64  `json:"expected_version"`
}
type MaterialSpecView struct {
	MaterialID, MaterialName, Manufacturer, CommercialName, PolymerType, MoistureSensitivity, RequiredSurface, Notes, Status, SourceReference string
	PrintTempMinC, PrintTempMaxC, PrintTempRecommendedC, BedTempMinC, BedTempMaxC, BedTempRecommendedC, RecommendedSpeedMMS                   *int
	RequiresEnclosure, RequiresDrying                                                                                                         *bool
	Version                                                                                                                                   int64
}

func (v MaterialSpecView) MarshalJSON() ([]byte, error) {
	return marshalTechnical(map[string]any{"material_id": v.MaterialID, "material_name": v.MaterialName, "manufacturer": v.Manufacturer, "commercial_name": v.CommercialName, "polymer_type": v.PolymerType, "print_temp_min_c": v.PrintTempMinC, "print_temp_max_c": v.PrintTempMaxC, "print_temp_recommended_c": v.PrintTempRecommendedC, "bed_temp_min_c": v.BedTempMinC, "bed_temp_max_c": v.BedTempMaxC, "bed_temp_recommended_c": v.BedTempRecommendedC, "recommended_speed_mm_s": v.RecommendedSpeedMMS, "requires_enclosure": v.RequiresEnclosure, "requires_drying": v.RequiresDrying, "moisture_sensitivity": v.MoistureSensitivity, "required_surface": v.RequiredSurface, "notes": v.Notes, "status": v.Status, "source_reference": v.SourceReference, "version": v.Version})
}

type OperationalConfigInput struct {
	Status                 string `json:"status"`
	InstalledNozzleType    string `json:"installed_nozzle_type"`
	InstalledSurface       string `json:"installed_surface"`
	InstalledNozzleMicrons *int   `json:"installed_nozzle_microns"`
	MaterialSlotCount      *int   `json:"material_slot_count"`
	ExpectedVersion        int64  `json:"expected_version"`
}
type SpoolInstallationView struct {
	ID, PrinterID, PrinterName, SpoolID, SpoolCode, MaterialID, MaterialName, ColorID, ColorName string
	SlotNumber                                                                                   int
	InstalledAt                                                                                  time.Time
	RemovedAt                                                                                    *time.Time
}

func (v SpoolInstallationView) MarshalJSON() ([]byte, error) {
	return marshalTechnical(map[string]any{"id": v.ID, "printer_id": v.PrinterID, "printer_name": v.PrinterName, "spool_id": v.SpoolID, "spool_code": v.SpoolCode, "material_id": v.MaterialID, "material_name": v.MaterialName, "color_id": v.ColorID, "color_name": v.ColorName, "slot_number": v.SlotNumber, "installed_at": v.InstalledAt, "removed_at": v.RemovedAt})
}

type TechnicalCompatibilityRequest struct {
	PrinterID  string `json:"printer_id"`
	MaterialID string `json:"material_id"`
	ColorID    string `json:"color_id"`
	WidthMM    *int   `json:"width_mm"`
	DepthMM    *int   `json:"depth_mm"`
	HeightMM   *int   `json:"height_mm"`
}
type TechnicalRepository interface {
	ListPrinterSpecs(context.Context) ([]PrinterSpecView, error)
	SavePrinterSpec(context.Context, string, PrinterSpecInput, string) (PrinterSpecView, error)
	GetPrinterSpec(context.Context, string) (PrinterSpecView, error)
	PrinterSpecHistory(context.Context, string) ([]PrinterSpecView, error)
	SaveOperationalConfig(context.Context, string, OperationalConfigInput, string) error
	ListMaterialSpecs(context.Context) ([]MaterialSpecView, error)
	SaveMaterialSpec(context.Context, string, MaterialSpecInput, string) (MaterialSpecView, error)
	GetMaterialSpec(context.Context, string) (MaterialSpecView, error)
	InstallSpool(context.Context, string, string, int, string) (SpoolInstallationView, error)
	RemoveSpool(context.Context, string, string, string) error
	ListSpoolInstallations(context.Context, string, bool) ([]SpoolInstallationView, error)
	TechnicalCompatibility(context.Context, TechnicalCompatibilityRequest) (domain.CompatibilityResult, error)
}

func (s *Service) SetTechnicalRepository(r TechnicalRepository) { s.technical = r }
func (s *Service) ListPrinterSpecs(ctx context.Context, a admindomain.Session) ([]PrinterSpecView, error) {
	if err := require(canRead(a)); err != nil {
		return nil, err
	}
	return s.technical.ListPrinterSpecs(ctx)
}
func (s *Service) SavePrinterSpec(ctx context.Context, a admindomain.Session, id string, in PrinterSpecInput) (PrinterSpecView, error) {
	if err := require(canManage(a)); err != nil {
		return PrinterSpecView{}, err
	}
	return s.technical.SavePrinterSpec(ctx, id, in, a.UserID)
}
func (s *Service) GetPrinterSpec(ctx context.Context, a admindomain.Session, id string) (PrinterSpecView, error) {
	if err := require(canRead(a)); err != nil {
		return PrinterSpecView{}, err
	}
	return s.technical.GetPrinterSpec(ctx, id)
}
func (s *Service) PrinterSpecHistory(ctx context.Context, a admindomain.Session, id string) ([]PrinterSpecView, error) {
	if err := require(canRead(a)); err != nil {
		return nil, err
	}
	return s.technical.PrinterSpecHistory(ctx, id)
}
func (s *Service) SaveOperationalConfig(ctx context.Context, a admindomain.Session, id string, in OperationalConfigInput) error {
	if err := require(canManage(a)); err != nil {
		return err
	}
	return s.technical.SaveOperationalConfig(ctx, id, in, a.UserID)
}
func (s *Service) ListMaterialSpecs(ctx context.Context, a admindomain.Session) ([]MaterialSpecView, error) {
	if err := require(canRead(a)); err != nil {
		return nil, err
	}
	return s.technical.ListMaterialSpecs(ctx)
}
func (s *Service) SaveMaterialSpec(ctx context.Context, a admindomain.Session, id string, in MaterialSpecInput) (MaterialSpecView, error) {
	if err := require(canManage(a)); err != nil {
		return MaterialSpecView{}, err
	}
	return s.technical.SaveMaterialSpec(ctx, id, in, a.UserID)
}
func (s *Service) GetMaterialSpec(ctx context.Context, a admindomain.Session, id string) (MaterialSpecView, error) {
	if err := require(canRead(a)); err != nil {
		return MaterialSpecView{}, err
	}
	return s.technical.GetMaterialSpec(ctx, id)
}
func (s *Service) InstallSpool(ctx context.Context, a admindomain.Session, printer, spool string, slot int) (SpoolInstallationView, error) {
	if err := require(canManage(a)); err != nil {
		return SpoolInstallationView{}, err
	}
	return s.technical.InstallSpool(ctx, printer, spool, slot, a.UserID)
}
func (s *Service) RemoveSpool(ctx context.Context, a admindomain.Session, id, reason string) error {
	if err := require(canManage(a)); err != nil {
		return err
	}
	return s.technical.RemoveSpool(ctx, id, reason, a.UserID)
}
func (s *Service) ListSpoolInstallations(ctx context.Context, a admindomain.Session, printer string, history bool) ([]SpoolInstallationView, error) {
	if err := require(canRead(a)); err != nil {
		return nil, err
	}
	return s.technical.ListSpoolInstallations(ctx, printer, history)
}
func (s *Service) CheckTechnicalCompatibility(ctx context.Context, a admindomain.Session, in TechnicalCompatibilityRequest) (domain.CompatibilityResult, error) {
	if err := require(canRead(a)); err != nil {
		return domain.CompatibilityResult{}, err
	}
	return s.technical.TechnicalCompatibility(ctx, in)
}
