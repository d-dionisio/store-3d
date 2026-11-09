package handlers

import (
	domain "example.com/loja-3d/backend/internal/domain/catalog"
)

type categoryDTO struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type materialDTO struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type colorDTO struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	HexCode string `json:"hex_code"`
}

type sizeDTO struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	WidthMM  *int32 `json:"width_mm,omitempty"`
	HeightMM *int32 `json:"height_mm,omitempty"`
	DepthMM  *int32 `json:"depth_mm,omitempty"`
}

type finishDTO struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type dimensionsDTO struct {
	WidthMM  *int32 `json:"width_mm,omitempty"`
	HeightMM *int32 `json:"height_mm,omitempty"`
	DepthMM  *int32 `json:"depth_mm,omitempty"`
}

type variantDTO struct {
	ID                 string         `json:"id"`
	SKU                string         `json:"sku"`
	Name               string         `json:"name,omitempty"`
	PriceCents         int64          `json:"price_cents"`
	AvailableQuantity  int32          `json:"available_quantity"`
	TrackInventory     bool           `json:"track_inventory"`
	WeightGrams        *int32         `json:"weight_grams,omitempty"`
	Dimensions         dimensionsDTO  `json:"dimensions"`
	ProductionLeadTime productionTime `json:"production_lead_time"`
	Material           materialDTO    `json:"material"`
	Color              colorDTO       `json:"color"`
	Size               *sizeDTO       `json:"size,omitempty"`
	Finish             *finishDTO     `json:"finish,omitempty"`
}

type productionTime struct {
	MinDays int32 `json:"min_days"`
	MaxDays int32 `json:"max_days"`
}

type productImageDTO struct {
	ID        string  `json:"id"`
	VariantID *string `json:"variant_id,omitempty"`
	URL       string  `json:"url"`
	AltText   string  `json:"alt_text"`
	MediaType string  `json:"media_type"`
	SortOrder int32   `json:"sort_order"`
	IsPrimary bool    `json:"is_primary"`
}

type productDTO struct {
	ID               string            `json:"id"`
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	ShortDescription string            `json:"short_description"`
	Description      string            `json:"description"`
	IsFeatured       bool              `json:"is_featured"`
	ProductionType   string            `json:"production_type"`
	MinPriceCents    int64             `json:"min_price_cents"`
	Category         categoryDTO       `json:"category"`
	Images           []productImageDTO `json:"images"`
	Materials        []materialDTO     `json:"materials"`
	Colors           []colorDTO        `json:"colors"`
	Variants         []variantDTO      `json:"variants"`
}

func toProductDTO(product domain.Product) productDTO {
	result := productDTO{
		ID: product.ID, Slug: product.Slug, Name: product.Name,
		ShortDescription: product.ShortDescription, Description: product.Description,
		IsFeatured: product.IsFeatured, ProductionType: product.ProductionType,
		Category: categoryDTO{ID: product.Category.ID, Slug: product.Category.Slug, Name: product.Category.Name, Description: product.Category.Description},
		Images:   make([]productImageDTO, 0, len(product.Images)), Materials: make([]materialDTO, 0),
		Colors: make([]colorDTO, 0), Variants: make([]variantDTO, 0, len(product.Variants)),
	}
	result.MinPriceCents, _ = product.MinimumPriceCents()

	for _, image := range product.Images {
		result.Images = append(result.Images, productImageDTO{ID: image.ID, VariantID: image.VariantID, URL: image.URL, AltText: image.AltText, MediaType: image.MediaType, SortOrder: image.SortOrder, IsPrimary: image.IsPrimary})
	}

	materialSeen := make(map[string]bool)
	colorSeen := make(map[string]bool)
	for _, variant := range product.Variants {
		material := materialDTO{ID: variant.Material.ID, Code: variant.Material.Code, Name: variant.Material.Name}
		color := colorDTO{ID: variant.Color.ID, Slug: variant.Color.Slug, Name: variant.Color.Name, HexCode: variant.Color.HexCode}
		if !materialSeen[material.ID] {
			result.Materials = append(result.Materials, material)
			materialSeen[material.ID] = true
		}
		if !colorSeen[color.ID] {
			result.Colors = append(result.Colors, color)
			colorSeen[color.ID] = true
		}

		dto := variantDTO{
			ID: variant.ID, SKU: variant.SKU, Name: variant.Name, PriceCents: variant.PriceCents,
			AvailableQuantity: variant.AvailableQuantity, TrackInventory: variant.TrackInventory,
			WeightGrams:        variant.WeightGrams,
			Dimensions:         dimensionsDTO{WidthMM: variant.Dimensions.WidthMM, HeightMM: variant.Dimensions.HeightMM, DepthMM: variant.Dimensions.DepthMM},
			ProductionLeadTime: productionTime{MinDays: variant.LeadTimeMinDays, MaxDays: variant.LeadTimeMaxDays},
			Material:           material, Color: color,
		}
		if variant.Size != nil {
			dto.Size = &sizeDTO{ID: variant.Size.ID, Code: variant.Size.Code, Name: variant.Size.Name, WidthMM: variant.Size.WidthMM, HeightMM: variant.Size.HeightMM, DepthMM: variant.Size.DepthMM}
		}
		if variant.Finish != nil {
			dto.Finish = &finishDTO{ID: variant.Finish.ID, Code: variant.Finish.Code, Name: variant.Finish.Name, Description: variant.Finish.Description}
		}
		result.Variants = append(result.Variants, dto)
	}
	return result
}
