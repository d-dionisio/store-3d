export const formatCurrency = (cents: number) => new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" }).format(cents / 100);
export const formatDimensions = (value: { widthMm: number | null; heightMm: number | null; depthMm: number | null }) => [value.widthMm, value.depthMm, value.heightMm].every((item) => item !== null) ? `${value.widthMm} × ${value.depthMm} × ${value.heightMm} mm` : "Sob consulta";
export const formatLeadTime = ({ minDays, maxDays }: { minDays: number; maxDays: number }) => minDays === maxDays ? `${minDays} dias úteis` : `${minDays} a ${maxDays} dias úteis`;

