import type { Metadata } from "next";
import { ArrowDown, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { CatalogImage } from "@/components/catalog/catalog-image";
import { ShowroomNav } from "@/components/catalog/showroom-nav";
import { ShowroomProduct } from "@/components/catalog/showroom-product";
import { ShowroomReveal } from "@/components/catalog/showroom-reveal";
import { catalogCover, uniqueCatalogCategories, uniqueCatalogProducts } from "@/lib/showroom";
import { getPublishedCatalogs } from "@/services/editorial-api";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Luxury Digital Showroom", description: "Coleções e objetos Lumé Studio em uma experiência editorial digital." };

export default async function CatalogIndexPage() {
  const catalogs = await getPublishedCatalogs();
  const products = uniqueCatalogProducts(catalogs);
  const featured = products.filter((product) => product.isFeatured);
  const selected = (featured.length ? featured : products).slice(0, 5);
  const categories = uniqueCatalogCategories(catalogs);
  const lead = catalogs[0];

  return <main className="showroom-page showroom-home">
    <ShowroomNav catalogs={catalogs} />
    <section className="showroom-hero">
      <div className="showroom-hero-media"><CatalogImage src={catalogCover(lead) || "/images/hero-studio.png"} alt={lead ? `Coleção ${lead.title}` : "Ambiente do Lumé Studio"} priority sizes="100vw" /><span /></div>
      <div className="container showroom-hero-copy">
        <p className="showroom-kicker">Lumé Studio · Digital showroom</p>
        <h1>Objetos para<br /><em>viver devagar.</em></h1>
        <div><p>Uma curadoria de formas, texturas e volumes impressos em 3D — apresentada como espaço, não como prateleira.</p>{lead && <Link href={`/catalogo/${lead.slug}`}>Entrar na coleção <ArrowUpRight size={17} /></Link>}</div>
      </div>
      <a href="#colecoes" className="showroom-scroll" aria-label="Ir para as coleções"><ArrowDown size={17} /> Explorar</a>
    </section>

    {catalogs.length === 0 ? <section className="container showroom-empty"><span>Em preparação</span><h2>O showroom está sendo composto.</h2><p>As coleções publicadas aparecerão aqui com seus produtos reais.</p><Link href="/produtos">Conhecer produtos</Link></section> : <>
      <section id="colecoes" className="container showroom-section showroom-collections">
        <ShowroomReveal className="showroom-section-heading"><p className="showroom-kicker">Coleções atuais</p><h2>Um olhar sobre<br />cada universo.</h2><span>{String(catalogs.length).padStart(2, "0")} edições publicadas</span></ShowroomReveal>
        <div className="showroom-collection-list">{catalogs.map((catalog, index) => <ShowroomReveal key={catalog.id} className={`showroom-collection-entry showroom-collection-entry-${index % 3}`} delay={(index % 3) * 90}>
          <Link href={`/catalogo/${catalog.slug}`} className="showroom-collection-image"><CatalogImage src={catalogCover(catalog)} alt={`Capa da coleção ${catalog.title}`} priority={index === 0} sizes="(max-width: 899px) 92vw, 64vw" /><span>{catalog.editionLabel || `Edição ${String(index + 1).padStart(2, "0")}`}</span></Link>
          <div><p>{catalog.eyebrow || "Coleção editorial"}</p><h3><Link href={`/catalogo/${catalog.slug}`}>{catalog.title}</Link></h3><p>{catalog.description}</p><Link href={`/catalogo/${catalog.slug}`}>Descobrir coleção <ArrowUpRight size={16} /></Link></div>
        </ShowroomReveal>)}</div>
      </section>

      {products.length > 0 && <section className="showroom-latest">
        <div className="container showroom-latest-header"><div><p className="showroom-kicker">Última edição</p><h2>Novos no olhar.</h2></div><p>Uma seleção da edição publicada mais recente, com disponibilidade e valores consultados diretamente no catálogo.</p></div>
        <div className="showroom-product-rail" role="region" aria-label="Produtos da última edição">{(lead?.products ?? products).slice(0, 8).map((product, index) => <ShowroomProduct key={product.id} product={product} priority={index < 2} />)}</div>
      </section>}

      {selected.length > 0 && <section className="container showroom-section showroom-featured">
        <ShowroomReveal className="showroom-featured-intro"><p className="showroom-kicker">Peças em destaque</p><h2>Forma, matéria<br />e presença.</h2><p>Objetos escolhidos para ocupar o espaço com personalidade, sem excessos.</p></ShowroomReveal>
        <div className="showroom-featured-grid">{selected.map((product, index) => <ShowroomReveal key={product.id} className={`showroom-featured-item item-${index}`}><ShowroomProduct product={product} variant={index === 0 || index === 3 ? "feature" : index === 2 ? "landscape" : "portrait"} /></ShowroomReveal>)}</div>
      </section>}

      {categories.length > 0 && <section className="showroom-categories"><div className="container"><p className="showroom-kicker">Navegue por afinidade</p>{categories.map((category, index) => <Link key={category.id} href={`/produtos?category=${encodeURIComponent(category.slug)}`}><span>{String(index + 1).padStart(2, "0")}</span><strong>{category.name}</strong><ArrowUpRight /></Link>)}</div></section>}
    </>}

    <section className="showroom-closing"><div className="container"><p className="showroom-kicker">Lumé Studio</p><h2>Design que transforma<br /><em>o cotidiano.</em></h2><div><p>Explore cada objeto em detalhes ou compartilhe uma coleção com quem aprecia novas formas de habitar.</p><Link href="/produtos">Ver toda a loja <ArrowUpRight size={16} /></Link></div></div></section>
  </main>;
}
