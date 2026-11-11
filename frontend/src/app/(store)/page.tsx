import { ArrowRight, Box, Feather, Layers3 } from "lucide-react";
import Link from "next/link";
import { Hero } from "@/components/home/hero";
import { ProductCard } from "@/components/products/product-card";
import { Button } from "@/components/ui/button";
import { getCategories, getProducts } from "@/services/catalog-api";
export const dynamic = "force-dynamic";

export default async function HomePage() {
  const [categories, productPage] = await Promise.all([getCategories(), getProducts({ limit: 20 })]);
  const products = productPage.data.filter((product) => product.isFeatured).slice(0, 4);
  return <main>
    <Hero />
    <section className="benefits"><div className="container benefits-grid"><div><Box /><span><strong>Produção sob demanda</strong>Feito especialmente para você</span></div><div><Layers3 /><span><strong>Acabamento cuidadoso</strong>Precisão em cada detalhe</span></div><div><Feather /><span><strong>Materiais selecionados</strong>Matéria, forma e intenção</span></div></div></section>
    <section id="colecoes" className="section container collections-section"><div className="section-heading"><div><span className="eyebrow dark">Coleções Lumé</span><h2>Design para viver<br /><em>com beleza.</em></h2></div><Link href="/produtos">Ver todas as peças <ArrowRight size={17} /></Link></div>{categories.length ? <div className="category-grid">{categories.slice(0, 4).map((category, index) => <Link key={category.id} href={`/produtos?category=${category.slug}`} className={`category-card category-${index + 1}`}><span>0{index + 1}</span><div><small>Coleção</small><h3>{category.name}</h3><p>{category.description}</p></div><ArrowRight /></Link>)}</div> : <div className="empty-state"><h2>Categorias em preparação.</h2></div>}</section>
    <section className="section container"><div className="section-heading"><div><span className="eyebrow dark">Seleção especial</span><h2>Peças em destaque</h2></div><Link href="/produtos">Descobrir todas <ArrowRight size={17} /></Link></div>{products.length ? <div className="product-grid home-products">{products.map((product) => <ProductCard key={product.id} product={product} />)}</div> : <div className="empty-state"><h2>Novidades em preparação.</h2><p>Em breve teremos novas peças em destaque.</p></div>}</section>
    <section className="custom-banner"><div className="container custom-grid"><div><span className="tech-label">Lumé Personal</span><h2>Sua ideia,<br /><em>uma peça única.</em></h2><p>Envie seu modelo e escolha material, cor e quantidade. Nossa equipe analisa cada solicitação e prepara uma proposta personalizada.</p><Button href="/personalizados" variant="dark">Solicitar orçamento <ArrowRight size={17} /></Button></div><div className="personal-object" aria-hidden="true"><span /></div></div></section>
    <section className="section about-preview container"><div><span className="eyebrow dark">Manifesto Lumé</span><h2>Tecnologia com alma.<br />Objetos com presença.</h2></div><div><p>Unimos desenho, fabricação digital e produção cuidadosa para criar peças contemporâneas que fazem sentido no cotidiano.</p><Link href="/sobre">Conheça nosso processo <ArrowRight size={17} /></Link></div></section>
  </main>;
}

