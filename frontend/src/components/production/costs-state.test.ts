import { describe, expect, it } from "vitest";
import { buildCostProfileInput, costErrorMessage, formatCostMoney, formatCostPercent, parseCostInteger, parseCostMoney } from "./costs-state";

describe("manufacturing cost UI state", () => {
  it("formats cent-based values and preserves unknown values", () => {
    expect(formatCostMoney(24900)).toContain("249,00");
    expect(formatCostMoney(null)).toBe("Não disponível");
    expect(formatCostPercent(1250)).toBe("12,50%");
    expect(formatCostPercent(null)).toBe("Não disponível");
  });

  it("parses editable currency as integer cents without floating point", () => {
    expect(parseCostMoney("249,90")).toBe(24990);
    expect(parseCostMoney("0")).toBe(0);
    expect(parseCostMoney("1.234,56")).toBe(123456);
    expect(parseCostMoney("12.34,56")).toBeNull();
    expect(parseCostMoney("-10,00")).toBeNull();
    expect(parseCostMoney("2,999")).toBeNull();
  });

  it("builds the documented profile DTO with cent precision and optimistic version", () => {
    const input = buildCostProfileInput({
      electricity_rate: "0,92", printer_power_milliwatts: "35000", power_measurement_note: "Leitura medida",
      machine_acquisition: "1.299,00", machine_useful_life_hours: "4000", maintenance_rate: "2,50",
      labor_rate: "35,75", preparation_seconds: "120", finishing_seconds: "90", notes: "  Perfil  ",
    }, 7);
    expect(input).toMatchObject({
      electricity_rate_cents_per_kwh: 92, printer_power_milliwatts: 35000,
      machine_acquisition_cents: 129900, machine_useful_life_seconds: 14400000,
      maintenance_rate_cents_per_hour: 250, labor_rate_cents_per_hour: 3575,
      preparation_seconds: 120, finishing_seconds: 90, notes: "Perfil", expected_version: 7,
    });
  });

  it("requires a measurement note for unknown power and rejects malformed integers", () => {
    const input = { electricity_rate: "0", printer_power_milliwatts: "", power_measurement_note: "", machine_acquisition: "0", machine_useful_life_hours: "1", maintenance_rate: "0", labor_rate: "0", preparation_seconds: "0", finishing_seconds: "0", notes: "" };
    expect(() => buildCostProfileInput(input, 0)).toThrow("origem da medição");
    expect(parseCostInteger("1.5")).toBeNull();
    expect(parseCostInteger("0", false)).toBeNull();
  });

  it("explains API authorization and optimistic-concurrency errors", () => {
    expect(costErrorMessage({ status: 403 })).toContain("permissão");
    expect(costErrorMessage({ status: 409 })).toContain("alterados");
    expect(costErrorMessage({ status: 401 })).toContain("sessão expirou");
  });
});
