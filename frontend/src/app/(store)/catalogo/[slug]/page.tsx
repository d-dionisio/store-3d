import type { Metadata } from "next";
import { ArrowDown, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { CatalogImage } from "@/components/catalog/catalog-image";
import { ShareControls } from "@/components/catalog/share-controls";
import { ShowroomNav } from "@/components/catalog/showroom-nav";
import { ShowroomProduct } from "@/components/catalog/showroom-product";
import { ShowroomReveal } from "@/components/catalog/showroom-reveal";
import { catalogCover } from "@/lib/showroom";
import { getPublishedCatalog, getPublishedCatalogs } from "@/services/editorial-api";

export const dynamic = "force-dynamic";
type Props = { params: Promise<{ slug: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const catalog = await getPublishedCatalog(slug);
  if (!catalog) return { title: "Coleção não encontrada" };
  const cover = catalogCover(catalog);
  return { title: `${catalog.title} — Digital Showroom`, description: catalog.description, openGraph: { title: `${catalog.title} — Lumé Studio`, description: catalog.description, type: "website", images: cover ? [{ url: cover, alt: `Coleção ${catalog.title}` }] : undefined } };
}

export default async function CollectionCatalogPage({ params }: Props) {
  const { slug } = await params;
  const [catalog, catalogs] = await Promise.all([getPublishedCatalog(slug), getPublishedCatalogs()]);
  if (!catalog) notFound();
  const others = catalogs.filter((item) => item.id !== catalog.id).slice(0, 3);

  return <main className="showroom-page showroom-collection-page">
    <ShowroomNav catalogs={catalogs} currentSlug={catalog.slug} />
    <section className="showroom-collection-hero">
      <div className="showroom-collection-hero-media"><CatalogImage src={catalogCover(catalog)} alt={`Coleção ${catalog.title}`} priority sizes="100vw" /><span /></div>
      <div className="container showroom-collection-hero-copy"><p className="showroom-kicker">{catalog.eyebrow || "Coleção Lumé"}</p><h1>{catalog.title}</h1><div><p>{catalog.description}</p><ShareControls title={catalog.title} /></div></div>
      <a href="#historia" className="showroom-scroll"><ArrowDown size={17} /> Conhecer</a>
    </section>

    {catalog.products.length === 0 ? <section className="container showroom-empty"><span>Em preparação</span><h2>Esta coleção ainda está sendo composta.</h2><p>Os produtos surgirão aqui assim que estiverem disponíveis para publicação.</p></section> : <>
      <section id="historia" className="container showroom-collection-intro"><ShowroomReveal><p className="showroom-kicker">{catalog.editionLabel || "Edição digital"}</p><h2>Uma coleção para ser<br /><em>vista de perto.</em></h2></ShowroomReveal><ShowroomReveal><p>{catalog.description}</p><span>{catalog.products.length} {catalog.products.length === 1 ? "objeto selecionado" : "objetos selecionados"}</span></ShowroomReveal></section>

      <section className="container showroom-story" aria-label={`Produtos da coleção ${catalog.title}`}>
        {catalog.products.map((product, index) => {
          const mode = index % 5;
          if (mode === 0) return <ShowroomReveal key={product.id} className="showroom-story-feature"><ShowroomProduct product={product} variant="feature" priority={index === 0} /></ShowroomReveal>;
          return <ShowroomReveal key={product.id} className={`showroom-story-product story-${mode}`} delay={(mode % 2) * 80}><span className="showroom-story-number">{String(index + 1).padStart(2, "0")}</span><ShowroomProduct product={product} variant={mode === 2 ? "landscape" : "portrait"} /></ShowroomReveal>;
        })}
      </section>

      <section className="showroom-collection-share"><div className="container"><p className="showroom-kicker">Compartilhe a curadoria</p><h2>Leve esta coleção<br />para outra conversa.</h2><ShareControls title={catalog.title} /></div></section>
    </>}

    <section className="container showroom-next"><div><p className="showroom-kicker">Continue explorando</p><h2>Outras coleções</h2></div>{others.length ? <div className="showroom-next-grid">{others.map((item) => <Link key={item.id} href={`/catalogo/${item.slug}`}><span className="showroom-next-image"><CatalogImage src={catalogCover(item)} alt={item.title} sizes="(max-width: 899px) 88vw, 30vw" /></span><strong>{item.title}</strong><ArrowUpRight /></Link>)}</div> : <Link className="showroom-back-link" href="/catalogo">Voltar ao showroom <ArrowUpRight size={16} /></Link>}</section>
  </main>;
}
