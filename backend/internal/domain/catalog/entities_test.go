package catalog

import "testing"

func TestMinimumPriceCentsUsesCommercialVariants(t *testing.T) {
	t.Parallel()

	product := Product{Variants: []ProductVariant{
		{PriceCents: 5000, TrackInventory: true, AvailableQuantity: 0},
		{PriceCents: 7500, TrackInventory: true, AvailableQuantity: 2},
		{PriceCents: 9000, TrackInventory: false},
	}}

	price, ok := product.MinimumPriceCents()
	if !ok || price != 7500 {
		t.Fatalf("expected minimum commercial price 7500, got %d, %t", price, ok)
	}
}

func TestVariantWithoutInventoryTrackingIsCommercial(t *testing.T) {
	t.Parallel()

	variant := ProductVariant{PriceCents: 1000, TrackInventory: false}
	if !variant.IsCommercial() {
		t.Fatal("expected made-to-order variant to be commercial")
	}
}
