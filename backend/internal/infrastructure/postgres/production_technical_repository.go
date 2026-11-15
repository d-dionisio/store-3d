package postgres

import (
	"context"
	"encoding/json"
	"errors"
	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"github.com/jackc/pgx/v5"
	"time"
)

const printerSpecSelect = `SELECT s.id,s.printer_id,s.status,s.technology,coalesce(s.technical_notes,''),coalesce(s.source_reference,''),s.version,coalesce((SELECT version FROM printer_operational_configs WHERE printer_id=s.printer_id),0),s.max_width_mm,s.max_depth_mm,s.max_height_mm,s.max_hotend_temp_c,s.max_bed_temp_c,s.enclosed_chamber,s.created_at,coalesce((SELECT array_agg(material_id::text ORDER BY material_id) FROM printer_supported_materials WHERE printer_spec_id=s.id),ARRAY[]::text[]),coalesce((SELECT jsonb_agg(jsonb_build_object('diameter_microns',diameter_microns,'type',nozzle_type) ORDER BY diameter_microns,nozzle_type)::text FROM printer_supported_nozzles WHERE printer_spec_id=s.id),'[]'),coalesce((SELECT array_agg(surface ORDER BY surface) FROM printer_build_surfaces WHERE printer_spec_id=s.id),ARRAY[]::text[]),coalesce((SELECT array_agg(feature ORDER BY feature) FROM printer_features WHERE printer_spec_id=s.id),ARRAY[]::text[]) FROM printer_technical_specs s`

