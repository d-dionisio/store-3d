"use client";

import Link from "next/link";
import { FormEvent, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { AlertTriangle, ArrowUpRight, RefreshCw, Save, ShieldCheck } from "lucide-react";
import { requestCostReauthentication, getCostProfile, getCostProfileHistory, saveCostProfile, getRunCostSnapshots, createRunCostSnapshot, type CostProfile, type CostProfileInput, type CostSnapshot, type Printer, type ProductionHistoryItem } from "@/services/production-api";
import { buildCostProfileInput, costErrorMessage, formatCostMoney, formatCostPercent, parseCostInteger } from "./costs-state";

type SensitiveAction = (proof: string) => Promise<void>;

function ReauthenticationDialog({ action, onClose, onSuccess }: { action: SensitiveAction | null; onClose: () => void; onSuccess: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => { const dialog = ref.current; if (action && !dialog?.open) dialog?.showModal(); if (!action && dialog?.open) dialog.close(); }, [action]);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!action || busy) return;
    setBusy(true); setError(""); const data = new FormData(event.currentTarget);
    try {
      const proof = await requestCostReauthentication(String(data.get("password")), String(data.get("code") || ""), String(data.get("recovery_code") || ""));
      await action(proof.reauthentication_token); onSuccess(); onClose();
    } catch (cause) { setError(costErrorMessage(cause)); } finally { setBusy(false); }
  }
  return <dialog ref={ref} className="admin-dialog costs-dialog" onCancel={event => { event.preventDefault(); onClose(); }} aria-labelledby="cost-reauth-title"><form onSubmit={submit}><span className="admin-kicker">Ação administrativa sensível</span><h2 id="cost-reauth-title">Confirme sua identidade</h2><p>A prova será obtida pelo fluxo MFA atual e enviada uma única vez. Ela não é armazenada no navegador.</p><label>Senha atual<input name="password" type="password" autoComplete="current-password" required autoFocus /></label><label>Código TOTP<input name="code" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" /></label><label>Ou código de recuperação<input name="recovery_code" autoComplete="off" /></label>{error && <p role="alert" className="admin-alert">{error}</p>}<div className="cost-actions"><button type="button" className="cost-button secondary" onClick={onClose}>Cancelar</button><button className="cost-button primary" disabled={busy}>{busy ? "Confirmando…" : "Confirmar e executar"}</button></div></form></dialog>;
}

export function CostsNav({ active }: { active: "dashboard" | "printers" | "runs" | "profitability" }) {
  const items = [["dashboard", "/admin/producao/custos", "Visão geral"], ["printers", "/admin/producao/custos/impressoras", "Perfis por impressora"], ["runs", "/admin/producao/custos/execucoes", "Execuções"], ["profitability", "/admin/producao/custos/rentabilidade", "Rentabilidade"]] as const;
  return <nav className="costs-nav" aria-label="Navegação de custos">{items.map(([key, href, label]) => <Link key={key} href={href} aria-current={active === key ? "page" : undefined}>{label}<ArrowUpRight aria-hidden="true" /></Link>)}</nav>;
}

function CostPermissionNote() { return <p className="costs-permission"><ShieldCheck aria-hidden="true" /> A API aplica a autorização definitiva. Perfis, snapshots e relatórios financeiros exigem admin ou production_manager.</p>; }

