"use client";

import { Minus, Plus, ShoppingBag, Trash2 } from "lucide-react";
import Link from "next/link";
import { useCart } from "@/components/cart/cart-provider";
import { ProductImage } from "@/components/ui/product-image";
import { products } from "@/data/products";
import { formatCurrency } from "@/lib/format";

export function CartView() {
  const { items, ready, updateQuantity, removeItem } = useCart();
  if (!ready) return <div className="cart-skeleton" aria-label="Carregando carrinho" />;
  const detailed = items.flatMap((item) => { const product = products.find((entry) => entry.id === item.productId); return product ? [{ ...item, product }] : []; });
  if (!detailed.length) return <div className="empty-state cart-empty"><ShoppingBag /><h2>Seu carrinho está vazio.</h2><p>Encontre uma peça especial ou comece uma criação personalizada.</p><Link href="/produtos" className="button button-primary">Explorar produtos</Link></div>;
  const subtotal = detailed.reduce((sum, item) => sum + item.product.price * item.quantity, 0); const shipping = 24.9;
  return <div className="cart-layout"><div className="cart-list">{detailed.map(({ product, color, quantity }) => <article key={`${product.id}-${color}`} className="cart-item"><ProductImage product={product} /><div className="cart-item-info"><span>{product.category}</span><h2>{product.name}</h2><p>Cor: {color}</p><div className="quantity-control"><button onClick={() => updateQuantity(product.id, color, quantity - 1)}><Minus size={15} /></button><span>{quantity}</span><button onClick={() => updateQuantity(product.id, color, quantity + 1)}><Plus size={15} /></button></div></div><div className="cart-item-end"><strong>{formatCurrency(product.price * quantity)}</strong><button onClick={() => removeItem(product.id, color)}><Trash2 size={17} /> Remover</button></div></article>)}</div><aside className="order-summary"><h2>Resumo</h2><div><span>Subtotal</span><strong>{formatCurrency(subtotal)}</strong></div><div><span>Frete demonstrativo</span><strong>{formatCurrency(shipping)}</strong></div><div className="total"><span>Total ilustrativo</span><strong>{formatCurrency(subtotal + shipping)}</strong></div><Link href="/checkout" className="button button-primary">Ir para checkout protótipo</Link><small>Nenhuma cobrança será realizada.</small></aside></div>;
}