func scanPrinterSpec(row pgx.Row) (app.PrinterSpecView, error) {
	var v app.PrinterSpecView
	var nozzles string
	err := row.Scan(&v.ID, &v.PrinterID, &v.Status, &v.Technology, &v.TechnicalNotes, &v.SourceReference, &v.Version, &v.OperationalConfigVersion, &v.MaxWidthMM, &v.MaxDepthMM, &v.MaxHeightMM, &v.MaxHotendTempC, &v.MaxBedTempC, &v.EnclosedChamber, &v.CreatedAt, &v.SupportedMaterialIDs, &nozzles, &v.Surfaces, &v.Features)
	if err == nil {
		err = json.Unmarshal([]byte(nozzles), &v.SupportedNozzles)
	}
	return v, productionError(err)
}
func (r *ProductionRepository) SavePrinterSpec(ctx context.Context, printerID string, in app.PrinterSpecInput, actor string) (app.PrinterSpecView, error) {
	spec := domain.PrinterTechnicalSpec{PrinterID: printerID, Status: in.Status, Technology: in.Technology, Version: in.ExpectedVersion, MaxWidthMM: in.MaxWidthMM, MaxDepthMM: in.MaxDepthMM, MaxHeightMM: in.MaxHeightMM, MaxHotendTempC: in.MaxHotendTempC, MaxBedTempC: in.MaxBedTempC, EnclosedChamber: in.EnclosedChamber, SupportedMaterials: in.SupportedMaterialIDs, SupportedNozzles: in.SupportedNozzles}
	if err := spec.Validate(); err != nil {
		return app.PrinterSpecView{}, app.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.PrinterSpecView{}, err
	}
	defer tx.Rollback(ctx)
	var currentID string
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT id,version FROM printer_technical_specs WHERE printer_id=$1 AND status IN ('draft','verified') FOR UPDATE`, printerID).Scan(&currentID, &currentVersion)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return app.PrinterSpecView{}, productionError(err)
	}
	if currentID != "" && currentVersion != in.ExpectedVersion {
		return app.PrinterSpecView{}, app.ErrConflict
	}
	next := int64(1)
	if currentID != "" {
		next = currentVersion + 1
		if _, err = tx.Exec(ctx, `UPDATE printer_technical_specs SET status='superseded' WHERE id=$1`, currentID); err != nil {
			return app.PrinterSpecView{}, productionError(err)
		}
	}
	verifiedAt := any(nil)
	if in.Status == "verified" {
		verifiedAt = time.Now().UTC()
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO printer_technical_specs(printer_id,version,status,technology,max_width_mm,max_depth_mm,max_height_mm,max_hotend_temp_c,max_bed_temp_c,enclosed_chamber,technical_notes,source_reference,source_verified_at,created_by_admin_user_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,nullif($12,''),$13,$14)RETURNING id`, printerID, next, in.Status, in.Technology, in.MaxWidthMM, in.MaxDepthMM, in.MaxHeightMM, in.MaxHotendTempC, in.MaxBedTempC, in.EnclosedChamber, in.TechnicalNotes, in.SourceReference, verifiedAt, actor).Scan(&id)
	if err != nil {
		return app.PrinterSpecView{}, productionError(err)
	}
	for _, m := range in.SupportedMaterialIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO printer_supported_materials(printer_spec_id,material_id)VALUES($1,$2)`, id, m); err != nil {
			return app.PrinterSpecView{}, productionError(err)
		}
	}
	for _, n := range in.SupportedNozzles {
		if _, err = tx.Exec(ctx, `INSERT INTO printer_supported_nozzles(printer_spec_id,diameter_microns,nozzle_type)VALUES($1,$2,$3)`, id, n.DiameterMicrons, n.Type); err != nil {
			return app.PrinterSpecView{}, productionError(err)
		}
	}
	for _, surface := range in.Surfaces {
		if _, err = tx.Exec(ctx, `INSERT INTO printer_build_surfaces(printer_spec_id,surface)VALUES($1,$2)`, id, surface); err != nil {
			return app.PrinterSpecView{}, productionError(err)
		}
	}
	for _, feature := range in.Features {
		if _, err = tx.Exec(ctx, `INSERT INTO printer_features(printer_spec_id,feature)VALUES($1,$2)`, id, feature); err != nil {
			return app.PrinterSpecView{}, productionError(err)
		}
	}
	v, err := scanPrinterSpec(tx.QueryRow(ctx, printerSpecSelect+` WHERE s.id=$1`, id))
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (r *ProductionRepository) GetPrinterSpec(ctx context.Context, id string) (app.PrinterSpecView, error) {
	return scanPrinterSpec(r.pool.QueryRow(ctx, printerSpecSelect+` WHERE s.printer_id=$1 AND s.status IN ('draft','verified')`, id))
}
func (r *ProductionRepository) ListPrinterSpecs(ctx context.Context) ([]app.PrinterSpecView, error) {
	rows, err := r.pool.Query(ctx, printerSpecSelect+` WHERE s.status IN ('draft','verified') ORDER BY s.created_at DESC`)
	if err != nil {
		return nil, productionError(err)
	}
	defer rows.Close()
	out := []app.PrinterSpecView{}
	for rows.Next() {
		v, e := scanPrinterSpec(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ProductionRepository) PrinterSpecHistory(ctx context.Context, id string) ([]app.PrinterSpecView, error) {
	rows, err := r.pool.Query(ctx, printerSpecSelect+` WHERE s.printer_id=$1 ORDER BY s.version DESC`, id)
	if err != nil {
		return nil, productionError(err)
	}
	defer rows.Close()
	out := []app.PrinterSpecView{}
	for rows.Next() {
		v, scanErr := scanPrinterSpec(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ProductionRepository) SaveOperationalConfig(ctx context.Context, id string, in app.OperationalConfigInput, actor string) error {
	tag, err := r.pool.Exec(ctx, `INSERT INTO printer_operational_configs(printer_id,status,installed_nozzle_microns,installed_nozzle_type,installed_surface,material_slot_count,updated_by_admin_user_id)VALUES($1,$2,$3,nullif($4,''),nullif($5,''),$6,$7) ON CONFLICT(printer_id)DO UPDATE SET status=excluded.status,installed_nozzle_microns=excluded.installed_nozzle_microns,installed_nozzle_type=excluded.installed_nozzle_type,installed_surface=excluded.installed_surface,material_slot_count=excluded.material_slot_count,updated_by_admin_user_id=excluded.updated_by_admin_user_id,version=printer_operational_configs.version+1,updated_at=now() WHERE printer_operational_configs.version=$8`, id, in.Status, in.InstalledNozzleMicrons, in.InstalledNozzleType, in.InstalledSurface, in.MaterialSlotCount, actor, in.ExpectedVersion)
	if err != nil {
		return productionError(err)
	}
	if tag.RowsAffected() != 1 {
		return app.ErrConflict
	}
	return nil
}

const materialSpecSelect = `SELECT m.id,m.name,coalesce(s.manufacturer,''),coalesce(s.commercial_name,''),coalesce(s.polymer_type,''),s.print_temp_min_c,s.print_temp_max_c,s.print_temp_recommended_c,s.bed_temp_min_c,s.bed_temp_max_c,s.bed_temp_recommended_c,s.recommended_speed_mm_s,s.requires_enclosure,s.requires_drying,coalesce(s.moisture_sensitivity,''),coalesce(s.required_surface,''),coalesce(s.notes,''),coalesce(s.status,'unconfigured'),coalesce(s.source_reference,''),coalesce(s.version,0) FROM materials m LEFT JOIN material_technical_specs s ON s.material_id=m.id`

func scanMaterialSpec(row pgx.Row) (app.MaterialSpecView, error) {
	var v app.MaterialSpecView
	err := row.Scan(&v.MaterialID, &v.MaterialName, &v.Manufacturer, &v.CommercialName, &v.PolymerType, &v.PrintTempMinC, &v.PrintTempMaxC, &v.PrintTempRecommendedC, &v.BedTempMinC, &v.BedTempMaxC, &v.BedTempRecommendedC, &v.RecommendedSpeedMMS, &v.RequiresEnclosure, &v.RequiresDrying, &v.MoistureSensitivity, &v.RequiredSurface, &v.Notes, &v.Status, &v.SourceReference, &v.Version)
	return v, productionError(err)
}
func (r *ProductionRepository) ListMaterialSpecs(ctx context.Context) ([]app.MaterialSpecView, error) {
	rows, err := r.pool.Query(ctx, materialSpecSelect+` ORDER BY m.name`)
	if err != nil {
		return nil, productionError(err)
	}
	defer rows.Close()
	out := []app.MaterialSpecView{}
	for rows.Next() {
		v, e := scanMaterialSpec(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ProductionRepository) GetMaterialSpec(ctx context.Context, id string) (app.MaterialSpecView, error) {
	return scanMaterialSpec(r.pool.QueryRow(ctx, materialSpecSelect+` WHERE m.id=$1`, id))
}
func (r *ProductionRepository) SaveMaterialSpec(ctx context.Context, id string, in app.MaterialSpecInput, actor string) (app.MaterialSpecView, error) {
	spec := domain.MaterialTechnicalSpec{MaterialID: id, PrintTempMinC: in.PrintTempMinC, PrintTempMaxC: in.PrintTempMaxC, PrintTempRecommendedC: in.PrintTempRecommendedC, BedTempMinC: in.BedTempMinC, BedTempMaxC: in.BedTempMaxC, BedTempRecommendedC: in.BedTempRecommendedC, Version: in.ExpectedVersion}
	if spec.Validate() != nil {
		return app.MaterialSpecView{}, app.ErrInvalid
	}
	verifiedAt := any(nil)
	if in.Status == "verified" {
		verifiedAt = time.Now().UTC()
	}
	tag, err := r.pool.Exec(ctx, `INSERT INTO material_technical_specs(material_id,manufacturer,commercial_name,polymer_type,print_temp_min_c,print_temp_max_c,print_temp_recommended_c,bed_temp_min_c,bed_temp_max_c,bed_temp_recommended_c,recommended_speed_mm_s,requires_enclosure,requires_drying,moisture_sensitivity,required_surface,notes,status,source_reference,source_verified_at,updated_by_admin_user_id)VALUES($1,nullif($2,''),nullif($3,''),nullif($4,''),$5,$6,$7,$8,$9,$10,$11,$12,$13,nullif($14,''),nullif($15,''),nullif($16,''),$17,nullif($18,''),$19,$20) ON CONFLICT(material_id)DO UPDATE SET manufacturer=excluded.manufacturer,commercial_name=excluded.commercial_name,polymer_type=excluded.polymer_type,print_temp_min_c=excluded.print_temp_min_c,print_temp_max_c=excluded.print_temp_max_c,print_temp_recommended_c=excluded.print_temp_recommended_c,bed_temp_min_c=excluded.bed_temp_min_c,bed_temp_max_c=excluded.bed_temp_max_c,bed_temp_recommended_c=excluded.bed_temp_recommended_c,recommended_speed_mm_s=excluded.recommended_speed_mm_s,requires_enclosure=excluded.requires_enclosure,requires_drying=excluded.requires_drying,moisture_sensitivity=excluded.moisture_sensitivity,required_surface=excluded.required_surface,notes=excluded.notes,status=excluded.status,source_reference=excluded.source_reference,source_verified_at=excluded.source_verified_at,updated_by_admin_user_id=excluded.updated_by_admin_user_id,version=material_technical_specs.version+1,updated_at=now() WHERE material_technical_specs.version=$21`, id, in.Manufacturer, in.CommercialName, in.PolymerType, in.PrintTempMinC, in.PrintTempMaxC, in.PrintTempRecommendedC, in.BedTempMinC, in.BedTempMaxC, in.BedTempRecommendedC, in.RecommendedSpeedMMS, in.RequiresEnclosure, in.RequiresDrying, in.MoistureSensitivity, in.RequiredSurface, in.Notes, in.Status, in.SourceReference, verifiedAt, actor, in.ExpectedVersion)
	if err != nil {
		return app.MaterialSpecView{}, productionError(err)
	}
	if tag.RowsAffected() != 1 {
		return app.MaterialSpecView{}, app.ErrConflict
	}
	return r.GetMaterialSpec(ctx, id)
}
func (r *ProductionRepository) InstallSpool(ctx context.Context, printer, spool string, slot int, actor string) (app.SpoolInstallationView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.SpoolInstallationView{}, err
	}
	defer tx.Rollback(ctx)
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT ms.status NOT IN ('quarantined','discarded','depleted') AND poc.status='configured' AND $3<=poc.material_slot_count AND EXISTS(SELECT 1 FROM printer_technical_specs pts JOIN printer_supported_materials psm ON psm.printer_spec_id=pts.id WHERE pts.printer_id=$1 AND pts.status='verified' AND psm.material_id=ms.material_id) FROM material_spools ms JOIN printer_operational_configs poc ON poc.printer_id=$1 WHERE ms.id=$2 FOR UPDATE OF ms,poc`, printer, spool, slot).Scan(&allowed)
	if err != nil || !allowed {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.SpoolInstallationView{}, app.ErrInvalid
		}
		if err != nil {
			return app.SpoolInstallationView{}, productionError(err)
		}
		return app.SpoolInstallationView{}, app.ErrInvalid
	}
	var id string
	if err = tx.QueryRow(ctx, `INSERT INTO spool_installations(printer_id,material_spool_id,slot_number,installed_by_admin_user_id)VALUES($1,$2,$3,$4)RETURNING id`, printer, spool, slot, actor).Scan(&id); err != nil {
		return app.SpoolInstallationView{}, productionError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return app.SpoolInstallationView{}, err
	}
	list, err := r.ListSpoolInstallations(ctx, printer, false)
	for _, v := range list {
		if v.ID == id {
			return v, err
		}
	}
	return app.SpoolInstallationView{}, app.ErrNotFound
}
func (r *ProductionRepository) RemoveSpool(ctx context.Context, id, reason, actor string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE spool_installations SET removed_at=now(),removed_by_admin_user_id=$2,removal_reason=$3 WHERE id=$1 AND removed_at IS NULL`, id, actor, reason)
	if err != nil {
		return productionError(err)
	}
	if tag.RowsAffected() != 1 {
		return app.ErrConflict
	}
	return nil
}
func (r *ProductionRepository) ListSpoolInstallations(ctx context.Context, printer string, history bool) ([]app.SpoolInstallationView, error) {
	rows, err := r.pool.Query(ctx, `SELECT si.id,si.printer_id,p.name,si.material_spool_id,ms.spool_code,ms.material_id,m.name,ms.color_id,c.name,si.slot_number,si.installed_at,si.removed_at FROM spool_installations si JOIN printers p ON p.id=si.printer_id JOIN material_spools ms ON ms.id=si.material_spool_id JOIN materials m ON m.id=ms.material_id JOIN colors c ON c.id=ms.color_id WHERE ($1='' OR si.printer_id=$1::uuid) AND ($2 OR si.removed_at IS NULL) ORDER BY si.installed_at DESC`, printer, history)
	if err != nil {
		return nil, productionError(err)
	}
	defer rows.Close()
	out := []app.SpoolInstallationView{}
	for rows.Next() {
		var v app.SpoolInstallationView
		if err = rows.Scan(&v.ID, &v.PrinterID, &v.PrinterName, &v.SpoolID, &v.SpoolCode, &v.MaterialID, &v.MaterialName, &v.ColorID, &v.ColorName, &v.SlotNumber, &v.InstalledAt, &v.RemovedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ProductionRepository) TechnicalCompatibility(ctx context.Context, in app.TechnicalCompatibilityRequest) (domain.CompatibilityResult, error) {
	var job domain.OptimizationJob
	job.MaterialID = in.MaterialID
	job.ColorID = in.ColorID
	job.WidthMM = in.WidthMM
	job.DepthMM = in.DepthMM
	job.HeightMM = in.HeightMM
	var printer domain.OptimizationPrinter
	var materials []string
	var nozzles []int32
	err := r.pool.QueryRow(ctx, `SELECT p.id,p.status,least(p.build_width_mm,coalesce(s.max_width_mm,p.build_width_mm)),least(p.build_depth_mm,coalesce(s.max_depth_mm,p.build_depth_mm)),least(p.build_height_mm,coalesce(s.max_height_mm,p.build_height_mm)),s.max_hotend_temp_c,s.max_bed_temp_c,s.enclosed_chamber,cfg.installed_nozzle_microns,coalesce(cfg.installed_nozzle_type,''),coalesce(cfg.installed_surface,''),coalesce(s.status='verified',false),coalesce(cfg.status='configured',false),coalesce((SELECT array_agg(material_id::text) FROM printer_supported_materials WHERE printer_spec_id=s.id),ARRAY[]::text[]),coalesce((SELECT array_agg(DISTINCT diameter_microns) FROM printer_supported_nozzles WHERE printer_spec_id=s.id),ARRAY[]::integer[]),mts.print_temp_recommended_c,mts.bed_temp_recommended_c,mts.requires_enclosure,coalesce(mts.required_surface,''),coalesce(mts.status='verified',false) FROM printers p LEFT JOIN printer_technical_specs s ON s.printer_id=p.id AND s.status IN ('draft','verified') LEFT JOIN printer_operational_configs cfg ON cfg.printer_id=p.id LEFT JOIN material_technical_specs mts ON mts.material_id=$2 WHERE p.id=$1`, in.PrinterID, in.MaterialID).Scan(&printer.ID, &printer.Status, &printer.BuildWidthMM, &printer.BuildDepthMM, &printer.BuildHeightMM, &printer.MaxHotendTempC, &printer.MaxBedTempC, &printer.EnclosedChamber, &printer.InstalledNozzleMicrons, &printer.InstalledNozzleType, &printer.InstalledSurface, &printer.TechnicalSpecsVerified, &printer.OperationalConfigReady, &materials, &nozzles, &job.PrintTempC, &job.BedTempC, &job.RequiresEnclosure, &job.RequiredSurface, &job.MaterialSpecsVerified)
	if err != nil {
		return domain.CompatibilityResult{}, productionError(err)
	}
	printer.SupportedMaterialIDs = map[string]bool{}
	for _, v := range materials {
		printer.SupportedMaterialIDs[v] = true
	}
	printer.SupportedNozzles = map[int]bool{}
	for _, v := range nozzles {
		printer.SupportedNozzles[int(v)] = true
	}
	return domain.EvaluateCompatibility(job, printer), nil
}
