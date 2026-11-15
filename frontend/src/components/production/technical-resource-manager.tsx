"use client";

import { FormEvent, useState, useTransition } from "react";
import type { MaterialTechnicalSpec, Printer, PrinterTechnicalSpec, Spool, SpoolInstallation } from "@/services/production-api";
import { installSpool, removeInstalledSpool, saveMaterialTechnicalSpec, savePrinterOperationalConfig, savePrinterTechnicalSpec } from "@/services/production-api";
import { parseOptionalBoolean } from "./technical-state";

const value = (data: FormData, key: string) => String(data.get(key) ?? "").trim();
const optionalNumber = (data: FormData, key: string) => value(data, key) ? Number(value(data, key)) : null;
const optionalBool = (data: FormData, key: string) => parseOptionalBoolean(value(data, key));

export function PrinterTechnicalEditor({ printers, materials, specs }: { printers: Printer[]; materials: MaterialTechnicalSpec[]; specs: PrinterTechnicalSpec[] }) {
  const [message, setMessage] = useState("");
  const [pending, startTransition] = useTransition();
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const printer = value(data, "printer_id");
    const current = specs.find((spec) => spec.printer_id === printer);
    startTransition(async () => {
      setMessage("");
      try {
        await savePrinterTechnicalSpec(printer, {
          status: value(data, "status") as "draft" | "verified", technology: "fdm", version: current?.version ?? 0, expected_version: current?.version ?? 0,
          max_width_mm: optionalNumber(data, "max_width_mm"), max_depth_mm: optionalNumber(data, "max_depth_mm"), max_height_mm: optionalNumber(data, "max_height_mm"),
          max_hotend_temp_c: optionalNumber(data, "max_hotend_temp_c"), max_bed_temp_c: optionalNumber(data, "max_bed_temp_c"), enclosed_chamber: optionalBool(data, "enclosed_chamber"),
          technical_notes: value(data, "technical_notes"), source_reference: value(data, "source_reference"), supported_material_ids: data.getAll("materials").map(String),
          supported_nozzles: [{ diameter_microns: Number(value(data, "nozzle")), type: value(data, "nozzle_type") }], surfaces: value(data, "surface") ? [value(data, "surface")] : [], features: [],
        });
        await savePrinterOperationalConfig(printer, {
          status: "configured", installed_nozzle_microns: Number(value(data, "nozzle")), installed_nozzle_type: value(data, "nozzle_type"),
          installed_surface: value(data, "surface"), material_slot_count: Number(value(data, "slots")), expected_version: current?.operational_config_version ?? 0,
        });
        setMessage("Configuração técnica salva. Atualize a página para visualizar a nova versão.");
      } catch (error) { setMessage(error instanceof Error ? error.message : "Falha ao salvar."); }
    });
  };
  return <form className="production-section technical-form" onSubmit={submit}>
    <header><div><span className="admin-kicker">Especificação versionada</span><h2>Configuração técnica</h2></div></header>
    <label>Impressora<select name="printer_id" required><option value="">Selecione</option>{printers.map((printer) => <option key={printer.id} value={printer.id}>{printer.name}</option>)}</select></label>
    <label>Status<select name="status"><option value="draft">Rascunho</option><option value="verified">Verificada</option></select></label>
    <label>Fonte técnica<input name="source_reference" placeholder="Documento ou URL verificável" /></label>
    {["max_width_mm", "max_depth_mm", "max_height_mm", "max_hotend_temp_c", "max_bed_temp_c"].map((key) => <label key={key}>{key.replaceAll("_", " ")}<input type="number" min="1" name={key} /></label>)}
    <label>Bico instalado (µm)<input required type="number" min="1" name="nozzle" defaultValue="400" /></label><label>Tipo de bico<input required name="nozzle_type" /></label><label>Superfície<input name="surface" /></label><label>Slots<input required type="number" min="1" name="slots" defaultValue="1" /></label>
    <label>Ambiente fechado<select name="enclosed_chamber"><option value="">Não documentado</option><option value="yes">Sim</option><option value="no">Não</option></select></label>
    <label>Materiais suportados<select name="materials" multiple>{materials.map((material) => <option key={material.material_id} value={material.material_id}>{material.material_name}</option>)}</select></label>
    <label>Observações<input name="technical_notes" /></label><button disabled={pending}>{pending ? "Salvando…" : "Salvar nova versão"}</button>{message && <p role="status">{message}</p>}
  </form>;
}

