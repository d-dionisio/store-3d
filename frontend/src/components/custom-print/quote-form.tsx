"use client";

import { Check, FileBox, UploadCloud } from "lucide-react";
import { FormEvent, useState } from "react";

export function QuoteForm() {
  const [file, setFile] = useState(""); const [feedback, setFeedback] = useState(false);
  const submit = (event: FormEvent) => { event.preventDefault(); setFeedback(true); };
  return <form className="quote-form" onSubmit={submit}>
    <div className="form-heading"><FileBox /><div><h2>Conte sobre sua peça</h2><p>Este formulário é visual e não envia nem armazena dados.</p></div></div>
    <label className="file-drop"><UploadCloud /><strong>{file || "Selecione um arquivo STL ou 3MF"}</strong><span>Somente demonstração — nenhum upload será realizado.</span><input type="file" accept=".stl,.3mf" onChange={(e) => setFile(e.target.files?.[0]?.name ?? "")} /></label>
    <div className="form-grid"><label><span>Material</span><select><option>PLA — acabamento fosco</option><option>PETG — maior resistência</option><option>A definir com a equipe</option></select></label><label><span>Cor</span><select><option>Marfim</option><option>Grafite</option><option>Violeta</option><option>Outra opção</option></select></label><label><span>Quantidade</span><input type="number" min="1" defaultValue="1" /></label><label><span>Dimensão aproximada</span><input placeholder="Ex.: 20 × 12 × 8 cm" /></label></div>
    <label><span>Observações</span><textarea rows={5} placeholder="Conte como a peça será usada, acabamento desejado e outras informações úteis." /></label>
    <button className="button button-dark" type="submit">Simular solicitação de orçamento</button>
    {feedback && <div className="form-feedback" role="status"><Check /> Simulação concluída. Nenhuma informação foi enviada.</div>}
  </form>;
}

