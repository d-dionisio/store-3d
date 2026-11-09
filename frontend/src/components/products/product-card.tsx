"use client";

import { ArrowUpRight, Plus } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { useCart } from "@/components/cart/cart-provider";
import { ProductImage } from "@/components/ui/product-image";
import { formatCurrency } from "@/lib/format";
import type { Product } from "@/types/product";

export function ProductCard({ product }: { product: Product }) {
  const { addItem } = useCart();
  const [added, setAdded] = useState(false);
  const quickAdd = () => {
    const variant = product.variants[0];
    if (!variant) return;
    addItem({ productId: product.id, productSlug: product.slug, productVariantId: variant.id, quantity: 1 });
    setAdded(true); setTimeout(() => setAdded(false), 1400);
  };
  return (
    <article className="product-card">
      <Link href={`/produtos/${product.slug}`} className="product-media"><ProductImage product={product} /><span className="product-category">{product.category.name}</span><span className="view-product"><ArrowUpRight size={18} /></span></Link>
      <div className="product-info"><div><h3><Link href={`/produtos/${product.slug}`}>{product.name}</Link></h3><p>A partir de <strong>{formatCurrency(product.minPriceCents)}</strong></p></div><button disabled={!product.variants.length} onClick={quickAdd} aria-label={`Adicionar ${product.name} ao carrinho`} className={added ? "added" : ""}>{added ? "✓" : <Plus size={19} />}</button></div>
    </article>
  );
}

