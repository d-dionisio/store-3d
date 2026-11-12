import { describe, expect, it } from "vitest";
import type { EditorialCatalog } from "@/services/editorial-api";
import type { Product } from "@/types/product";
import { catalogCover, primaryProductImage, uniqueCatalogCategories, uniqueCatalogProducts } from "./showroom";

const product = (id: string, categoryId = "decoracao") => ({
  id,
  category: { id: categoryId, slug: categoryId, name: categoryId, description: "" },
  images: [{ id: `${id}-secondary`, isPrimary: false, url: "/secondary.png" }, { id: `${id}-primary`, isPrimary: true, url: "/primary.png" }],
} as Product);

const catalog = (id: string, products: Product[], coverImageUrl = "") => ({ id, products, coverImageUrl } as EditorialCatalog);

describe("showroom editorial", () => {
  it("prioriza a imagem principal e respeita a capa editorial", () => {
    expect(primaryProductImage(product("p1"))?.url).toBe("/primary.png");
    expect(catalogCover(catalog("c1", [product("p1")], "/cover.png"))).toBe("/cover.png");
  });

  it("remove produtos e categorias repetidos sem alterar os dados comerciais", () => {
    const first = product("p1");
    const second = product("p2", "natal");
    const catalogs = [catalog("c1", [first, second]), catalog("c2", [first])];
    expect(uniqueCatalogProducts(catalogs).map((item) => item.id)).toEqual(["p1", "p2"]);
    expect(uniqueCatalogCategories(catalogs).map((item) => item.id)).toEqual(["decoracao", "natal"]);
  });
});
