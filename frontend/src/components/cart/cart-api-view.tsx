"use client";
import { Minus, Plus, ShoppingBag, Trash2 } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { useCart } from "@/components/cart/cart-provider";
import { ProductImage } from "@/components/ui/product-image";
import { formatCurrency } from "@/lib/format";
import { getProduct } from "@/services/catalog-api";
import type { Product } from "@/types/product";
export function CartView() {
  const { items, ready, updateQuantity, removeItem } = useCart(); const [products, setProducts] = useState<Product[]>([]); const [state, setState] = useState<"loading" | "idle" | "error">("loading");
  useEffect(() => { if (!ready || !items.length) return; const controller = new AbortController(); Promise.all([...new Set(items.map((item) => item.productSlug))].map((slug) => getProduct(slug, controller.signal))).then((data) => { setProducts(data); setState("idle"); }).catch(() => { if (!controller.signal.aborted) setState("error"); }); return () => controller.abort(); }, [items, ready]);
  const detailed = useMemo(() => items.flatMap((item) => { const product = products.find((entry) => entry.id === item.productId); const variant = product?.variants.find((entry) => entry.id === item.productVariantId); return product && variant ? [{ ...item, product, variant }] : []; }), [items, products]);
  if (!ready) return <div className="cart-skeleton" aria-label="Carregando preços atuais do carrinho" />;
  if (!items.length) return <div className="empty-state cart-empty"><ShoppingBag /><h2>Seu carrinho está vazio.</h2><p>Encontre uma peça especial ou comece uma criação personalizada.</p><Link href="/produtos" className="button button-primary">Explorar produtos</Link></div>;
  if (state === "loading") return <div className="cart-skeleton" aria-label="Carregando preços atuais do carrinho" />;
  if (state === "error") return <div className="empty-state"><h2>Não foi possível atualizar o carrinho.</h2><p>Verifique se a API está disponível e tente novamente.</p></div>;
  const subtotal = detailed.reduce((sum, item) => sum + item.variant.priceCents * item.quantity, 0);
  return <div className="cart-layout"><div className="cart-list">{detailed.map(({ product, variant, quantity }) => <article key={variant.id} className="cart-item"><ProductImage product={product} selectedImage={product.images.find((image) => image.variantId === variant.id)} /><div className="cart-item-info"><span>{product.category.name}</span><h2>{product.name}</h2><p>{variant.color.name} · {variant.material.name}</p><div className="quantity-control"><button onClick={() => updateQuantity(variant.id, quantity - 1)} aria-label="Diminuir"><Minus size={15} /></button><span>{quantity}</span><button onClick={() => updateQuantity(variant.id, quantity + 1)} aria-label="Aumentar"><Plus size={15} /></button></div></div><div className="cart-item-end"><strong>{formatCurrency(variant.priceCents * quantity)}</strong><button onClick={() => removeItem(variant.id)}><Trash2 size={17} /> Remover</button></div></article>)}</div><aside className="order-summary"><h2>Resumo</h2><div><span>Subtotal atualizado</span><strong>{formatCurrency(subtotal)}</strong></div><div><span>Frete</span><strong>Calculado depois</strong></div><div className="total"><span>Total de produtos</span><strong>{formatCurrency(subtotal)}</strong></div><Link href="/checkout" className="button button-primary">Ir para checkout protótipo</Link><small>Valores revalidados na API. Nenhuma cobrança será realizada.</small></aside></div>;
}
