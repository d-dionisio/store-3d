import type { ProductFilters } from "@/services/catalog-api";
type Params = Record<string, string | string[] | undefined>;
const one = (value: string | string[] | undefined) => Array.isArray(value) ? value[0] : value;
function positiveInteger(value: string | undefined, name: string) { if (value === undefined) return undefined; if (!/^\d+$/.test(value) || Number(value) < 1) throw new Error(`Filtro ${name} inválido.`); return Number(value); }
export function parseCatalogFilters(params: Params): ProductFilters { const minPrice = positiveInteger(one(params.min_price), "min_price"); const maxPrice = positiveInteger(one(params.max_price), "max_price"); if (minPrice !== undefined && maxPrice !== undefined && minPrice > maxPrice) throw new Error("Faixa de preço inválida."); return { category: one(params.category), search: one(params.search), color: one(params.color), minPrice, maxPrice, sort: one(params.sort), page: positiveInteger(one(params.page), "page") ?? 1, limit: 12 }; }
