import type { CartItem } from "@/types/product";
import type { CartLine } from "@/services/customer-api";

export const CART_STORAGE_KEY = "loja-3d-demo-cart";
export const MAX_CART_QUANTITY = 100;

export type CartState = { items: CartItem[]; isOpen: boolean; announcement: string };
export type CartAction =
  | { type: "hydrate"; items: CartItem[] }
  | { type: "add"; item: CartItem }
  | { type: "quantity"; variantId: string; quantity: number }
  | { type: "remove"; variantId: string }
  | { type: "open" }
  | { type: "close" }
  | { type: "clear" };

const quantity = (value: number) => Math.min(MAX_CART_QUANTITY, Math.max(1, Math.trunc(value)));

export function sanitizeCart(value: unknown): CartItem[] {
  if (!Array.isArray(value)) return [];
  const items = new Map<string, CartItem>();
  value.forEach((candidate) => {
    if (!candidate || typeof candidate !== "object") return;
    const raw = candidate as Record<string, unknown>;
    if (typeof raw.productVariantId !== "string" || !raw.productVariantId.trim() || typeof raw.quantity !== "number" || !Number.isFinite(raw.quantity)) return;
    const item: CartItem = {
      productVariantId: raw.productVariantId,
      quantity: quantity(raw.quantity),
      ...(typeof raw.productId === "string" ? { productId: raw.productId } : {}),
      ...(typeof raw.productSlug === "string" ? { productSlug: raw.productSlug } : {}),
      ...(typeof raw.imageUrl === "string" && raw.imageUrl.startsWith("/") ? { imageUrl: raw.imageUrl } : {}),
      ...(typeof raw.imageAlt === "string" ? { imageAlt: raw.imageAlt } : {}),
      ...(typeof raw.variantLabel === "string" ? { variantLabel: raw.variantLabel } : {}),
      ...(typeof raw.priceSnapshotCents === "number" && Number.isInteger(raw.priceSnapshotCents) && raw.priceSnapshotCents >= 0 ? { priceSnapshotCents: raw.priceSnapshotCents } : {}),
    };
    const previous = items.get(item.productVariantId);
    items.set(item.productVariantId, previous ? { ...previous, ...item, quantity: quantity(previous.quantity + item.quantity) } : item);
  });
  return [...items.values()];
}

export function parseStoredCart(raw: string | null): CartItem[] {
  if (!raw) return [];
  try { return sanitizeCart(JSON.parse(raw)); } catch { return []; }
}

export function serializeCart(items: CartItem[]) { return JSON.stringify(sanitizeCart(items)); }

export function cartReducer(state: CartState, action: CartAction): CartState {
  switch (action.type) {
    case "hydrate": return { ...state, items: sanitizeCart(action.items) };
    case "add": {
      const incoming = sanitizeCart([action.item])[0];
      if (!incoming) return state;
      const current = state.items.find((item) => item.productVariantId === incoming.productVariantId);
      const items = current
        ? state.items.map((item) => item.productVariantId === incoming.productVariantId ? { ...item, ...incoming, quantity: quantity(item.quantity + incoming.quantity) } : item)
        : [...state.items, incoming];
      return { items, isOpen: true, announcement: `${incoming.variantLabel || "Produto"} adicionado ao carrinho.` };
    }
    case "quantity": return { ...state, items: state.items.map((item) => item.productVariantId === action.variantId ? { ...item, quantity: quantity(action.quantity) } : item), announcement: "Quantidade atualizada." };
    case "remove": return { ...state, items: state.items.filter((item) => item.productVariantId !== action.variantId), announcement: "Produto removido do carrinho." };
    case "open": return { ...state, isOpen: true };
    case "close": return { ...state, isOpen: false };
    case "clear": return { items: [], isOpen: false, announcement: "Carrinho limpo." };
  }
}

export function priceChanges(items: CartItem[], lines: CartLine[]) {
  return lines.filter((line) => {
    const snapshot = items.find((item) => item.productVariantId === line.VariantID)?.priceSnapshotCents;
    return snapshot !== undefined && snapshot !== line.UnitPriceCents;
  }).map((line) => line.VariantID);
}
