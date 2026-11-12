import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ProductDetail } from "@/components/products/product-detail-view";
import { ProductCard } from "@/components/products/product-card";
import { ApiError, getProduct, getProducts } from "@/services/catalog-api";
import "./product-detail.css";
export const dynamic = "force-dynamic";

async function find(slug: string) { try { return await getProduct(slug); } catch (error) { if (error instanceof ApiError && error.status === 404) notFound(); throw error; } }
export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> { const product = await find((await params).slug); return { title: product.name, description: product.shortDescription || product.description }; }
export default async function ProductPage({ params }: { params: Promise<{ slug: string }> }) { const product = await find((await params).slug); const relatedPage = await getProducts({ category: product.category.slug, limit: 4 }); const related = relatedPage.data.filter((item) => item.id !== product.id).slice(0, 3); return <main className="page product-page"><section className="container detail-section"><ProductDetail product={product} /></section>{related.length > 0 && <section className="section container product-related"><div className="section-heading"><div><span className="eyebrow dark">Continue explorando</span><h2>Peças relacionadas</h2></div></div><div className="product-grid related-grid">{related.map((item) => <ProductCard key={item.id} product={item} />)}</div></section>}</main>; }

