"use client";

import { Search, SlidersHorizontal } from "lucide-react";
import { useMemo, useState } from "react";
import { ProductCard } from "@/components/products/product-card";
import { products } from "@/data/products";
import type { ProductCategory } from "@/types/product";

const allCategories: (ProductCategory | "Todos")[] = ["Todos", "Decoração", "Natal", "Religiosos", "Acessórios", "Personalizados"];

export function Catalog() {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<ProductCategory | "Todos">("Todos");
  const [price, setPrice] = useState("all");
  const [color, setColor] = useState("all");
  const [sort, setSort] = useState("name");

  const filtered = useMemo(() => products.filter((product) => {
    const matchesQuery = `${product.name} ${product.description}`.toLowerCase().includes(query.toLowerCase());
    const matchesCategory = category === "Todos" || product.category === category;
    const matchesPrice = price === "all" || (price === "under100" ? product.price < 100 : price === "100to180" ? product.price >= 100 && product.price <= 180 : product.price > 180);
    const matchesColor = color === "all" || product.colors.includes(color);
    return matchesQuery && matchesCategory && matchesPrice && matchesColor;
  }).sort((a, b) => sort === "low" ? a.price - b.price : sort === "high" ? b.price - a.price : a.name.localeCompare(b.name)), [query, category, price, color, sort]);

  return <>
    <div className="catalog-toolbar">
      <label className="search-field"><Search size={18} /><span className="sr-only">Buscar produtos</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar por nome ou estilo" /></label>
      <div className="filter-title"><SlidersHorizontal size={18} /> Filtros</div>
      <label><span>Categoria</span><select value={category} onChange={(e) => setCategory(e.target.value as ProductCategory | "Todos")}>{allCategories.map((item) => <option key={item}>{item}</option>)}</select></label>
      <label><span>Preço</span><select value={price} onChange={(e) => setPrice(e.target.value)}><option value="all">Todos</option><option value="under100">Até R$ 100</option><option value="100to180">R$ 100–180</option><option value="over180">Acima de R$ 180</option></select></label>
      <label><span>Cor</span><select value={color} onChange={(e) => setColor(e.target.value)}><option value="all">Todas</option>{["Marfim", "Areia", "Grafite", "Violeta", "Lilás"].map((item) => <option key={item}>{item}</option>)}</select></label>
      <label><span>Ordenar</span><select value={sort} onChange={(e) => setSort(e.target.value)}><option value="name">Nome A–Z</option><option value="low">Menor preço</option><option value="high">Maior preço</option></select></label>
    </div>
    <div className="results-line"><span>{filtered.length} produtos demonstrativos</span><button onClick={() => { setQuery(""); setCategory("Todos"); setPrice("all"); setColor("all"); }}>Limpar filtros</button></div>
    {filtered.length ? <div className="product-grid">{filtered.map((product) => <ProductCard key={product.id} product={product} />)}</div> : <div className="empty-state"><h2>Nenhuma peça por aqui.</h2><p>Tente combinar outros filtros ou limpar sua busca.</p></div>}
  </>;
}

