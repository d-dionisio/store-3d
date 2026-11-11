import { beforeEach, describe, expect, it, vi } from "vitest";
import { getProducts } from "@/services/catalog-api";
import { getCollectionProducts } from "@/services/catalog-collections";
import type { Product } from "@/types/product";

vi.mock("@/services/catalog-api", () => ({ getProducts: vi.fn() }));

const product = (id: string) => ({ id } as Product);
const getProductsMock = vi.mocked(getProducts);

describe("getCollectionProducts", () => {
  beforeEach(() => getProductsMock.mockReset());

  it("consulta todas as páginas da coleção sem duplicar os dados comerciais", async () => {
    getProductsMock
      .mockResolvedValueOnce({ data: [product("1")], meta: { page: 1, limit: 24, total: 3, totalPages: 3 } })
      .mockResolvedValueOnce({ data: [product("2")], meta: { page: 2, limit: 24, total: 3, totalPages: 3 } })
      .mockResolvedValueOnce({ data: [product("3")], meta: { page: 3, limit: 24, total: 3, totalPages: 3 } });

    await expect(getCollectionProducts("decoracao")).resolves.toEqual([product("1"), product("2"), product("3")]);
    expect(getProductsMock).toHaveBeenNthCalledWith(1, { category: "decoracao", page: 1, limit: 24, sort: "name_asc" });
    expect(getProductsMock).toHaveBeenNthCalledWith(3, { category: "decoracao", page: 3, limit: 24, sort: "name_asc" });
  });
});
