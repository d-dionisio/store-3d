package shipping

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var postalCodePattern = regexp.MustCompile(`^\d{8}$`)

type Package struct {
	Quantity    int `json:"quantity"`
	WeightGrams int `json:"weight_grams"`
	LengthMM    int `json:"length_mm"`
	WidthMM     int `json:"width_mm"`
	HeightMM    int `json:"height_mm"`
}

func (p Package) Validate() error {
	if p.Quantity < 1 || p.Quantity > 20 || p.WeightGrams < 1 || p.WeightGrams > 100000 ||
		p.LengthMM < 10 || p.LengthMM > 2000 || p.WidthMM < 10 || p.WidthMM > 2000 || p.HeightMM < 10 || p.HeightMM > 2000 {
		return errors.New("invalid package")
	}
	return nil
}

func NormalizePostalCode(value string) (string, error) {
	value = strings.NewReplacer("-", "", " ", "", ".", "").Replace(value)
	if !postalCodePattern.MatchString(value) {
		return "", errors.New("invalid postal code")
	}
	return value, nil
}

type QuoteRequest struct {
	OrderID, Reference, OriginPostalCode, DestinationPostalCode string
	Packages                                                    []Package
	InsuranceValueCents                                         int64
}

type ProviderQuote struct {
	ServiceCode, CarrierName, ServiceName string
	PriceCents                            int64
	MinDays, MaxDays                      int
	PackageSnapshot                       any
}

type Quote struct {
	ID, ServiceCode, CarrierName, ServiceName string
	PriceCents                                int64
	MinDays, MaxDays                          int
	ExpiresAt                                 time.Time
}

type Selection struct {
	QuoteID, OrderID, ServiceCode, CarrierName, ServiceName string
	PriceCents                                              int64
	MinDays, MaxDays                                        int
	ExpiresAt                                               time.Time
}