function centsInput(value: number | undefined) { if (value == null) return ""; return `${Math.trunc(value / 100)}.${String(value % 100).padStart(2, "0")}`; }
export function CostProfileManager({ printers }: { printers: Printer[] }) {
  const router = useRouter();
  const [printerID, setPrinterID] = useState(printers[0]?.id ?? "");
  const [profile, setProfile] = useState<CostProfile | null>(null);
  const [history, setHistory] = useState<CostProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [pending, setPending] = useState<SensitiveAction | null>(null);
  const [reload, setReload] = useState(0);
  const selectedPrinter = printers.find(printer => printer.id === printerID);

  useEffect(() => {
    if (!printerID) return;
    let active = true;
    Promise.all([getCostProfile(printerID).catch(cause => { if ((cause as { status?: number }).status === 404) return null; throw cause; }), getCostProfileHistory(printerID)])
      .then(([current, versions]) => { if (active) { setProfile(current); setHistory(versions); } })
      .catch(cause => { if (active) setError(costErrorMessage(cause)); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [printerID, reload]);

  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const form = event.currentTarget; const data = new FormData(form);
    let input: CostProfileInput;
    try {
      input = buildCostProfileInput({
        electricity_rate: String(data.get("electricity_rate") ?? ""),
        printer_power_milliwatts: String(data.get("printer_power_milliwatts") ?? ""),
        power_measurement_note: String(data.get("power_measurement_note") ?? ""),
        machine_acquisition: String(data.get("machine_acquisition") ?? ""),
        machine_useful_life_hours: String(data.get("machine_useful_life_hours") ?? ""),
        maintenance_rate: String(data.get("maintenance_rate") ?? ""),
        labor_rate: String(data.get("labor_rate") ?? ""),
        preparation_seconds: String(data.get("preparation_seconds") ?? ""),
        finishing_seconds: String(data.get("finishing_seconds") ?? ""),
        notes: String(data.get("notes") ?? ""),
      }, profile?.version ?? 0);
    } catch (cause) { setError(costErrorMessage(cause)); return; }
    if (!window.confirm(`Salvar nova versão do perfil de custos de ${selectedPrinter?.name ?? "impressora"}? Versões anteriores não serão alteradas.`)) return;
    setError(""); setMessage("");
    setPending(() => async (proof: string) => { await saveCostProfile(printerID, input, proof); setMessage("Nova versão do perfil registrada."); router.refresh(); });
  }

  const p = profile;
  return <><CostPermissionNote /><section className="costs-panel"><header className="costs-panel-heading"><div><span className="admin-kicker">Configuração versionada</span><h2>Perfil por impressora</h2><p>Valores desconhecidos permanecem sem preenchimento; potência não é presumida.</p></div><button className="cost-button secondary" onClick={() => setReload(value => value + 1)} disabled={loading}><RefreshCw aria-hidden="true"/> Atualizar</button></header>
    {printers.length ? <label className="costs-printer-select">Impressora<select value={printerID} onChange={event => { setLoading(true); setError(""); setMessage(""); setPrinterID(event.target.value); }}>{printers.map(printer => <option key={printer.id} value={printer.id}>{printer.name} · {printer.manufacturer} {printer.model}</option>)}</select></label> : <div className="costs-empty"><h3>Nenhuma impressora cadastrada</h3><p>Cadastre uma impressora na área de produção antes de configurar custos.</p><Link href="/admin/producao/impressoras">Abrir impressoras</Link></div>}
    {loading && <div className="costs-skeleton" role="status">Carregando perfil e histórico…</div>}{error && <p className="admin-alert" role="alert">{error}</p>}{message && <p className="costs-success" role="status">{message}</p>}
    {!loading && printerID && <>
      {p && <div className="costs-version-banner"><span>Versão vigente <strong>v{p.version}</strong></span><span>Atualizada em {new Date(p.created_at).toLocaleString("pt-BR", { timeZone: "America/Sao_Paulo" })}</span></div>}
      {!p && !error && <p className="costs-not-configured"><AlertTriangle aria-hidden="true"/> Perfil ainda não configurado. Salvar criará a versão 1.</p>}
      <form className="costs-form" key={`${printerID}-${p?.version ?? 0}`} onSubmit={save}>
        <label>Tarifa elétrica (R$/kWh)<input name="electricity_rate" inputMode="decimal" placeholder="0,00" defaultValue={centsInput(p?.electricity_rate_cents_per_kwh)} required /></label>
        <label>Potência medida (mW)<input name="printer_power_milliwatts" type="number" min="1" step="1" placeholder="Não configurada" defaultValue={p?.printer_power_milliwatts ?? ""}/></label>
        <label className="costs-span">Origem/observação da medição<input name="power_measurement_note" maxLength={500} defaultValue={p?.power_measurement_note ?? ""} placeholder="Obrigatória quando não houver potência configurada"/></label>
        <label>Aquisição da máquina (R$)<input name="machine_acquisition" inputMode="decimal" placeholder="0,00" defaultValue={centsInput(p?.machine_acquisition_cents)} required/></label>
        <label>Vida útil estimada (horas)<input name="machine_useful_life_hours" type="number" min="1" step="1" defaultValue={p ? p.machine_useful_life_seconds / 3600 : ""} required/></label>
        <label>Manutenção (R$/hora)<input name="maintenance_rate" inputMode="decimal" placeholder="0,00" defaultValue={centsInput(p?.maintenance_rate_cents_per_hour)} required/></label>
        <label>Mão de obra (R$/hora)<input name="labor_rate" inputMode="decimal" placeholder="0,00" defaultValue={centsInput(p?.labor_rate_cents_per_hour)} required/></label>
        <label>Preparação padrão (segundos)<input name="preparation_seconds" type="number" min="0" step="1" defaultValue={p?.preparation_seconds ?? ""} required/></label>
        <label>Acabamento padrão (segundos)<input name="finishing_seconds" type="number" min="0" step="1" defaultValue={p?.finishing_seconds ?? ""} required/></label>
        <label className="costs-span">Notas<input name="notes" maxLength={1000} defaultValue={p?.notes ?? ""}/></label>
        <p className="costs-disclaimer costs-span">Salvar cria uma nova versão e exige reautenticação. Snapshots já registrados mantêm os parâmetros históricos.</p>
        <button className="cost-button primary costs-span" disabled={!printerID || loading}><Save aria-hidden="true"/> {p ? "Salvar nova versão" : "Criar perfil"}</button>
      </form>
    </>}
    </section>
    <section className="costs-panel"><header className="costs-panel-heading"><div><span className="admin-kicker">Histórico imutável</span><h2>Versões anteriores</h2></div><span className="costs-count">{history.length} versões</span></header>{history.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Versão</th><th>Tarifa</th><th>Potência</th><th>Aquisição</th><th>Manutenção/h</th><th>Mão de obra/h</th><th>Registrada em</th></tr></thead><tbody>{history.map(item => <tr key={item.id}><td>v{item.version}{item.id === profile?.id ? " · vigente" : ""}</td><td>{formatCostMoney(item.electricity_rate_cents_per_kwh)}</td><td>{item.printer_power_milliwatts == null ? "Desconhecida" : `${item.printer_power_milliwatts.toLocaleString("pt-BR")} mW`}</td><td>{formatCostMoney(item.machine_acquisition_cents)}</td><td>{formatCostMoney(item.maintenance_rate_cents_per_hour)}</td><td>{formatCostMoney(item.labor_rate_cents_per_hour)}</td><td>{new Date(item.created_at).toLocaleString("pt-BR")}</td></tr>)}</tbody></table></div> : !loading && printerID && !error ? <p className="costs-muted">Ainda não há versões registradas.</p> : null}</section>
    <ReauthenticationDialog action={pending} onClose={() => setPending(null)} onSuccess={() => setReload(value => value + 1)} />
  </>;
}

function SnapshotCard({ snapshot }: { snapshot: CostSnapshot }) {
  const rows: [string, string][] = [["Material consumido",formatCostMoney(snapshot.material_consumed_cents)],["Desperdício",formatCostMoney(snapshot.material_waste_cents)],["Material total",formatCostMoney(snapshot.material_total_cents)],["Energia",formatCostMoney(snapshot.energy_cents)],["Depreciação",formatCostMoney(snapshot.depreciation_cents)],["Manutenção",formatCostMoney(snapshot.maintenance_cents)],["Preparação",formatCostMoney(snapshot.preparation_cents)],["Acabamento",formatCostMoney(snapshot.finishing_cents)],["Mão de obra total",formatCostMoney(snapshot.labor_total_cents)],["Custo total",formatCostMoney(snapshot.total_cost_cents)],["Custo unitário",formatCostMoney(snapshot.unit_cost_cents)],["Margem de contribuição",formatCostMoney(snapshot.contribution_margin_cents)],["Percentual de margem",formatCostPercent(snapshot.contribution_margin_basis_points)],["Lucro",formatCostMoney(snapshot.profit_cents)],["Margem de lucro",formatCostPercent(snapshot.profit_margin_basis_points)]];
  return <article className="cost-snapshot"><header><div><span className={`admin-badge ${snapshot.snapshot_type === "actual" ? "security-clean" : "security-scanning"}`}>{snapshot.snapshot_type === "actual" ? "Realizado" : "Estimado"}</span><h3>Custo {snapshot.snapshot_type === "actual" ? "realizado" : "estimado"}</h3></div><span>Perfil v{snapshot.cost_profile_version} · {snapshot.energy_source === "measured" ? "energia medida" : snapshot.energy_source === "configured_power" ? "potência configurada" : snapshot.energy_source}</span></header><dl className="cost-snapshot-grid">{rows.map(([label,value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl><p className="costs-muted">{snapshot.consumed_grams.toLocaleString("pt-BR")} g consumidos · {snapshot.wasted_grams.toLocaleString("pt-BR")} g de desperdício · {snapshot.run_seconds.toLocaleString("pt-BR")} s de execução · energia {snapshot.energy_milliwh.toLocaleString("pt-BR")} milliWh</p>{Array.isArray(snapshot.material_breakdown) && snapshot.material_breakdown.length > 0 && <details><summary>Composição e snapshots de material</summary><pre>{JSON.stringify(snapshot.material_breakdown,null,2)}</pre></details>}<small>Snapshot imutável. As cifras e a configuração exibidas foram retornadas pela API.</small></article>;
}

export function CostExecutionManager({ runs }: { runs: ProductionHistoryItem[] }) {
  const [runID,setRunID]=useState(runs[0]?.id??""); const [snapshots,setSnapshots]=useState<CostSnapshot[]>([]); const [loading,setLoading]=useState(runs.length>0); const [error,setError]=useState(""); const [message,setMessage]=useState(""); const [pending,setPending]=useState<SensitiveAction|null>(null); const [reload,setReload]=useState(0);
  const run=runs.find(item=>item.id===runID);
  useEffect(()=>{if(!runID)return;let active=true;getRunCostSnapshots(runID).then(value=>{if(active)setSnapshots(value)}).catch(cause=>{if(active)setError(costErrorMessage(cause))}).finally(()=>{if(active)setLoading(false)});return()=>{active=false}},[runID,reload]);
  function generate(kind:"estimate"|"actual",event?:FormEvent<HTMLFormElement>){event?.preventDefault();if(!run)return;let input:{energy_milliwh?:number;preparation_seconds?:number;finishing_seconds?:number}={};if(kind==="actual"&&event){const data=new FormData(event.currentTarget);const parse=(value:string,label:string)=>{const result=parseCostInteger(value);if(result==null)throw new Error(`${label}: informe um número inteiro válido.`);return result};try{const energy=String(data.get("energy_milliwh")??"").trim();input={...(energy?{energy_milliwh:parse(energy,"Energia medida")}:{}),preparation_seconds:parse(String(data.get("preparation_seconds")??""),"Preparação"),finishing_seconds:parse(String(data.get("finishing_seconds")??""),"Acabamento")}}catch(cause){setError(costErrorMessage(cause));return}}
    const label=kind==="estimate"?"gerar e gravar um snapshot estimado imutável":"registrar e gravar o snapshot de custo realizado imutável";
    if(!window.confirm(`Esta ação irá ${label} para ${run.job_number} · ${run.product_name}. O registro não poderá ser editado ou removido. Continuar?`))return;
    setError("");setMessage("");setPending(()=>async (proof:string)=>{await createRunCostSnapshot(runID,kind,input,proof);setMessage(`Snapshot ${kind==="estimate"?"estimado":"realizado"} registrado.`)});
  }
  return <><CostPermissionNote/><section className="costs-panel"><header className="costs-panel-heading"><div><span className="admin-kicker">Custos por execução</span><h2>Selecionar execução real</h2><p>A lista vem do histórico de produção; nenhum registro comercial é criado ou alterado aqui.</p></div></header>{runs.length?<label className="costs-run-select">Execução<select value={runID} onChange={event=>{setLoading(true);setError("");setMessage("");setRunID(event.target.value)}}>{runs.map(item=><option key={item.id} value={item.id}>{item.job_number} · {item.product_name} · tentativa {item.attempt_number} · {item.status}</option>)}</select></label>:<div className="costs-empty"><h3>Nenhuma execução disponível</h3><p>O histórico ainda não retornou execuções para consulta.</p><Link href="/admin/producao/historico">Abrir histórico de produção</Link></div>}{run&&<div className="costs-run-summary"><span>{run.product_name}{run.variant_name?` · ${run.variant_name}`:""}</span><span>{run.printer_name||"Impressora não informada"}</span><span>{run.status} · tentativa {run.attempt_number}{run.is_reprint?" · reimpressão":""}</span><span>{run.consumed_grams.toLocaleString("pt-BR")} g registrados · {run.wasted_grams.toLocaleString("pt-BR")} g desperdício</span></div>}{error&&<p role="alert" className="admin-alert">{error}</p>}{message&&<p role="status" className="costs-success">{message}</p>}{loading&&<div className="costs-skeleton" role="status">Consultando snapshots…</div>}
      {run&&<div className="costs-create-grid"><article><h3>Estimativa</h3><p>Disponível somente quando a API reconhecer dados operacionais suficientes. Gerar grava um snapshot imutável com a configuração e os preços de material usados naquele momento.</p><button className="cost-button secondary" disabled={loading} onClick={()=>generate("estimate")}>Gerar snapshot estimado</button></article><form onSubmit={event=>generate("actual",event)}><h3>Custo realizado</h3><p>Informe os tempos observados. Energia medida é opcional quando a potência configurada puder ser usada pela API.</p><label>Energia medida (milliWh), se disponível<input name="energy_milliwh" type="number" min="0" step="1" placeholder="Usar potência configurada"/></label><label>Preparação real (segundos)<input name="preparation_seconds" type="number" min="0" step="1" required/></label><label>Acabamento real (segundos)<input name="finishing_seconds" type="number" min="0" step="1" required/></label><button className="cost-button primary" disabled={loading}>Registrar custo realizado</button></form></div>}
    </section><section className="costs-panel"><header className="costs-panel-heading"><div><span className="admin-kicker">Histórico da execução</span><h2>Snapshots registrados</h2></div><span className="costs-count">{snapshots.length} registros</span></header>{snapshots.map(snapshot=><SnapshotCard key={snapshot.id} snapshot={snapshot}/>)}{!loading&&run&&snapshots.length===0&&!error&&<div className="costs-empty"><h3>Sem snapshots de custo</h3><p>Esta execução ainda não possui estimativa ou custo realizado persistido.</p></div>}</section><ReauthenticationDialog action={pending} onClose={()=>setPending(null)} onSuccess={()=>{setLoading(true);setReload(value=>value+1)}}/></>;
}

export type CostReportData={products:import("@/services/production-api").ProductCostProfitability[];printers:import("@/services/production-api").PrinterCostProfitability[];comparison:import("@/services/production-api").CostEstimateComparison[];analytics:import("@/services/production-api").CostAnalytics};
function ReportTable({title,headers,rows,empty}:{title:string;headers:string[];rows:React.ReactNode[][];empty:string}){return <section className="costs-panel"><header className="costs-panel-heading"><div><span className="admin-kicker">Relatório da API</span><h2>{title}</h2></div><span className="costs-count">{rows.length} registros</span></header>{rows.length?<div className="admin-table-wrap"><table className="admin-table"><thead><tr>{headers.map(h=><th key={h}>{h}</th>)}</tr></thead><tbody>{rows.map((row,index)=><tr key={index}>{row.map((cell,i)=><td key={i}>{cell}</td>)}</tr>)}</tbody></table></div>:<div className="costs-empty"><h3>{empty}</h3><p>Não há dados reais para o período selecionado.</p></div>}</section>}
function MaterialCostChart({items}:{items:CostReportData["analytics"]["timeline"]}){const maximum=Math.max(0,...items.map(item=>item.material_cost_cents));return <section className="costs-panel"><header className="costs-panel-heading"><div><span className="admin-kicker">Visualização dos valores da API</span><h2>Custo de material por período</h2></div></header>{items.length?<div className="costs-chart" role="list" aria-label="Custo de material por período">{items.map(item=><div className="costs-chart-row" role="listitem" key={item.period}><div><strong>{item.period}</strong><span>{formatCostMoney(item.material_cost_cents)} · {item.consumed_grams.toLocaleString("pt-BR")} g</span></div><i aria-hidden="true"><b style={{width:maximum>0?`${item.material_cost_cents/maximum*100}%`:"0%"}}/></i></div>)}</div>:<div className="costs-empty"><h3>Sem consumo no período</h3><p>O analytics não retornou pontos temporais.</p></div>}<p className="costs-muted">Escala visual relativa entre pontos da API. As barras não representam custo total de fabricação.</p></section>}
export function CostReports({data,from,to,mode}:{data:CostReportData;from:string;to:string;mode:"dashboard"|"profitability"}){
  const reports=<>
    <ReportTable title="Rentabilidade por produto" headers={["Produto","Execuções","Unidades","Receita","Custo fabricação","Margem contribuição","Lucro","Margens"]} rows={data.products.map(item=>[<strong key={item.product_id}>{item.product_name}</strong>,item.runs,item.units,formatCostMoney(item.revenue_cents),formatCostMoney(item.manufacturing_cost_cents),formatCostMoney(item.contribution_margin_cents),formatCostMoney(item.profit_cents),<span key={`${item.product_id}-margins`}>{formatCostPercent(item.contribution_margin_basis_points)} contrib. · {formatCostPercent(item.profit_margin_basis_points)} lucro</span>])} empty="Nenhuma rentabilidade por produto"/>
    <ReportTable title="Custos por impressora" headers={["Impressora","Execuções","Receita","Custo fabricação","Margem contribuição","Lucro","Margens"]} rows={data.printers.map(item=>[<strong key={item.printer_id}>{item.printer_name}</strong>,item.runs,formatCostMoney(item.revenue_cents),formatCostMoney(item.manufacturing_cost_cents),formatCostMoney(item.contribution_margin_cents),formatCostMoney(item.profit_cents),<span key={`${item.printer_id}-margins`}>{formatCostPercent(item.contribution_margin_basis_points)} contrib. · {formatCostPercent(item.profit_margin_basis_points)} lucro</span>])} empty="Nenhuma rentabilidade por impressora"/>
    <ReportTable title="Estimado versus realizado" headers={["Execução","Estimativa","Realizado","Diferença","Variação"]} rows={data.comparison.map(item=>[<Link key={item.run_id} href={`/admin/producao/historico/${encodeURIComponent(item.run_id)}`}>{item.run_id}</Link>,formatCostMoney(item.estimated_cost_cents),formatCostMoney(item.actual_cost_cents),formatCostMoney(item.difference_cents),formatCostPercent(item.difference_basis_points)])} empty="Nenhuma comparação disponível"/>
  </>;
  return <><CostPermissionNote/><section className="costs-panel"><form method="get" className="costs-period-filter"><label>De<input type="date" name="from" defaultValue={from}/></label><label>Até<input type="date" name="to" defaultValue={to}/></label><button className="cost-button primary">Aplicar período</button></form><p className="costs-muted">Os relatórios mostram snapshots realizados retornados pela API. Valores apresentados não são recalculados no navegador.</p></section>
    {mode==="dashboard"&&<><section className="costs-report-cards"><article><span>Custo de material registrado</span><strong>{formatCostMoney(data.analytics.summary.material_cost_cents)}</strong><small>Indicador de material do analytics operacional; não representa custo total de fabricação.</small></article><article><span>Material consumido</span><strong>{data.analytics.summary.consumed_grams.toLocaleString("pt-BR")} g</strong><small>Consumo informado pela API para o filtro aplicado.</small></article><article><span>Desperdício registrado</span><strong>{data.analytics.summary.wasted_grams.toLocaleString("pt-BR")} g</strong><small>Métrica operacional, não custo monetário total de desperdício.</small></article><article><span>Execuções no analytics</span><strong>{data.analytics.summary.total_runs.toLocaleString("pt-BR")}</strong><small>{data.analytics.summary.completed_runs.toLocaleString("pt-BR")} concluídas · {data.analytics.summary.failed_runs.toLocaleString("pt-BR")} falhas</small></article></section><MaterialCostChart items={data.analytics.timeline}/><section className="costs-panel"><header className="costs-panel-heading"><div><span className="admin-kicker">Consumo real</span><h2>Detalhamento por período</h2></div></header>{data.analytics.timeline.length?<div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Período</th><th>Execuções</th><th>Concluídas</th><th>Falhas</th><th>Consumido</th><th>Desperdício</th><th>Custo material</th></tr></thead><tbody>{data.analytics.timeline.map(item=><tr key={item.period}><td>{item.period}</td><td>{item.runs}</td><td>{item.completed}</td><td>{item.failed}</td><td>{item.consumed_grams.toLocaleString("pt-BR")} g</td><td>{item.wasted_grams.toLocaleString("pt-BR")} g</td><td>{formatCostMoney(item.material_cost_cents)}</td></tr>)}</tbody></table></div>:<div className="costs-empty"><h3>Sem consumo no período</h3><p>O analytics não retornou pontos temporais.</p></div>}</section></>}{reports}</>;
}
