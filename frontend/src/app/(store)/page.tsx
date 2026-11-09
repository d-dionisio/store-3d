import { ArrowRight, Box, Feather, Layers3, ScanLine, Sparkles } from "lucide-react";
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
    <section className="benefits"><div className="container benefits-grid"><div><Box /><span><strong>Produção sob demanda</strong>Menos estoque, mais intenção</span></div><div><Layers3 /><span><strong>Acabamento cuidadoso</strong>Camadas precisas e revisadas</span></div><div><Feather /><span><strong>Materiais selecionados</strong>Opções adequadas a cada uso</span></div></div></section>
    <section className="section container"><div className="section-heading"><div><span className="eyebrow dark">Curadoria Loja 3D</span><h2>Encontre sua próxima peça</h2></div><Link href="/produtos">Ver catálogo <ArrowRight size={17} /></Link></div>{categories.length ? <div className="category-grid">{categories.slice(0, 4).map((category, index) => <Link key={category.id} href={`/produtos?category=${category.slug}`} className={`category-card category-${index + 1}`}><span>0{index + 1}</span><div><h3>{category.name}</h3><p>{category.description}</p></div><ArrowRight /></Link>)}</div> : <div className="empty-state"><h2>Categorias em preparação.</h2></div>}</section>
    <section className="section container"><div className="section-heading"><div><span className="eyebrow dark">Seleção especial</span><h2>Peças em destaque</h2></div><Link href="/produtos">Descobrir todas <ArrowRight size={17} /></Link></div>{products.length ? <div className="product-grid home-products">{products.map((product) => <ProductCard key={product.id} product={product} />)}</div> : <div className="empty-state"><h2>Novidades em preparação.</h2><p>Em breve teremos novas peças em destaque.</p></div>}</section>
    <section className="custom-banner"><div className="container custom-grid"><div><span className="tech-label"><ScanLine size={16} /> Impressão personalizada</span><h2>Da sua ideia<br />para o mundo físico.</h2><p>Conte o que você imagina. Nossa futura experiência de orçamento será preparada para arquivos STL e 3MF, materiais e acabamentos.</p><Button href="/personalizados" variant="dark">Começar orçamento <ArrowRight size={17} /></Button></div><div className="tech-visual"><div className="wire-cube"><Sparkles /></div><span>Visualização 3D<br /><small>em uma próxima etapa</small></span></div></div></section>
    <section className="section about-preview container"><div><span className="eyebrow dark">Sobre a Loja 3D</span><h2>Objetos com propósito.<br />Tecnologia com sensibilidade.</h2></div><div><p>Acreditamos que fabricação digital também pode ser calorosa. Unimos desenho, prototipagem e produção cuidadosa para criar peças que fazem sentido no cotidiano.</p><Link href="/sobre">Conheça nosso processo <ArrowRight size={17} /></Link></div></section>
  </main>;
}

