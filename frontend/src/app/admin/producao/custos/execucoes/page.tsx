import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { AdminShell } from "@/components/admin/admin-shell";
import { CostsNav, CostExecutionManager } from "@/components/production/cost-management";
import { adminCookie } from "@/lib/admin-auth";
import { listProductionHistory, ProductionApiError, type ProductionHistoryItem } from "@/services/production-api";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Custos por execução" };
export default async function RunCostsPage() {
  const cookie = await adminCookie();
  let runs: ProductionHistoryItem[] = []; let loadError: unknown;
  try { runs = (await listProductionHistory(cookie, { page: 1, limit: 100 })).items; }
  catch (error) { if (error instanceof ProductionApiError && error.status === 401) redirect("/admin/login"); loadError = error; }
  if (loadError) return <AdminShell><header className="admin-header production-heading"><span>Snapshots de fabricação</span><h1>Custos por execução</h1></header><CostsNav active="runs"/><section className="costs-panel"><h2>{loadError instanceof ProductionApiError && loadError.status === 403 ? "Acesso não autorizado" : "Histórico indisponível"}</h2><p role="alert">{loadError instanceof Error ? loadError.message : "Não foi possível carregar as execuções."}</p></section></AdminShell>;
  return <AdminShell><header className="admin-header production-heading"><span>Snapshots de fabricação</span><h1>Custos por execução</h1><p>Consulte snapshots de estimativa e realizados vinculados a execuções reais.</p></header><CostsNav active="runs"/><CostExecutionManager runs={runs ?? []}/></AdminShell>;
}
