import type { ProductionApiError } from "@/services/production-api";
import type { CostProfileInput } from "@/services/production-api";

export function formatCostMoney(cents: number | null | undefined): string {
  if (cents == null || !Number.isSafeInteger(cents)) return "Não disponível";
  return new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" }).format(cents / 100);
}

export function formatCostPercent(basisPoints: number | null | undefined): string {
  if (basisPoints == null) return "Não disponível";
  return `${(basisPoints / 100).toLocaleString("pt-BR", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}%`;
}

export function parseCostMoney(value: string): number | null {
  const input = value.trim();
  const normalized = input.includes(",") && input.includes(".")
    ? (/^\d{1,3}(?:\.\d{3})+,\d{1,2}$/.test(input) ? input.replaceAll(".", "").replace(",", ".") : "")
    : input.replace(",", ".");
  if (!/^\d{1,12}(?:\.\d{1,2})?$/.test(normalized)) return null;
  const [whole, fraction = ""] = normalized.split(".");
  const cents = Number(`${whole}${fraction.padEnd(2, "0")}`);
  return Number.isSafeInteger(cents) ? cents : null;
}

export function parseCostInteger(value: string, allowZero = true): number | null {
  const normalized = value.trim();
  if (!/^\d+$/.test(normalized)) return null;
  const parsed = Number(normalized);
  return Number.isSafeInteger(parsed) && (allowZero || parsed > 0) ? parsed : null;
}

export type CostProfileFormValues = {
  electricity_rate: string;
  printer_power_milliwatts: string;
  power_measurement_note: string;
  machine_acquisition: string;
  machine_useful_life_hours: string;
  maintenance_rate: string;
  labor_rate: string;
  preparation_seconds: string;
  finishing_seconds: string;
  notes: string;
};

export function buildCostProfileInput(values: CostProfileFormValues, expectedVersion: number): CostProfileInput {
  const money = (value: string, label: string) => {
    const cents = parseCostMoney(value);
    if (cents == null) throw new Error(`${label}: informe um valor em reais com até duas casas decimais.`);
    return cents;
  };
  const integer = (value: string, label: string, allowZero = true) => {
    const parsed = parseCostInteger(value, allowZero);
    if (parsed == null) throw new Error(`${label}: informe um número inteiro válido.`);
    return parsed;
  };
  const power = values.printer_power_milliwatts.trim()
    ? integer(values.printer_power_milliwatts, "Potência medida", false)
    : null;
  const powerNote = values.power_measurement_note.trim();
  if (power == null && !powerNote) throw new Error("Informe a origem da medição de energia quando a potência não estiver configurada.");
  const lifeHours = integer(values.machine_useful_life_hours, "Vida útil (horas)", false);
  if (!Number.isSafeInteger(lifeHours * 3600)) throw new Error("Vida útil (horas): valor inválido.");
  return {
    electricity_rate_cents_per_kwh: money(values.electricity_rate, "Tarifa elétrica"),
    printer_power_milliwatts: power,
    power_measurement_note: powerNote,
    machine_acquisition_cents: money(values.machine_acquisition, "Aquisição da máquina"),
    machine_useful_life_seconds: lifeHours * 3600,
    maintenance_rate_cents_per_hour: money(values.maintenance_rate, "Manutenção por hora"),
    labor_rate_cents_per_hour: money(values.labor_rate, "Mão de obra por hora"),
    preparation_seconds: integer(values.preparation_seconds, "Preparação"),
    finishing_seconds: integer(values.finishing_seconds, "Acabamento"),
    notes: values.notes.trim(),
    expected_version: expectedVersion,
  };
}

export function costErrorMessage(error: unknown): string {
  const status = (error as ProductionApiError | undefined)?.status;
  if (status === 401) return "Sua sessão expirou. Entre novamente para continuar.";
  if (status === 403) return "Seu perfil não tem permissão para consultar ou alterar custos.";
  if (status === 404) return "O recurso ainda não possui configuração ou não está disponível.";
  if (status === 409) return "Os dados foram alterados por outra pessoa. Atualize a tela e tente novamente.";
  if (status === 503) return "A operação não foi confirmada com segurança. Tente novamente mais tarde.";
  return error instanceof Error ? error.message : "Não foi possível concluir a operação.";
}
