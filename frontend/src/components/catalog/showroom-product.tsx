import { ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { formatCurrency } from "@/lib/format";
import { primaryProductImage } from "@/lib/showroom";
import type { Product } from "@/types/product";
import { CatalogImage } from "./catalog-image";

export function ShowroomProduct({ product, priority = false, variant = "portrait" }: { product: Product; priority?: boolean; variant?: "portrait" | "landscape" | "feature" }) {
  const image = primaryProductImage(product);
  return <article className={`showroom-product showroom-product-${variant}`}>
    <Link href={`/produtos/${product.slug}`} className="showroom-product-visual" aria-label={`Ver ${product.name}`}>
      <CatalogImage src={image?.url} alt={image?.altText || product.name} priority={priority} sizes={variant === "feature" ? "(max-width: 899px) 100vw, 68vw" : "(max-width: 899px) 88vw, 42vw"} />
      <span className="showroom-product-index" aria-hidden="true">Lumé object</span>
    </Link>
    <div className="showroom-product-copy">
      <div><p>{product.category.name}</p><h3>{product.name}</h3></div>
      <div className="showroom-product-action"><strong>A partir de {formatCurrency(product.minPriceCents)}</strong><Link href={`/produtos/${product.slug}`}>Ver produto <ArrowUpRight size={15} /></Link></div>
    </div>
    {variant === "feature" && <p className="showroom-product-description">{product.shortDescription || product.description}</p>}
  </article>;
}
