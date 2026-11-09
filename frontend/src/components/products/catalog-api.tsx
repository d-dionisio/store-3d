"use client";
import { Search } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { ProductCard } from "@/components/products/product-card";
import type { Category, PaginationMeta, Product } from "@/types/product";
export function Catalog({ products, categories, meta }: { products: Product[]; categories: Category[]; meta: PaginationMeta }) {
  const router = useRouter(); const pathname = usePathname(); const current = useSearchParams(); const [search, setSearch] = useState(current.get("search") ?? "");
  const update = (name: string, value: string) => { const params = new URLSearchParams(current); if (value) params.set(name, value); else params.delete(name); if (name !== "page") params.delete("page"); router.push(`${pathname}?${params.toString()}`); };
  const price = current.get("min_price") === "10000" ? "100to180" : current.get("min_price") === "18001" ? "over180" : current.get("max_price") === "10000" ? "under100" : "all";
  const setPrice = (value: string) => { const params = new URLSearchParams(current); params.delete("min_price"); params.delete("max_price"); params.delete("page"); if (value === "under100") params.set("max_price", "10000"); if (value === "100to180") { params.set("min_price", "10000"); params.set("max_price", "18000"); } if (value === "over180") params.set("min_price", "18001"); router.push(`${pathname}?${params.toString()}`); };
  const colors = [...new Map(products.flatMap((p) => p.colors).map((c) => [c.slug, c])).values()];
  return <><div className="catalog-toolbar"><form className="search-field" onSubmit={(event) => { event.preventDefault(); update("search", search.trim()); }}><Search size={18} /><span className="sr-only">Buscar produtos</span><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Buscar por nome ou estilo" /></form>
    <label><span>Categoria</span><select value={current.get("category") ?? ""} onChange={(e) => update("category", e.target.value)}><option value="">Todas</option>{categories.map((item) => <option key={item.id} value={item.slug}>{item.name}</option>)}</select></label>
    <label><span>Preço</span><select value={price} onChange={(e) => setPrice(e.target.value)}><option value="all">Todos</option><option value="under100">Até R$ 100</option><option value="100to180">R$ 100–180</option><option value="over180">Acima de R$ 180</option></select></label>
    <label><span>Cor</span><select value={current.get("color") ?? ""} onChange={(e) => update("color", e.target.value)}><option value="">Todas</option>{colors.map((item) => <option key={item.id} value={item.slug}>{item.name}</option>)}</select></label>
    <label><span>Ordenar</span><select value={current.get("sort") ?? "name_asc"} onChange={(e) => update("sort", e.target.value)}><option value="name_asc">Nome A–Z</option><option value="price_asc">Menor preço</option><option value="price_desc">Maior preço</option></select></label></div>
    <div className="results-line"><span>{meta.total} {meta.total === 1 ? "produto" : "produtos"}</span><Link href={pathname}>Limpar filtros</Link></div>
    {products.length ? <div className="product-grid">{products.map((product) => <ProductCard key={product.id} product={product} />)}</div> : <div className="empty-state"><h2>Nenhuma peça por aqui.</h2><p>Tente combinar outros filtros ou limpar sua busca.</p></div>}
    {meta.totalPages > 1 && <nav className="pagination" aria-label="Paginação">{meta.page > 1 && <button onClick={() => update("page", String(meta.page - 1))}>Anterior</button>}<span>Página {meta.page} de {meta.totalPages}</span>{meta.page < meta.totalPages && <button onClick={() => update("page", String(meta.page + 1))}>Próxima</button>}</nav>}</>;
}
