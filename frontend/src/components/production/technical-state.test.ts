import { describe, expect, it } from "vitest";
import { parseOptionalBoolean, technicalStatusLabel } from "./technical-state";

describe("technical form state", () => {
  it("preserves an unknown boolean instead of assuming false", () => {
    expect(parseOptionalBoolean("")).toBeNull();
    expect(parseOptionalBoolean("yes")).toBe(true);
    expect(parseOptionalBoolean("no")).toBe(false);
  });

  it("labels insufficient data explicitly", () => {
    expect(technicalStatusLabel("insufficient_data")).toBe("Dados insuficientes");
    expect(technicalStatusLabel("incompatible")).toBe("Incompatível");
  });
});