export function MaterialTechnicalEditor({ materials }: { materials: MaterialTechnicalSpec[] }) {
  const [message, setMessage] = useState(""); const [pending, startTransition] = useTransition();
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); const data = new FormData(event.currentTarget); const id = value(data, "material_id"); const current = materials.find((material) => material.material_id === id);
    startTransition(async () => { try {
      await saveMaterialTechnicalSpec(id, {
        manufacturer: value(data, "manufacturer"), commercial_name: value(data, "commercial_name"), polymer_type: value(data, "polymer_type"),
        print_temp_min_c: optionalNumber(data, "print_min"), print_temp_max_c: optionalNumber(data, "print_max"), print_temp_recommended_c: optionalNumber(data, "print_rec"),
        bed_temp_min_c: optionalNumber(data, "bed_min"), bed_temp_max_c: optionalNumber(data, "bed_max"), bed_temp_recommended_c: optionalNumber(data, "bed_rec"), recommended_speed_mm_s: optionalNumber(data, "speed"),
        requires_enclosure: optionalBool(data, "enclosure"), requires_drying: optionalBool(data, "drying"), moisture_sensitivity: value(data, "moisture"), required_surface: value(data, "surface"), notes: value(data, "notes"),
        status: value(data, "status") as "draft" | "verified", source_reference: value(data, "source"), version: current?.version ?? 0, expected_version: current?.version ?? 0,
      }); setMessage("Especificação do material salva.");
    } catch (error) { setMessage(error instanceof Error ? error.message : "Falha ao salvar."); } });
  };
  return <form className="production-section technical-form" onSubmit={submit}><header><h2>Especificação do material</h2></header>
    <label>Material<select required name="material_id"><option value="">Selecione</option>{materials.map((material) => <option key={material.material_id} value={material.material_id}>{material.material_name} · {material.status}</option>)}</select></label>
    <label>Status<select name="status"><option value="draft">Rascunho</option><option value="verified">Verificada</option></select></label><label>Fonte<input name="source" /></label><label>Fabricante<input name="manufacturer" /></label><label>Nome comercial<input name="commercial_name" /></label><label>Polímero<input name="polymer_type" /></label>
    {[["print_min", "Hotend mín."], ["print_rec", "Hotend rec."], ["print_max", "Hotend máx."], ["bed_min", "Mesa mín."], ["bed_rec", "Mesa rec."], ["bed_max", "Mesa máx."], ["speed", "Velocidade mm/s"]].map(([key, label]) => <label key={key}>{label}<input type="number" min="0" name={key} /></label>)}
    <label>Ambiente fechado<select name="enclosure"><option value="">Não configurado</option><option value="yes">Sim</option><option value="no">Não</option></select></label><label>Secagem<select name="drying"><option value="">Não configurada</option><option value="yes">Sim</option><option value="no">Não</option></select></label>
    <label>Umidade<select name="moisture"><option value="">Não configurada</option><option value="low">Baixa</option><option value="medium">Média</option><option value="high">Alta</option></select></label><label>Superfície<input name="surface" /></label><label>Observações<input name="notes" /></label>
    <button disabled={pending}>{pending ? "Salvando…" : "Salvar especificação"}</button>{message && <p role="status">{message}</p>}
  </form>;
}

export function SpoolInstallationManager({ printers, spools, installations }: { printers: Printer[]; spools: Spool[]; installations: SpoolInstallation[] }) {
  const [items, setItems] = useState(installations); const [message, setMessage] = useState(""); const [pending, startTransition] = useTransition();
  const submit = (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); const data = new FormData(event.currentTarget); startTransition(async () => { try { const item = await installSpool(value(data, "printer"), value(data, "spool"), Number(value(data, "slot"))); setItems((current) => [item, ...current]); setMessage("Bobina instalada."); } catch (error) { setMessage(error instanceof Error ? error.message : "Falha na instalação."); } }); };
  const remove = (id: string) => startTransition(async () => { try { await removeInstalledSpool(id, "Troca operacional registrada"); setItems((current) => current.filter((item) => item.id !== id)); } catch (error) { setMessage(error instanceof Error ? error.message : "Falha na remoção."); } });
  return <section className="production-section"><header><h2>Bobinas instaladas</h2></header><form className="production-inline-form" onSubmit={submit}><select required name="printer"><option value="">Impressora</option>{printers.map((printer) => <option key={printer.id} value={printer.id}>{printer.name}</option>)}</select><select required name="spool"><option value="">Bobina</option>{spools.map((spool) => <option key={spool.id} value={spool.id}>{spool.code}</option>)}</select><input required type="number" min="1" name="slot" placeholder="Slot" /><button disabled={pending}>Instalar</button></form>
    {items.map((item) => <article className="production-capacity-row" key={item.id}><div><strong>{item.spool_code} · {item.material_name}</strong><small>{item.printer_name} · slot {item.slot_number}</small></div><button type="button" onClick={() => remove(item.id)}>Remover</button></article>)}{message && <p role="status">{message}</p>}
  </section>;
}
