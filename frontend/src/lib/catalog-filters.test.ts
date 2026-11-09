import { describe, expect, it } from "vitest";
import { parseCatalogFilters } from "./catalog-filters";
describe("filtros do catálogo", () => {
  it("converte paginação e preços válidos", () => expect(parseCatalogFilters({ page: "2", min_price: "1000" })).toMatchObject({ page: 2, minPrice: 1000, limit: 12 }));
  it("rejeita valores não numéricos", () => expect(() => parseCatalogFilters({ min_price: "dez" })).toThrow("min_price"));
  it("rejeita faixa invertida", () => expect(() => parseCatalogFilters({ min_price: "200", max_price: "100" })).toThrow("Faixa"));
});
