import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ProductDetail } from "@/components/products/product-detail";
import { ProductCard } from "@/components/products/product-card";
import { getProduct, products } from "@/data/products";

export function generateStaticParams() { return products.map(({ slug }) => ({ slug })); }
export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> { const product = getProduct((await params).slug); return product ? { title: product.name, description: product.description } : {}; }
export default async function ProductPage({ params }: { params: Promise<{ slug: string }> }) { const product = getProduct((await params).slug); if (!product) notFound(); const related = products.filter((item) => item.id !== product.id).slice(0, 3); return <main className="page"><section className="container detail-section"><ProductDetail product={product} /></section><section className="section container"><div className="section-heading"><div><span className="eyebrow dark">Continue explorando</span><h2>Peças relacionadas</h2></div></div><div className="product-grid related-grid">{related.map((item) => <ProductCard key={item.id} product={item} />)}</div></section></main>; }

