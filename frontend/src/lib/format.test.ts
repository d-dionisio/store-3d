import { describe, expect, it } from "vitest";
import { formatCurrency, formatDimensions, formatLeadTime } from "./format";
describe("formatação comercial", () => {
  it("converte centavos para BRL sem usar reais como entrada", () => expect(formatCurrency(18900)).toContain("189,00"));
  it("formata dimensões completas e protege dimensões ausentes", () => { expect(formatDimensions({ widthMm: 180, depthMm: 180, heightMm: 240 })).toBe("180 × 180 × 240 mm"); expect(formatDimensions({ widthMm: null, depthMm: 180, heightMm: 240 })).toBe("Sob consulta"); });
  it("formata prazo de produção", () => expect(formatLeadTime({ minDays: 5, maxDays: 8 })).toBe("5 a 8 dias úteis"));
});
