package catalog

import "errors"

var ErrProductNotFound = errors.New("product not found")

type Category struct {
	ID          string
	Slug        string
	Name        string
	Description string
}

type Material struct {
	ID   string
	Code string
	Name string
}

type Color struct {
	ID      string
	Slug    string
	Name    string
	HexCode string
}

type Size struct {
	ID       string
	Code     string
	Name     string
	WidthMM  *int32
	HeightMM *int32
	DepthMM  *int32
}

type Finish struct {
	ID          string
	Code        string
	Name        string
	Description string
}

type Dimensions struct {
	WidthMM  *int32
	HeightMM *int32
	DepthMM  *int32
}

type ProductVariant struct {
	ID                string
	SKU               string
	Name              string
	PriceCents        int64
	StockQuantity     int32
	AvailableQuantity int32
	TrackInventory    bool
	Dimensions        Dimensions
	WeightGrams       *int32
	LeadTimeMinDays   int32
	LeadTimeMaxDays   int32
	Material          Material
	Color             Color
	Size              *Size
	Finish            *Finish
}

func (variant ProductVariant) IsCommercial() bool {
	return variant.PriceCents >= 0 && (!variant.TrackInventory || variant.AvailableQuantity > 0)
}

type ProductImage struct {
	ID        string
	VariantID *string
	URL       string
	AltText   string
	MediaType string
	SortOrder int32
	IsPrimary bool
}

type Product struct {
	ID               string
	Slug             string
	Name             string
	ShortDescription string
	Description      string
	IsFeatured       bool
	ProductionType   string
	Category         Category
	Images           []ProductImage
	Variants         []ProductVariant
}

func (product Product) MinimumPriceCents() (int64, bool) {
	var minimum int64
	found := false
	for _, variant := range product.Variants {
		if !variant.IsCommercial() {
			continue
		}
		if !found || variant.PriceCents < minimum {
			minimum = variant.PriceCents
			found = true
		}
	}
	return minimum, found
}
