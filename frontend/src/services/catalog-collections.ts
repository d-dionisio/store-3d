import { getProducts } from "@/services/catalog-api";
import type { Product } from "@/types/product";

const PAGE_SIZE = 24;

export async function getCollectionProducts(category: string): Promise<Product[]> {
  const first = await getProducts({ category, page: 1, limit: PAGE_SIZE, sort: "name_asc" });
  if (first.meta.totalPages <= 1) return first.data;

  const remaining = await Promise.all(
    Array.from({ length: first.meta.totalPages - 1 }, (_, index) =>
      getProducts({ category, page: index + 2, limit: PAGE_SIZE, sort: "name_asc" }),
    ),
  );
  return [first, ...remaining].flatMap((page) => page.data);
}
