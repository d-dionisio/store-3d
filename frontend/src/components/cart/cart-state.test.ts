import { describe, expect, it } from "vitest";
import { cartReducer, parseStoredCart, priceChanges, serializeCart, type CartState } from "./cart-state";

const initial: CartState = { items: [], isOpen: false, announcement: "" };
const item = { productVariantId: "variant-1", quantity: 1, productSlug: "vaso", variantLabel: "Vaso areia" };

describe("cart state", () => {
  it("adds once, opens automatically and merges duplicates", () => {
    const added = cartReducer(initial, { type: "add", item });
    expect(added.isOpen).toBe(true);
    expect(added.items).toEqual([item]);
    const repeated = cartReducer(added, { type: "add", item: { ...item, quantity: 2 } });
    expect(repeated.items[0].quantity).toBe(3);
  });

  it("increments, decrements and removes without accepting zero", () => {
    const added = cartReducer(initial, { type: "add", item });
    expect(cartReducer(added, { type: "quantity", variantId: "variant-1", quantity: 2 }).items[0].quantity).toBe(2);
    expect(cartReducer(added, { type: "quantity", variantId: "variant-1", quantity: 0 }).items[0].quantity).toBe(1);
    expect(cartReducer(added, { type: "remove", variantId: "variant-1" }).items).toHaveLength(0);
  });

  it("persists safe metadata and hydrates cross-tab state", () => {
    const stored = serializeCart([{ ...item, imageUrl: "/images/product.png", priceSnapshotCents: 1000 }]);
    expect(parseStoredCart(stored)).toEqual([{ ...item, imageUrl: "/images/product.png", priceSnapshotCents: 1000 }]);
    expect(parseStoredCart("invalid")).toEqual([]);
  });

  it("detects a price change only against a previous server snapshot", () => {
    expect(priceChanges([{ ...item, priceSnapshotCents: 1000 }], [{ VariantID: "variant-1", ProductName: "Vaso", SKU: "V1", Material: "PLA", Color: "Areia", Quantity: 1, AvailableQuantity: 5, UnitPriceCents: 1200, LineTotalCents: 1200 }])).toEqual(["variant-1"]);
  });
});
