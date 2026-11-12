import { getApiUrl } from "@/lib/env";
import type { Color, Material, PaginationMeta, Product, ProductImage, ProductVariant, VariantAttribute } from "@/types/product";
type ErrorDTO = { error?: { code?: string; message?: string } };
export class ApiError extends Error { constructor(message: string, public status: number, public code = "api_error") { super(message); } }
type CategoryDTO = { id: string; slug: string; name: string; description: string };
type AttributeDTO = { id: string; code: string; name: string };
type ColorDTO = { id: string; slug: string; name: string; hex_code: string };
type ImageDTO = { id: string; variant_id: string | null; url: string; alt_text: string; media_type: string; sort_order: number; is_primary: boolean };
type VariantDTO = { id: string; sku: string; name: string | null; price_cents: number; available_quantity: number; track_inventory: boolean; weight_grams: number | null; dimensions: { width_mm: number | null; height_mm: number | null; depth_mm: number | null }; production_lead_time: { min_days: number; max_days: number }; material: AttributeDTO; color: ColorDTO; size?: AttributeDTO | null; finish?: AttributeDTO | null };
export type ProductDTO = { id: string; slug: string; name: string; short_description: string; description: string; is_featured: boolean; production_type: string; min_price_cents: number; category: CategoryDTO; images: ImageDTO[]; materials: AttributeDTO[]; colors: ColorDTO[]; variants: VariantDTO[] };
type ListDTO = { data: ProductDTO[]; meta: { page: number; limit: number; total: number; total_pages: number } };
const material = (v: AttributeDTO): Material => v;
const attribute = (v?: AttributeDTO | null): VariantAttribute | null => v ?? null;
const color = (v: ColorDTO): Color => ({ id: v.id, slug: v.slug, name: v.name, hexCode: v.hex_code });
const image = (v: ImageDTO): ProductImage => ({ id: v.id, variantId: v.variant_id, url: v.url, altText: v.alt_text, mediaType: v.media_type, sortOrder: v.sort_order, isPrimary: v.is_primary });
const variant = (v: VariantDTO): ProductVariant => ({ id: v.id, sku: v.sku, name: v.name, priceCents: v.price_cents, availableQuantity: v.available_quantity, trackInventory: v.track_inventory, weightGrams: v.weight_grams, dimensions: { widthMm: v.dimensions.width_mm, heightMm: v.dimensions.height_mm, depthMm: v.dimensions.depth_mm }, productionLeadTime: { minDays: v.production_lead_time.min_days, maxDays: v.production_lead_time.max_days }, material: material(v.material), color: color(v.color), size: attribute(v.size), finish: attribute(v.finish) });
export const mapProduct = (v: ProductDTO): Product => ({ id: v.id, slug: v.slug, name: v.name, shortDescription: v.short_description, description: v.description, isFeatured: v.is_featured, productionType: v.production_type, minPriceCents: v.min_price_cents, category: v.category, images: v.images.map(image), materials: v.materials.map(material), colors: v.colors.map(color), variants: v.variants.map(variant) });
async function request<T>(path: string, signal?: AbortSignal, revalidate?: number): Promise<T> {
  const controller = new AbortController(); const timer = setTimeout(() => controller.abort(), 8000); const abort = () => controller.abort(); signal?.addEventListener("abort", abort, { once: true });
  try { const response = await fetch(`${getApiUrl()}/api/v1${path}`, { headers: { Accept: "application/json" }, signal: controller.signal, ...(revalidate === undefined ? { cache: "no-store" as const } : { next: { revalidate } }) });
    if (!response.ok) { const body = await response.json().catch(() => ({})) as ErrorDTO; throw new ApiError(body.error?.message ?? "Não foi possível consultar a API.", response.status, body.error?.code); }
    return await response.json() as T;
  } catch (error) { if (error instanceof ApiError) throw error; throw new ApiError(error instanceof DOMException && error.name === "AbortError" ? "A API demorou para responder." : "API indisponível no momento.", 0, "unavailable"); }
  finally { clearTimeout(timer); signal?.removeEventListener("abort", abort); }
}
export type ProductFilters = { category?: string; search?: string; color?: string; minPrice?: number; maxPrice?: number; sort?: string; page?: number; limit?: number };
export function productQuery(filters: ProductFilters) { const p = new URLSearchParams(); const values: [string, string | number | undefined][] = [["category", filters.category], ["search", filters.search], ["color", filters.color], ["min_price", filters.minPrice], ["max_price", filters.maxPrice], ["sort", filters.sort], ["page", filters.page], ["limit", filters.limit]]; values.forEach(([k, v]) => { if (v !== undefined && v !== "") p.set(k, String(v)); }); return p.toString(); }
export async function getProducts(filters: ProductFilters = {}) { const query = productQuery(filters); const dto = await request<ListDTO>(`/products${query ? `?${query}` : ""}`); const meta: PaginationMeta = { page: dto.meta.page, limit: dto.meta.limit, total: dto.meta.total, totalPages: dto.meta.total_pages }; return { data: dto.data.map(mapProduct), meta }; }
export async function getProduct(slug: string, signal?: AbortSignal) { const dto = await request<{ data: ProductDTO }>(`/products/${encodeURIComponent(slug)}`, signal); return mapProduct(dto.data); }
export async function getCategories() { const dto = await request<{ data: CategoryDTO[] }>("/categories", undefined, 300); return dto.data; }
