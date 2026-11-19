import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { AdminShell } from "@/components/admin/admin-shell";
import { CostsNav, CostReports, type CostReportData } from "@/components/production/cost-management";
import { adminCookie } from "@/lib/admin-auth";
import { CostAnalytics, CostEstimateComparison, getCostAnalytics, getCostEstimateComparison, getCostProfitability, getPrinterCostProfitability, PrinterCostProfitability, ProductCostProfitability, ProductionApiError } from "@/services/production-api";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Rentabilidade da produção" };
export default async function ManufacturingProfitabilityPage({ searchParams }: { searchParams: Promise<Record<string, string | undefined>> }) {
  const params = await searchParams; const from = params.from ?? ""; const to = params.to ?? ""; const cookie = await adminCookie();
  let data: CostReportData | null = null; let loadError: unknown;
  try {
    const [analytics, products, printers, comparison]: [CostAnalytics, ProductCostProfitability[], PrinterCostProfitability[], CostEstimateComparison[]] = await Promise.all([
      getCostAnalytics(cookie, { from, to }), getCostProfitability(cookie, { from, to }), getPrinterCostProfitability(cookie, { from, to }), getCostEstimateComparison(cookie, { from, to }),
    ]); data = { analytics, products, printers, comparison };
  } catch (error) {
    if (error instanceof ProductionApiError && error.status === 401) redirect("/admin/login");
    loadError = error;
  }
  if (loadError || !data) return <AdminShell><header className="admin-header production-heading"><span>Margens e snapshots realizados</span><h1>Rentabilidade</h1></header><CostsNav active="profitability"/><section className="costs-panel"><h2>{loadError instanceof ProductionApiError && loadError.status === 403 ? "Acesso não autorizado" : "Relatórios indisponíveis"}</h2><p role="alert">{loadError instanceof Error ? loadError.message : "Não foi possível carregar os relatórios."}</p></section></AdminShell>;
  return <AdminShell><header className="admin-header production-heading"><span>Margens e snapshots realizados</span><h1>Rentabilidade</h1><p>Relatórios consolidados pelo PostgreSQL usando snapshots realizados; valores históricos não dependem das configurações atuais.</p></header><CostsNav active="profitability"/><CostReports data={data} from={from} to={to} mode="profitability"/></AdminShell>;
}
