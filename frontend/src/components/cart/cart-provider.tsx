"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { validateCart, type ValidatedCart } from "@/services/customer-api";
import type { CartItem } from "@/types/product";
import { CART_STORAGE_KEY, cartReducer, parseStoredCart, priceChanges, serializeCart } from "./cart-state";

type ValidationStatus = "idle" | "loading" | "ready" | "error";
type CartContextValue = {
  items: CartItem[]; count: number; ready: boolean; isOpen: boolean; announcement: string;
  validatedCart: ValidatedCart | null; validationStatus: ValidationStatus; validationError: string;
  unavailableVariantIds: string[]; changedPriceVariantIds: string[];
  addItem: (item: CartItem) => void; updateQuantity: (variantId: string, quantity: number) => void;
  removeItem: (variantId: string) => void; clear: () => void; openCart: () => void; closeCart: () => void;
  retryValidation: () => void;
};

const CartContext = createContext<CartContextValue | null>(null);

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = useReducer(cartReducer, { items: [], isOpen: false, announcement: "" });
  const [ready, setReady] = useState(false);
  const [validatedCart, setValidatedCart] = useState<ValidatedCart | null>(null);
  const [validationStatus, setValidationStatus] = useState<ValidationStatus>("idle");
  const [validationError, setValidationError] = useState("");
  const [unavailableVariantIds, setUnavailableVariantIds] = useState<string[]>([]);
  const [changedPriceVariantIds, setChangedPriceVariantIds] = useState<string[]>([]);
  const [validationAttempt, setValidationAttempt] = useState(0);
  const requestSequence = useRef(0);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      dispatch({ type: "hydrate", items: parseStoredCart(window.localStorage.getItem(CART_STORAGE_KEY)) });
      setReady(true);
    }, 0);
    const sync = (event: StorageEvent) => {
      if (event.key === CART_STORAGE_KEY) dispatch({ type: "hydrate", items: parseStoredCart(event.newValue) });
    };
    window.addEventListener("storage", sync);
    return () => { window.clearTimeout(timer); window.removeEventListener("storage", sync); };
  }, []);

  useEffect(() => {
    if (ready) window.localStorage.setItem(CART_STORAGE_KEY, serializeCart(state.items));
  }, [state.items, ready]);

  const validationKey = state.items.map((item) => `${item.productVariantId}:${item.quantity}`).sort().join("|");
  useEffect(() => {
    if (!ready) return;
    if (!state.items.length) {
      requestSequence.current += 1;
      return;
    }
    const sequence = ++requestSequence.current;
    const inputs = state.items.map((item) => ({ variant_id: item.productVariantId, quantity: item.quantity }));
    queueMicrotask(() => {
      if (requestSequence.current !== sequence) return;
      setValidationStatus("loading");
      setValidationError("");
    });
    validateCart(inputs).then((cart) => {
      if (sequence !== requestSequence.current) return;
      setChangedPriceVariantIds(priceChanges(state.items, cart.Items));
      setUnavailableVariantIds([]); setValidatedCart(cart); setValidationStatus("ready");
      dispatch({ type: "hydrate", items: state.items.map((item) => ({
        ...item,
        priceSnapshotCents: cart.Items.find((line) => line.VariantID === item.productVariantId)?.UnitPriceCents ?? item.priceSnapshotCents,
      })) });
    }).catch(async (cause: unknown) => {
      const results = await Promise.allSettled(inputs.map((input) => validateCart([input])));
      if (sequence !== requestSequence.current) return;
      const lines = results.flatMap((result) => result.status === "fulfilled" ? result.value.Items : []);
      const unavailable = results.flatMap((result, index) => result.status === "rejected" ? [inputs[index].variant_id] : []);
      setUnavailableVariantIds(unavailable);
      if (lines.length) {
        const cart = { Items: lines, SubtotalCents: lines.reduce((sum, line) => sum + line.LineTotalCents, 0) };
        setChangedPriceVariantIds(priceChanges(state.items, lines)); setValidatedCart(cart); setValidationStatus("ready");
      } else {
        setValidatedCart(null); setValidationStatus("error");
      }
      setValidationError(cause instanceof Error ? cause.message : "Não foi possível atualizar o carrinho.");
    });
  // Price/image snapshots are intentionally excluded from commercial revalidation.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, validationKey, validationAttempt]);

  const addItem = useCallback((item: CartItem) => dispatch({ type: "add", item }), []);
  const updateQuantity = useCallback((variantId: string, quantity: number) => dispatch({ type: "quantity", variantId, quantity }), []);
  const removeItem = useCallback((variantId: string) => dispatch({ type: "remove", variantId }), []);
  const clear = useCallback(() => dispatch({ type: "clear" }), []);
  const openCart = useCallback(() => { dispatch({ type: "open" }); setValidationAttempt((current) => current + 1); }, []);
  const closeCart = useCallback(() => dispatch({ type: "close" }), []);
  const retryValidation = useCallback(() => setValidationAttempt((current) => current + 1), []);

  const hasItems = state.items.length > 0;
  const value = useMemo(() => ({
    items: state.items, ready, isOpen: state.isOpen, announcement: state.announcement,
    count: state.items.reduce((sum, item) => sum + item.quantity, 0), validatedCart: hasItems ? validatedCart : null,
    validationStatus: hasItems ? validationStatus : "idle" as const,
    validationError: hasItems ? validationError : "", unavailableVariantIds: hasItems ? unavailableVariantIds : [],
    changedPriceVariantIds: hasItems ? changedPriceVariantIds : [], addItem, updateQuantity,
    removeItem, clear, openCart, closeCart, retryValidation,
  }), [state, ready, hasItems, validatedCart, validationStatus, validationError, unavailableVariantIds, changedPriceVariantIds, addItem, updateQuantity, removeItem, clear, openCart, closeCart, retryValidation]);

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

export function useCart() {
  const context = useContext(CartContext);
  if (!context) throw new Error("useCart must be used inside CartProvider");
  return context;
}
