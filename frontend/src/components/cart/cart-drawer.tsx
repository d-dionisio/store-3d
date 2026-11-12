"use client";

import Image from "next/image";
import Link from "next/link";
import { AlertTriangle, ArrowRight, Minus, Plus, ShoppingBag, X } from "lucide-react";
import { useEffect, useRef } from "react";
import { formatCurrency } from "@/lib/format";
import { useCart } from "./cart-provider";
import { MAX_CART_QUANTITY } from "./cart-state";
import styles from "./cart-drawer.module.css";

export function CartDrawer() {
  const cart = useCart();
  const { isOpen, closeCart } = cart;
  const dialog = useRef<HTMLElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);
  const restoreFocus = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    restoreFocus.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const frame = requestAnimationFrame(() => closeButton.current?.focus());
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") { event.preventDefault(); closeCart(); return; }
      if (event.key !== "Tab") return;
      const focusable = dialog.current?.querySelectorAll<HTMLElement>('a[href],button:not([disabled]),[tabindex]:not([tabindex="-1"])');
      if (!focusable?.length) return;
      const first = focusable[0], last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", keyboard);
    return () => {
      cancelAnimationFrame(frame); document.removeEventListener("keydown", keyboard);
      document.body.style.overflow = previousOverflow; restoreFocus.current?.focus();
    };
  }, [isOpen, closeCart]);

  if (!cart.isOpen) return <span className={styles.srOnly} aria-live="polite" aria-atomic="true">{cart.announcement}</span>;
  const lines = new Map(cart.validatedCart?.Items.map((line) => [line.VariantID, line]));
  const hasUnavailable = cart.unavailableVariantIds.length > 0;
  const checkoutDisabled = cart.validationStatus !== "ready" || hasUnavailable || !cart.validatedCart?.Items.length;

  return <div className={styles.root}>
    <button className={styles.overlay} type="button" aria-label="Fechar carrinho" onClick={cart.closeCart} />
    <aside ref={dialog} className={styles.dialog} role="dialog" aria-modal="true" aria-labelledby="smart-cart-title">
      <header className={styles.header}>
        <div><span className={styles.eyebrow}>Sua curadoria</span><h2 id="smart-cart-title">Carrinho</h2><p>{cart.count} {cart.count === 1 ? "peça selecionada" : "peças selecionadas"}</p></div>
        <button ref={closeButton} type="button" className={styles.close} onClick={cart.closeCart} aria-label="Fechar carrinho"><X size={19} /></button>
      </header>
      <div className={styles.body}>
        {!cart.ready || (cart.validationStatus === "loading" && !cart.validatedCart) ? <><div className={styles.skeleton} /><div className={styles.skeleton} /></> : null}
        {cart.ready && !cart.items.length ? <div className={styles.empty}><span className={styles.emptyIcon}><ShoppingBag /></span><h3>Sua seleção está vazia</h3><p>Descubra objetos autorais impressos em 3D para transformar seus espaços.</p><button type="button" onClick={cart.closeCart}>Explorar coleção</button></div> : null}
        {cart.validationStatus === "error" ? <div className={styles.error} role="alert"><strong>Não foi possível atualizar preços e disponibilidade.</strong><br />{cart.validationError}<br /><button type="button" onClick={cart.retryValidation}>Tentar novamente</button></div> : null}
        {cart.items.map((item) => {
          const line = lines.get(item.productVariantId);
          const unavailable = cart.unavailableVariantIds.includes(item.productVariantId);
          const changed = cart.changedPriceVariantIds.includes(item.productVariantId);
          const maxReached = item.quantity >= MAX_CART_QUANTITY || (line ? line.AvailableQuantity > 0 && line.Quantity >= line.AvailableQuantity : false);
          return <article className={styles.item} key={item.productVariantId}>
            <div className={styles.image}>{item.imageUrl?.startsWith("/") ? <Image src={item.imageUrl} alt={item.imageAlt || line?.ProductName || "Produto selecionado"} fill sizes="104px" /> : <div className={styles.placeholder}>Lumé</div>}</div>
            <div>
              <div className={styles.itemTop}><div><h3>{line?.ProductName || item.variantLabel || "Produto"}</h3><p className={styles.itemMeta}>{line ? `${line.Color} · ${line.Material}` : "Aguardando validação"}<br />{item.variantLabel && item.variantLabel !== line?.ProductName ? item.variantLabel : line?.SKU}</p></div>{line ? <span className={styles.unit}>{formatCurrency(line.UnitPriceCents)}</span> : null}</div>
              {line ? <div className={styles.lineTotal}>{formatCurrency(line.LineTotalCents)}</div> : null}
              <div className={styles.quantityRow}><div className={styles.quantity}><button type="button" disabled={item.quantity <= 1 || unavailable} onClick={() => cart.updateQuantity(item.productVariantId, item.quantity - 1)} aria-label={`Diminuir quantidade de ${line?.ProductName || "produto"}`}><Minus size={13} /></button><span aria-label={`Quantidade ${item.quantity}`}>{item.quantity}</span><button type="button" disabled={maxReached || unavailable} onClick={() => cart.updateQuantity(item.productVariantId, item.quantity + 1)} aria-label={`Aumentar quantidade de ${line?.ProductName || "produto"}`}><Plus size={13} /></button></div><button type="button" className={styles.remove} onClick={() => cart.removeItem(item.productVariantId)}>Remover</button></div>
            </div>
            {changed ? <p className={`${styles.notice} ${styles.changed}`} role="status"><AlertTriangle size={14} /> O preço foi atualizado para o valor atual da loja.</p> : null}
            {unavailable ? <p className={`${styles.notice} ${styles.unavailable}`} role="alert"><AlertTriangle size={14} /> Variante indisponível ou quantidade acima do estoque atual.</p> : null}
          </article>;
        })}
      </div>
      {cart.items.length ? <footer className={styles.footer}>
        <div className={styles.summary}><span>Subtotal validado</span><strong>{cart.validatedCart ? formatCurrency(cart.validatedCart.SubtotalCents) : "—"}</strong></div>
        <p className={styles.shipping}>Frete calculado no checkout, conforme endereço e modalidade disponível.</p>
        <Link href="/checkout" onClick={cart.closeCart} className={styles.checkout} aria-disabled={checkoutDisabled} tabIndex={checkoutDisabled ? -1 : undefined}>Finalizar compra <ArrowRight size={15} /></Link>
        <button type="button" className={styles.continue} onClick={cart.closeCart}>Continuar comprando</button>
      </footer> : null}
      <span className={styles.srOnly} aria-live="polite" aria-atomic="true">{cart.announcement}</span>
    </aside>
  </div>;
}
