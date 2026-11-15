package production

import "errors"

var ErrInvalidTechnicalSpecification = errors.New("invalid technical specification")

type PrinterTechnicalSpec struct {
	ID, PrinterID, Status, Technology, Notes, SourceReference        string
	Version                                                          int64
	MaxWidthMM, MaxDepthMM, MaxHeightMM, MaxHotendTempC, MaxBedTempC *int
	EnclosedChamber                                                  *bool
	SupportedMaterials                                               []string
	SupportedNozzles                                                 []NozzleOption
	Surfaces, Features                                               []string
}
type NozzleOption struct {
	DiameterMicrons int    `json:"diameter_microns"`
	Type            string `json:"type"`
}

func (p PrinterTechnicalSpec) Validate() error {
	if p.PrinterID == "" || p.Technology != "fdm" || p.Version < 0 {
		return ErrInvalidTechnicalSpecification
	}
	dims := []*int{p.MaxWidthMM, p.MaxDepthMM, p.MaxHeightMM}
	present := 0
	for _, v := range dims {
		if v != nil {
			if *v <= 0 {
				return ErrInvalidTechnicalSpecification
			}
			present++
		}
	}
	if present != 0 && present != 3 {
		return ErrInvalidTechnicalSpecification
	}
	for _, n := range p.SupportedNozzles {
		if n.DiameterMicrons <= 0 || n.Type == "" {
			return ErrInvalidTechnicalSpecification
		}
	}
	return nil
}

type MaterialTechnicalSpec struct {
	MaterialID, Manufacturer, CommercialName, PolymerType, MoistureSensitivity, RequiredSurface, Notes, Status, SourceReference string
	PrintTempMinC, PrintTempMaxC, PrintTempRecommendedC, BedTempMinC, BedTempMaxC, BedTempRecommendedC, RecommendedSpeedMMS     *int
	RequiresEnclosure, RequiresDrying                                                                                           *bool
	Version                                                                                                                     int64
}

func (m MaterialTechnicalSpec) Validate() error {
	if m.MaterialID == "" || m.Version < 0 {
		return ErrInvalidTechnicalSpecification
	}
	if !validRange(m.PrintTempMinC, m.PrintTempMaxC, m.PrintTempRecommendedC) || !validRange(m.BedTempMinC, m.BedTempMaxC, m.BedTempRecommendedC) {
		return ErrInvalidTechnicalSpecification
	}
	return nil
}
func validRange(min, max, recommended *int) bool {
	if min != nil && max != nil && *min > *max {
		return false
	}
	if recommended != nil && (min == nil || max == nil || *recommended < *min || *recommended > *max) {
		return false
	}
	return true
}

type PrinterOperationalConfig struct {
	PrinterID, Status, InstalledNozzleType, InstalledSurface string
	InstalledNozzleMicrons, MaterialSlotCount                *int
	Version                                                  int64
}

func (c PrinterOperationalConfig) Validate() error {
	if c.PrinterID == "" || c.Version < 0 {
		return ErrInvalidTechnicalSpecification
	}
	if c.Status == "configured" && (c.InstalledNozzleMicrons == nil || c.MaterialSlotCount == nil || c.InstalledNozzleType == "") {
		return ErrInvalidTechnicalSpecification
	}
	return nil
}
