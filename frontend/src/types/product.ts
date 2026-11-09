export type ProductCategory = "Decoração" | "Natal" | "Religiosos" | "Acessórios" | "Personalizados";

export type MockProduct = {
  id: string;
  slug: string;
  name: string;
  description: string;
  price: number;
  category: ProductCategory;
  colors: string[];
  dimensions: string;
  material: string;
  leadTime: string;
  imagePosition: "top-left" | "top-right" | "bottom-left" | "bottom-right";
  featured?: boolean;
};

export type Category = { id: string; slug: string; name: string; description: string };
export type Material = { id: string; code: string; name: string };
export type Color = { id: string; slug: string; name: string; hexCode: string };
export type VariantAttribute = { id: string; code: string; name: string };
export type ProductImage = { id: string; variantId: string | null; url: string; altText: string; mediaType: string; sortOrder: number; isPrimary: boolean };
export type ProductVariant = { id: string; sku: string; name: string | null; priceCents: number; availableQuantity: number; trackInventory: boolean; weightGrams: number | null; dimensions: { widthMm: number | null; heightMm: number | null; depthMm: number | null }; productionLeadTime: { minDays: number; maxDays: number }; material: Material; color: Color; size: VariantAttribute | null; finish: VariantAttribute | null };
export type Product = { id: string; slug: string; name: string; shortDescription: string; description: string; isFeatured: boolean; productionType: string; minPriceCents: number; category: Category; images: ProductImage[]; materials: Material[]; colors: Color[]; variants: ProductVariant[] };
export type CartItem = { productId: string; productSlug: string; productVariantId: string; quantity: number };
export type PaginationMeta = { page: number; limit: number; total: number; totalPages: number };

