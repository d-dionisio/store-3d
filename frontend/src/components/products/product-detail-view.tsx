"use client";
import { Check, Minus, Plus, ShoppingBag } from "lucide-react";
import { useMemo, useState } from "react";
import { useCart } from "@/components/cart/cart-provider";
import { ProductImage } from "@/components/ui/product-image";
import { formatCurrency, formatDimensions, formatLeadTime } from "@/lib/format";
import type { Product } from "@/types/product";

export function ProductDetail({ product }: { product: Product }) {
  const [variantId, setVariantId] = useState(product.variants[0]?.id ?? ""); const [quantity, setQuantity] = useState(1); const [added, setAdded] = useState(false); const { addItem } = useCart();
  const selected = useMemo(() => product.variants.find((item) => item.id === variantId) ?? product.variants[0], [product.variants, variantId]);
  if (!selected) return <div className="empty-state"><h2>Produto indisponível.</h2><p>Nenhuma variante comercializável foi encontrada.</p></div>;
  const selectedImage = product.images.find((item) => item.variantId === selected.id) ?? product.images.find((item) => item.isPrimary);
  const add = () => { addItem({ productId: product.id, productSlug: product.slug, productVariantId: selected.id, quantity }); setAdded(true); setTimeout(() => setAdded(false), 1800); };
  return <div className="product-detail"><div className="gallery"><ProductImage product={product} selectedImage={selectedImage} className="gallery-main" /><div className="gallery-thumbs"><ProductImage product={product} /><div className="material-closeup">Detalhe<br /><small>{selected.material.name}</small></div></div></div>
    <div className="product-summary"><span className="eyebrow dark">{product.category.name} · Produção sob demanda</span><h1>{product.name}</h1><p className="product-description">{product.description}</p><div className="detail-price">{formatCurrency(selected.priceCents)} <small>preço da configuração selecionada</small></div>
      <fieldset><legend>Configuração — <strong>{selected.name ?? selected.sku}</strong></legend><div className="variant-options">{product.variants.map((item) => <button key={item.id} aria-pressed={item.id === selected.id} onClick={() => setVariantId(item.id)} className={item.id === selected.id ? "selected" : ""}><span className="variant-color" style={{ backgroundColor: item.color.hexCode }} />{item.color.name} · {item.material.name}{item.size ? ` · ${item.size.name}` : ""}{item.finish ? ` · ${item.finish.name}` : ""}</button>)}</div></fieldset>
      <div className="quantity-row"><span>Quantidade</span><div className="quantity-control"><button onClick={() => setQuantity(Math.max(1, quantity - 1))} aria-label="Diminuir quantidade"><Minus size={16} /></button><span>{quantity}</span><button onClick={() => setQuantity(quantity + 1)} aria-label="Aumentar quantidade"><Plus size={16} /></button></div></div>
      <button className={`add-to-cart ${added ? "success" : ""}`} onClick={add}>{added ? <><Check /> Adicionado ao carrinho</> : <><ShoppingBag /> Adicionar ao carrinho</>}</button>
      <dl className="spec-list"><div><dt>Dimensões</dt><dd>{formatDimensions(selected.dimensions)}</dd></div><div><dt>Material</dt><dd>{selected.material.name}</dd></div><div><dt>Cor</dt><dd>{selected.color.name}</dd></div><div><dt>Produção</dt><dd>{formatLeadTime(selected.productionLeadTime)}</dd></div></dl>
    </div></div>;
}
