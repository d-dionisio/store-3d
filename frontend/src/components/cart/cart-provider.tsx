"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { CartItem } from "@/types/product";

type CartContextValue = {
  items: CartItem[];
  count: number;
  ready: boolean;
  addItem: (item: CartItem) => void;
  updateQuantity: (variantId: string, quantity: number) => void;
  removeItem: (variantId: string) => void;
  clear: () => void;
};

const CartContext = createContext<CartContextValue | null>(null);
const storageKey = "loja-3d-demo-cart";

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<CartItem[]>([]);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let storedItems: CartItem[] = [];
    try {
      const stored = window.localStorage.getItem(storageKey);
      if (stored) { const parsed: unknown = JSON.parse(stored); if (Array.isArray(parsed)) storedItems = parsed.filter((item): item is CartItem => typeof item === "object" && item !== null && "productVariantId" in item && "quantity" in item); }
    } catch { /* Invalid demo storage is ignored. */ }
    const timer = window.setTimeout(() => {
      setItems(storedItems);
      setReady(true);
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (ready) window.localStorage.setItem(storageKey, JSON.stringify(items.map(({productVariantId,quantity})=>({productVariantId,quantity}))));
  }, [items, ready]);

  const addItem = useCallback((item: CartItem) => {
    setItems((current) => {
      const match = current.find((entry) => entry.productVariantId === item.productVariantId);
      return match
        ? current.map((entry) => entry === match ? { ...entry, quantity: entry.quantity + item.quantity } : entry)
        : [...current, item];
    });
  }, []);

  const updateQuantity = useCallback((variantId: string, quantity: number) => {
    setItems((current) => current.map((item) => item.productVariantId === variantId ? { ...item, quantity: Math.max(1, quantity) } : item));
  }, []);

  const removeItem = useCallback((variantId: string) => {
    setItems((current) => current.filter((item) => item.productVariantId !== variantId));
  }, []);

  const clear = useCallback(()=>setItems([]),[]);

  const value = useMemo(() => ({ items, ready, addItem, updateQuantity, removeItem, clear, count: items.reduce((sum, item) => sum + item.quantity, 0) }), [items, ready, addItem, updateQuantity, removeItem, clear]);
  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

export function useCart() {
  const context = useContext(CartContext);
  if (!context) throw new Error("useCart must be used inside CartProvider");
  return context;
}

