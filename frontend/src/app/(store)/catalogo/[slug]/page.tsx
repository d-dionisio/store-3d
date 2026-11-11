import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { MagazineReader } from "@/components/catalog/magazine-reader";
import { ShareControls } from "@/components/catalog/share-controls";
import { getCategories } from "@/services/catalog-api";
import { getCollectionProducts } from "@/services/catalog-collections";

export const dynamic = "force-dynamic";
type Props = { params: Promise<{ slug: string }> };

async function collection(slug: string) {
  const categories = await getCategories();
  const category = categories.find((item) => item.slug === slug);
  if (!category) return null;
  return { category, products: await getCollectionProducts(category.slug) };
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const result = await collection(slug);
  if (!result) return { title: "Coleção não encontrada" };
  const image = result.products[0]?.images.find((item) => item.isPrimary) ?? result.products[0]?.images[0];
  return {
    title: `Catálogo ${result.category.name}`,
    description: result.category.description,
    openGraph: { title: `${result.category.name} — Catálogo Lumé Studio`, description: result.category.description, type: "website", images: image?.url ? [{ url: image.url, alt: image.altText || result.category.name }] : undefined },
  };
}

export default async function CollectionCatalogPage({ params }: Props) {
  const { slug } = await params;
  const result = await collection(slug);
  if (!result) notFound();
  return <main className="catalog-reader-page">
    <div className="container catalog-reader-toolbar"><div><span className="eyebrow">Edição digital</span><strong>{result.category.name}</strong></div><ShareControls title={result.category.name} /></div>
    {result.products.length === 0 ? <div className="container empty-state catalog-empty"><h1>Esta edição está em preparação.</h1><p>A coleção ainda não possui produtos disponíveis para publicação.</p></div> : <MagazineReader category={result.category} products={result.products} />}
  </main>;
}
