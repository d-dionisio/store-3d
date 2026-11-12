import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { MagazineReader } from "@/components/catalog/magazine-reader";
import { ShareControls } from "@/components/catalog/share-controls";
import { getPublishedCatalog } from "@/services/editorial-api";

export const dynamic = "force-dynamic";
type Props = { params: Promise<{ slug: string }> };

async function collection(slug: string) {
  return getPublishedCatalog(slug);
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const result = await collection(slug);
  if (!result) return { title: "Coleção não encontrada" };
  const image = result.products[0]?.images.find((item) => item.isPrimary) ?? result.products[0]?.images[0];
  return {
    title: `Catálogo ${result.title}`,
    description: result.description,
    openGraph: { title: `${result.title} — Catálogo Lumé Studio`, description: result.description, type: "website", images: (result.coverImageUrl || image?.url) ? [{ url: result.coverImageUrl || image!.url, alt: image?.altText || result.title }] : undefined },
  };
}

export default async function CollectionCatalogPage({ params }: Props) {
  const { slug } = await params;
  const result = await collection(slug);
  if (!result) notFound();
  return <main className="catalog-reader-page">
    <div className="container catalog-reader-toolbar"><div><span className="eyebrow">{result.eyebrow}</span><strong>{result.title}</strong></div><ShareControls title={result.title} /></div>
    {result.products.length === 0 ? <div className="container empty-state catalog-empty"><h1>Esta edição está em preparação.</h1><p>A coleção ainda não possui produtos disponíveis para publicação.</p></div> : <MagazineReader category={{slug:result.slug,name:result.title,description:result.description,eyebrow:result.eyebrow,editionLabel:result.editionLabel,coverImageUrl:result.coverImageUrl}} products={result.products} />}
  </main>;
}
