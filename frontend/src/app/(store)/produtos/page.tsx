import type { Metadata } from "next";
import { Catalog } from "@/components/products/catalog";

export const metadata: Metadata = { title: "Produtos", description: "Catálogo demonstrativo de objetos autorais impressos em 3D." };
export default function ProductsPage() { return <main className="page"><header className="page-hero container"><span className="eyebrow dark">Catálogo demonstrativo</span><h1>Objetos para viver<br /><em>com intenção.</em></h1><p>Explore formas autorais produzidas sob demanda. Preços e prazos são ilustrativos.</p></header><section className="container catalog-section"><Catalog /></section></main>; }

