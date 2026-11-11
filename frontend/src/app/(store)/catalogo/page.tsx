import type { Metadata } from "next";
import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { CatalogImage } from "@/components/catalog/catalog-image";
import { getCategories } from "@/services/catalog-api";
import { getCollectionProducts } from "@/services/catalog-collections";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Catálogos", description: "Explore as coleções Lumé Studio em edições digitais interativas." };

export default async function CatalogIndexPage() {
  const categories = await getCategories();
  const collections = await Promise.all(categories.map(async (category) => ({ category, products: await getCollectionProducts(category.slug) })));

  return <main className="page catalog-index-page">
    <header className="catalog-index-hero container">
      <p className="eyebrow">Biblioteca de coleções</p>
      <h1>Catálogos para<br /><em>folhear devagar.</em></h1>
      <p>Uma seleção editorial dos objetos Lumé Studio, organizada a partir das coleções disponíveis na loja.</p>
    </header>
    <section className="container catalog-editions" aria-label="Edições disponíveis">
      {collections.length === 0 ? <div className="empty-state"><h2>Nenhuma edição disponível</h2><p>As próximas coleções aparecerão aqui assim que estiverem publicadas.</p></div> : collections.map(({ category, products }, index) => {
        const cover = products[0]?.images.find((image) => image.isPrimary) ?? products[0]?.images[0];
        return <article className="catalog-edition" key={category.id}>
          <Link href={`/catalogo/${category.slug}`} className="catalog-edition-cover" aria-label={`Abrir catálogo ${category.name}`}>
            <CatalogImage src={cover?.url} alt={`Capa da coleção ${category.name}`} priority={index === 0} sizes="(max-width: 699px) 100vw, 42vw" />
            <span className="catalog-edition-overlay" />
            <span className="catalog-edition-brand">LUMÉ <small>EDIÇÃO {String(index + 1).padStart(2, "0")}</small></span>
            <strong>{category.name}</strong>
          </Link>
          <div className="catalog-edition-meta"><div><span>{products.length} {products.length === 1 ? "peça" : "peças"}</span><p>{category.description}</p></div><Link href={`/catalogo/${category.slug}`} aria-label={`Ler ${category.name}`}><ArrowUpRight /></Link></div>
        </article>;
      })}
    </section>
  </main>;
}
