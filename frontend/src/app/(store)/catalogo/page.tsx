import type { Metadata } from "next";
import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { CatalogImage } from "@/components/catalog/catalog-image";
import { getPublishedCatalogs } from "@/services/editorial-api";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Catálogos", description: "Explore as coleções Lumé Studio em edições digitais interativas." };

export default async function CatalogIndexPage() {
  const collections = await getPublishedCatalogs();

  return <main className="page catalog-index-page">
    <header className="catalog-index-hero container">
      <p className="eyebrow">Biblioteca de coleções</p>
      <h1>Catálogos para<br /><em>folhear devagar.</em></h1>
      <p>Uma seleção editorial dos objetos Lumé Studio, organizada a partir das coleções disponíveis na loja.</p>
    </header>
    <section className="container catalog-editions" aria-label="Edições disponíveis">
      {collections.length === 0 ? <div className="empty-state"><h2>Nenhuma edição disponível</h2><p>As próximas coleções aparecerão aqui assim que estiverem publicadas.</p></div> : collections.map((catalog, index) => {
        const cover = catalog.products[0]?.images.find((image) => image.isPrimary) ?? catalog.products[0]?.images[0];
        return <article className="catalog-edition" key={catalog.id}>
          <Link href={`/catalogo/${catalog.slug}`} className="catalog-edition-cover" aria-label={`Abrir catálogo ${catalog.title}`}>
            <CatalogImage src={catalog.coverImageUrl || cover?.url} alt={`Capa da coleção ${catalog.title}`} priority={index === 0} sizes="(max-width: 699px) 100vw, 42vw" />
            <span className="catalog-edition-overlay" />
            <span className="catalog-edition-brand">LUMÉ <small>{catalog.editionLabel || `EDIÇÃO ${String(index + 1).padStart(2, "0")}`}</small></span>
            <strong>{catalog.title}</strong>
          </Link>
          <div className="catalog-edition-meta"><div><span>{catalog.products.length} {catalog.products.length === 1 ? "peça" : "peças"}</span><p>{catalog.description}</p></div><Link href={`/catalogo/${catalog.slug}`} aria-label={`Ler ${catalog.title}`}><ArrowUpRight /></Link></div>
        </article>;
      })}
    </section>
  </main>;
}
