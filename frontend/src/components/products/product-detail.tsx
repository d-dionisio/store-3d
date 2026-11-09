"use client";

import { Check, Minus, Plus, ShoppingBag } from "lucide-react";
import { useState } from "react";
import { useCart } from "@/components/cart/cart-provider";
import { ProductImage } from "@/components/ui/product-image";
import { formatCurrency } from "@/lib/format";
import type { Product } from "@/types/product";

export function ProductDetail({ product }: { product: Product }) {
  const [color, setColor] = useState(product.colors[0]);
  const [quantity, setQuantity] = useState(1);
  const [added, setAdded] = useState(false);
  const { addItem } = useCart();
  const add = () => { addItem({ productId: product.id, color, quantity }); setAdded(true); setTimeout(() => setAdded(false), 1800); };
  return <div className="product-detail">
    <div className="gallery"><ProductImage product={product} className="gallery-main" /><div className="gallery-thumbs"><ProductImage product={product} /><div className="material-closeup">Detalhe<br /><small>textura demonstrativa</small></div></div></div>
    <div className="product-summary"><span className="eyebrow dark">{product.category} · Produção sob demanda</span><h1>{product.name}</h1><p className="product-description">{product.description}</p><div className="detail-price">{formatCurrency(product.price)} <small>preço demonstrativo</small></div>
      <fieldset><legend>Cor — <strong>{color}</strong></legend><div className="color-options">{product.colors.map((item) => <button key={item} aria-label={`Selecionar cor ${item}`} aria-pressed={color === item} onClick={() => setColor(item)} className={`swatch swatch-${item.toLowerCase().replace("á", "a")} ${color === item ? "selected" : ""}`} />)}</div></fieldset>
      <div className="quantity-row"><span>Quantidade</span><div className="quantity-control"><button onClick={() => setQuantity(Math.max(1, quantity - 1))} aria-label="Diminuir quantidade"><Minus size={16} /></button><span>{quantity}</span><button onClick={() => setQuantity(quantity + 1)} aria-label="Aumentar quantidade"><Plus size={16} /></button></div></div>
      <button className={`add-to-cart ${added ? "success" : ""}`} onClick={add}>{added ? <><Check /> Adicionado ao carrinho</> : <><ShoppingBag /> Adicionar ao carrinho</>}</button>
      <dl className="spec-list"><div><dt>Dimensões</dt><dd>{product.dimensions}</dd></div><div><dt>Material</dt><dd>{product.material}</dd></div><div><dt>Produção</dt><dd>{product.leadTime}*</dd></div></dl><p className="fine-print">* Prazo ilustrativo. A confirmação dependerá da disponibilidade e configuração final em uma etapa futura.</p>
    </div>
  </div>;
}

