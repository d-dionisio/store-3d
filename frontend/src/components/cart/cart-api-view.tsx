"use client";

import { Minus, Plus, ShoppingBag, Trash2 } from "lucide-react";
import Link from "next/link";
import { useCart } from "@/components/cart/cart-provider";
import { formatCurrency } from "@/lib/format";

export function CartView() {
  const cart = useCart();
  if (!cart.ready || (cart.validationStatus === "loading" && !cart.validatedCart)) return <div className="cart-skeleton" />;
  if (!cart.items.length) return <div className="empty-state cart-empty"><ShoppingBag /><h2>Seu carrinho está vazio.</h2><Link href="/produtos" className="button button-primary">Explorar produtos</Link></div>;
  if (cart.validationStatus === "error") return <div className="empty-state"><h2>Não foi possível validar o carrinho.</h2><p>{cart.validationError}</p><button type="button" className="button" onClick={cart.retryValidation}>Tentar novamente</button></div>;
  const lines = new Map(cart.validatedCart?.Items.map((line) => [line.VariantID, line]));
  return <div className="cart-layout"><div className="cart-list">{cart.items.map((item) => {
    const line = lines.get(item.productVariantId); const unavailable = cart.unavailableVariantIds.includes(item.productVariantId);
    return <article key={item.productVariantId} className="cart-item"><div className="cart-item-info"><span>{line?.SKU || "Indisponível"}</span><h2>{line?.ProductName || item.variantLabel || "Produto indisponível"}</h2><p>{line ? `${line.Color} · ${line.Material}` : "Remova este item para continuar."}</p><div className="quantity-control"><button disabled={unavailable || item.quantity <= 1} onClick={() => cart.updateQuantity(item.productVariantId, item.quantity - 1)} aria-label="Diminuir quantidade"><Minus size={15} /></button><span>{item.quantity}</span><button disabled={unavailable || Boolean(line?.AvailableQuantity && item.quantity >= line.AvailableQuantity)} onClick={() => cart.updateQuantity(item.productVariantId, item.quantity + 1)} aria-label="Aumentar quantidade"><Plus size={15} /></button></div></div><div className="cart-item-end"><strong>{line ? formatCurrency(line.LineTotalCents) : "—"}</strong><button onClick={() => cart.removeItem(item.productVariantId)}><Trash2 size={17} /> Remover</button></div></article>;
  })}</div><aside className="order-summary"><h2>Resumo validado</h2><div><span>Subtotal</span><strong>{cart.validatedCart ? formatCurrency(cart.validatedCart.SubtotalCents) : "—"}</strong></div><div><span>Frete</span><strong>Pendente de cotação</strong></div>{cart.unavailableVariantIds.length ? <p role="alert">Remova os itens indisponíveis para continuar.</p> : <Link href="/checkout" className="button button-primary">Continuar para checkout</Link>}<small>Preços e disponibilidade consultados no servidor.</small></aside></div>;
}
