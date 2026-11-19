import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { AdminShell } from "@/components/admin/admin-shell";
import { CostsNav, CostReports, type CostReportData } from "@/components/production/cost-management";
import { adminCookie } from "@/lib/admin-auth";
import { CostAnalytics, CostEstimateComparison, getCostAnalytics, getCostEstimateComparison, getCostProfitability, getPrinterCostProfitability, PrinterCostProfitability, ProductCostProfitability, ProductionApiError } from "@/services/production-api";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Custos de fabricação" };

export default async function ManufacturingCostsPage({ searchParams }: { searchParams: Promise<Record<string, string | undefined>> }) {
  const params = await searchParams;
  const from = params.from ?? ""; const to = params.to ?? ""; const cookie = await adminCookie();
  let data: CostReportData | null = null; let loadError: unknown;
  try {
    const [analytics, products, printers, comparison]: [CostAnalytics, ProductCostProfitability[], PrinterCostProfitability[], CostEstimateComparison[]] = await Promise.all([
      getCostAnalytics(cookie, { from, to }), getCostProfitability(cookie, { from, to }), getPrinterCostProfitability(cookie, { from, to }), getCostEstimateComparison(cookie, { from, to }),
    ]);
    data = { analytics, products, printers, comparison };
  } catch (error) {
    if (error instanceof ProductionApiError && error.status === 401) redirect("/admin/login");
    loadError = error;
  }
  if (loadError || !data) return <AdminShell><header className="admin-header production-heading"><span>Gestão financeira da produção</span><h1>Custos de fabricação</h1><p>Dados entregues pelos relatórios administrativos existentes.</p></header><CostsNav active="dashboard"/><section className="costs-panel"><h2>{loadError instanceof ProductionApiError && loadError.status === 403 ? "Acesso não autorizado" : "Relatórios indisponíveis"}</h2><p role="alert">{loadError instanceof Error ? loadError.message : "Não foi possível carregar os relatórios de custos."}</p>{loadError instanceof ProductionApiError && loadError.status === 403 && <p>A API autoriza relatórios financeiros para admin e production_manager.</p>}<Link href="/admin/producao">Voltar à produção</Link></section></AdminShell>;
  return <AdminShell><header className="admin-header production-heading"><span>Gestão financeira da produção</span><h1>Custos de fabricação</h1><p>Snapshots históricos e indicadores fornecidos pelo backend, sem recalcular valores no navegador.</p></header><CostsNav active="dashboard"/><CostReports data={data} from={from} to={to} mode="dashboard"/></AdminShell>;
}
