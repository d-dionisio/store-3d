import type { Metadata } from "next";
import { Catalog } from "@/components/products/catalog-api";
import { parseCatalogFilters } from "@/lib/catalog-filters";
import { getCategories, getProducts } from "@/services/catalog-api";
export const metadata: Metadata = { title: "Produtos", description: "Catálogo de objetos autorais impressos em 3D." };
type Params = Record<string, string | string[] | undefined>;
export default async function ProductsPage({ searchParams }: { searchParams: Promise<Params> }) { const filters = parseCatalogFilters(await searchParams); const [{ data, meta }, categories] = await Promise.all([getProducts(filters), getCategories()]); return <main className="page"><header className="page-hero container"><span className="eyebrow dark">Catálogo Loja 3D</span><h1>Objetos para viver<br /><em>com intenção.</em></h1><p>Explore formas autorais produzidas sob demanda, com preços e disponibilidade atualizados.</p></header><section className="container catalog-section"><Catalog products={data} categories={categories} meta={meta} /></section></main>; }
