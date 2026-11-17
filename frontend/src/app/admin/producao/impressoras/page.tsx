import Link from "next/link";
import {redirect} from "next/navigation";
import {AdminShell} from "@/components/admin/admin-shell";
import {PrinterManager} from "@/components/production/printer-manager";
import {PrinterMonitoringPanel} from "@/components/production/printer-monitoring";
import {PrinterTechnicalEditor} from "@/components/production/technical-resource-manager";
import {adminCookie} from "@/lib/admin-auth";
import {getPrinterConnection,getPrinterMonitoring,listMaterialTechnicalSpecs,listPrinters,listPrinterTechnicalSpecs,PrinterConnection,ProductionApiError} from "@/services/production-api";

export const dynamic="force-dynamic";

export default async function PrintersPage(){
  const cookie=await adminCookie();
  let printers,materials,specs;
  try{[printers,materials,specs]=await Promise.all([listPrinters(cookie),listMaterialTechnicalSpecs(cookie),listPrinterTechnicalSpecs(cookie)])}
  catch(error){if(error instanceof ProductionApiError&&error.status===401)redirect("/admin/login");throw error}
  const entries=await Promise.all(printers.map(async printer=>[printer.id,await getPrinterMonitoring(printer.id,cookie)] as const));
  const connectionEntries=await Promise.all(printers.map(async printer=>{
    try{return [printer.id,await getPrinterConnection(printer.id,cookie)] as const}
    catch(error){if(error instanceof ProductionApiError&&error.status===404)return [printer.id,null] as const;throw error}
  }));
  return <AdminShell>
    <header className="admin-header production-heading"><span>Recursos produtivos</span><h1>Impressoras</h1><p>Cadastro operacional, capacidade técnica e telemetria observada permanecem separados.</p><nav><Link href="/admin/producao">Produção</Link><Link href="/admin/producao/compatibilidade">Compatibilidade</Link><Link href="/admin/producao/materiais">Materiais</Link></nav></header>
    <PrinterMonitoringPanel printers={printers} initial={Object.fromEntries(entries)} initialConnections={Object.fromEntries(connectionEntries) as Record<string,PrinterConnection|null>}/>
    <PrinterManager printers={printers}/><PrinterTechnicalEditor printers={printers} materials={materials} specs={specs}/>
  </AdminShell>
}
