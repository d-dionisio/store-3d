"use client";

import { ArrowLeft, ArrowRight, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { CatalogImage } from "@/components/catalog/catalog-image";
import { formatCurrency } from "@/lib/format";
import type { Category, Product } from "@/types/product";

type MagazinePage = { kind: "cover" } | { kind: "product"; product: Product; number: number } | { kind: "closing" };

function ProductSpread({ product, number }: { product: Product; number: number }) {
  const image = product.images.find((item) => item.isPrimary) ?? product.images[0];
  return <div className="catalog-product-spread">
    <div className="catalog-spread-image"><CatalogImage src={image?.url} alt={image?.altText || product.name} sizes="(max-width: 899px) 100vw, 56vw" /></div>
    <div className="catalog-spread-copy">
      <span className="catalog-folio">{String(number).padStart(2, "0")} / Edição</span>
      <p className="eyebrow">{product.category.name}</p>
      <h2>{product.name}</h2>
      <p>{product.shortDescription || product.description}</p>
      <strong>A partir de {formatCurrency(product.minPriceCents)}</strong>
      <Link href={`/produtos/${product.slug}`}>Ver produto <ArrowUpRight size={16} /></Link>
    </div>
  </div>;
}

function PageContent({ page, category, products }: { page: MagazinePage; category: Category; products: Product[] }) {
  if (page.kind === "product") return <ProductSpread product={page.product} number={page.number} />;
  if (page.kind === "closing") return <div className="catalog-closing">
    <span className="eyebrow">Fim da edição</span><h2>Feito para permanecer.</h2>
    <p>Continue explorando objetos autorais produzidos em pequenas séries.</p>
    <Link className="button button-primary" href={`/produtos?category=${category.slug}`}>Ver coleção completa</Link>
  </div>;
  const coverImage = products[0]?.images.find((item) => item.isPrimary) ?? products[0]?.images[0];
  return <div className="catalog-cover-page">
    <CatalogImage src={coverImage?.url} alt={`Capa da coleção ${category.name}`} priority sizes="(max-width: 899px) 100vw, 80vw" />
    <div className="catalog-cover-shade" />
    <div className="catalog-cover-wordmark">LUMÉ <small>CATÁLOGO / 2026</small></div>
    <div className="catalog-cover-title"><span>Collection no. {String(products.length).padStart(2, "0")}</span><h1>{category.name}</h1><p>{category.description}</p></div>
  </div>;
}

export function MagazineReader({ category, products }: { category: Category; products: Product[] }) {
  const pages: MagazinePage[] = [{ kind: "cover" }, ...products.map((product, index) => ({ kind: "product" as const, product, number: index + 1 })), { kind: "closing" }];
  const [current, setCurrent] = useState(0);
  const [direction, setDirection] = useState<"next" | "previous">("next");
  const move = useCallback((next: number) => {
    const target = Math.max(0, Math.min(pages.length - 1, next));
    setDirection(target >= current ? "next" : "previous");
    setCurrent(target);
  }, [current, pages.length]);

  useEffect(() => {
    const keyboard = (event: KeyboardEvent) => {
      if (["ArrowRight", "PageDown"].includes(event.key)) { event.preventDefault(); move(current + 1); }
      if (["ArrowLeft", "PageUp"].includes(event.key)) { event.preventDefault(); move(current - 1); }
      if (event.key === "Home") move(0);
      if (event.key === "End") move(pages.length - 1);
    };
    window.addEventListener("keydown", keyboard);
    return () => window.removeEventListener("keydown", keyboard);
  }, [current, move, pages.length]);

  return <>
    <section className="magazine-desktop" aria-label={`Revista digital da coleção ${category.name}`} tabIndex={0}>
      <div className="magazine-stage">
        <article key={current} className={`magazine-page turn-${direction}`} aria-live="polite"><PageContent page={pages[current]} category={category} products={products} /></article>
      </div>
      <div className="magazine-navigation">
        <button type="button" onClick={() => move(current - 1)} disabled={current === 0} aria-label="Página anterior"><ArrowLeft size={18} /></button>
        <div><span>{String(current + 1).padStart(2, "0")}</span><i /><span>{String(pages.length).padStart(2, "0")}</span></div>
        <button type="button" onClick={() => move(current + 1)} disabled={current === pages.length - 1} aria-label="Próxima página"><ArrowRight size={18} /></button>
      </div>
      <p className="magazine-hint">Use as setas do teclado para folhear</p>
    </section>
    <section className="magazine-mobile" aria-label={`Catálogo vertical da coleção ${category.name}`}>
      {pages.map((page, index) => <article key={page.kind === "product" ? page.product.id : page.kind} className="magazine-mobile-page"><PageContent page={page} category={category} products={products} />{index < pages.length - 1 && <span className="mobile-page-number">{String(index + 1).padStart(2, "0")}</span>}</article>)}
    </section>
  </>;
}
