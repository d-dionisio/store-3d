export function parseOptionalBoolean(value: string): boolean | null {
  if (value === "") return null;
  return value === "yes";
}

export function technicalStatusLabel(status: string): string {
  return ({
    compatible: "Compatível",
    compatible_with_caveats: "Compatível com ressalvas",
    incompatible: "Incompatível",
    insufficient_data: "Dados insuficientes",
  } as Record<string, string>)[status] ?? status;
}
