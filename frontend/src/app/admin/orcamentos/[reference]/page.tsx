import { AdminShell } from "@/components/admin/admin-shell";
import { ProposalManager } from "@/components/admin/proposal-manager";
import { adminCookie, adminGuard } from "@/lib/admin-auth";
import { getAdminQuote } from "@/services/admin-api";

export const dynamic = "force-dynamic";
const dimensions = (w:number|null,h:number|null,d:number|null) => w&&h&&d ? `${w} × ${h} × ${d} mm` : "Não informadas";
export default async function QuoteDetailPage({params}:{params:Promise<{reference:string}>}) {
  const {reference}=await params; let quote;
  try { quote=await getAdminQuote(reference,await adminCookie()); } catch(e) { adminGuard(e); }
  return <AdminShell><header className="admin-header"><span>Solicitação {quote.Reference}</span><h1>{quote.CustomerName}</h1><p>Registrada em {new Date(quote.CreatedAt).toLocaleString("pt-BR")}</p></header><div className="admin-detail-grid">
    <section className="admin-panel"><h2>Cliente e especificação</h2><dl><div><dt>E-mail</dt><dd>{quote.CustomerEmail}</dd></div><div><dt>Telefone</dt><dd>{quote.CustomerPhone||"Não informado"}</dd></div><div><dt>Material</dt><dd>{quote.MaterialName}</dd></div><div><dt>Cor</dt><dd>{quote.ColorName}</dd></div><div><dt>Quantidade</dt><dd>{quote.Quantity}</dd></div><div><dt>Dimensões</dt><dd>{dimensions(quote.WidthMM,quote.HeightMM,quote.DepthMM)}</dd></div><div><dt>Status</dt><dd>{quote.Status}</dd></div></dl><h3>Observações</h3><p>{quote.Notes||"Nenhuma observação."}</p></section>
    <section className="admin-panel"><h2>Arquivos anexados</h2>{quote.Files.map(file=><article className="admin-file" key={file.ID}><div><strong>{file.Filename}</strong><small>{(file.SizeBytes/1024/1024).toFixed(2)} MiB · {file.Extension.toUpperCase()}</small></div><span className={`admin-badge security-${file.SecurityStatus}`}>{file.SecurityStatus}</span>{file.SecurityStatus!=="clean"&&<p>Download bloqueado até conclusão do scanner de segurança.</p>}</article>)}</section>
    <ProposalManager quote={quote}/><section className="admin-panel admin-history"><h2>Histórico</h2>{quote.History.map((event,index)=><div key={`${event.Kind}-${index}`}><span/><p><strong>{event.Kind}: {event.Status}</strong><small>{new Date(event.At).toLocaleString("pt-BR")}</small></p></div>)}</section>
  </div></AdminShell>;
}
