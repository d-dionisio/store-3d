import type { EditorialCatalog } from "@/services/editorial-api";
import type { Product, ProductImage } from "@/types/product";

export function primaryProductImage(product?: Product): ProductImage | undefined {
  return product?.images.find((image) => image.isPrimary) ?? product?.images[0];
}

export function catalogCover(catalog?: EditorialCatalog) {
  return catalog?.coverImageUrl || primaryProductImage(catalog?.products[0])?.url;
}

export function uniqueCatalogProducts(catalogs: EditorialCatalog[]): Product[] {
  const products = new Map<string, Product>();
  for (const catalog of catalogs) for (const product of catalog.products) products.set(product.id, product);
  return [...products.values()];
}

export function uniqueCatalogCategories(catalogs: EditorialCatalog[]) {
  const categories = new Map<string, Product["category"]>();
  for (const product of uniqueCatalogProducts(catalogs)) categories.set(product.category.id, product.category);
  return [...categories.values()];
}
