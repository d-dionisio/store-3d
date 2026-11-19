import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { AdminShell } from "@/components/admin/admin-shell";
import { CostsNav, CostProfileManager } from "@/components/production/cost-management";
import { adminCookie } from "@/lib/admin-auth";
import { listPrinters, ProductionApiError, type Printer } from "@/services/production-api";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Perfis de custos por impressora" };
export default async function PrinterCostProfilesPage() {
  const cookie = await adminCookie();
  let printers: Printer[] = []; let loadError: unknown;
  try { printers = await listPrinters(cookie); }
  catch (error) { if (error instanceof ProductionApiError && error.status === 401) redirect("/admin/login"); loadError = error; }
  if (loadError) return <AdminShell><header className="admin-header production-heading"><span>Parâmetros versionados</span><h1>Custos por impressora</h1></header><CostsNav active="printers"/><section className="costs-panel"><h2>{loadError instanceof ProductionApiError && loadError.status === 403 ? "Acesso não autorizado" : "Impressoras indisponíveis"}</h2><p role="alert">{loadError instanceof Error ? loadError.message : "Não foi possível carregar a lista de impressoras."}</p></section></AdminShell>;
  return <AdminShell><header className="admin-header production-heading"><span>Parâmetros versionados</span><h1>Custos por impressora</h1><p>Configure somente dados medidos ou fornecidos pela operação. Cada salvamento cria uma nova versão.</p></header><CostsNav active="printers"/><CostProfileManager printers={printers}/></AdminShell>;
}
