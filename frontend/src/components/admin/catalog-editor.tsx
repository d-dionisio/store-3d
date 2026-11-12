"use client";

import { ArrowDown, ArrowUp, Eye, Plus, Save, Send, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { MagazineReader } from "@/components/catalog/magazine-reader";
import { createAdminCatalog, replaceAdminCatalogProducts, setAdminCatalogPublication, updateAdminCatalog, type EditorialCatalog } from "@/services/editorial-api";
import type { Product } from "@/types/product";

const empty={slug:"",title:"",description:"",eyebrow:"Edição digital",editionLabel:"",coverImageId:null as string|null};
export function CatalogEditor({initial,products}:{initial?:EditorialCatalog;products:Product[]}){
  const router=useRouter();
  const [catalog,setCatalog]=useState(initial);
  const [form,setForm]=useState(initial?{slug:initial.slug,title:initial.title,description:initial.description,eyebrow:initial.eyebrow,editionLabel:initial.editionLabel,coverImageId:initial.coverImageId}:empty);
  const [selected,setSelected]=useState<string[]>(initial?.productIds??[]);
  const [candidate,setCandidate]=useState("");
  const [message,setMessage]=useState("");const [busy,setBusy]=useState(false);const [preview,setPreview]=useState(false);
  const chosen=useMemo(()=>selected.map(id=>products.find(item=>item.id===id)).filter((item):item is Product=>Boolean(item)),[selected,products]);
  const coverOptions=chosen.flatMap(product=>product.images.map(image=>({id:image.id,label:`${product.name} — ${image.altText||"imagem"}`})));
  const move=(index:number,offset:number)=>{const target=index+offset;if(target<0||target>=selected.length)return;const next=[...selected];[next[index],next[target]]=[next[target],next[index]];setSelected(next)};
  const save=async()=>{setBusy(true);setMessage("");try{const input={slug:form.slug,title:form.title,description:form.description,eyebrow:form.eyebrow,edition_label:form.editionLabel,cover_image_id:form.coverImageId};let value=catalog?await updateAdminCatalog(catalog.id,{...input,expected_version:catalog.version}):await createAdminCatalog(input);value=await replaceAdminCatalogProducts(value.id,selected,value.version);setCatalog(value);setMessage("Catálogo salvo com sucesso.");if(!initial)router.replace(`/admin/catalogos/${value.id}`);router.refresh()}catch(error){setMessage(error instanceof Error?error.message:"Não foi possível salvar.")}finally{setBusy(false)}};
  const publication=async()=>{if(!catalog)return;setBusy(true);setMessage("");try{const value=await setAdminCatalogPublication(catalog.id,catalog.status!=="published",catalog.version);setCatalog(value);setMessage(value.status==="published"?"Catálogo publicado.":"Catálogo despublicado.");router.refresh()}catch(error){setMessage(error instanceof Error?error.message:"Não foi possível alterar a publicação.")}finally{setBusy(false)}};
  return <div className="admin-catalog-editor">
    <section className="admin-panel admin-catalog-fields"><h2>Metadados editoriais</h2><div className="admin-form-grid">
      <label>Título<input value={form.title} onChange={e=>setForm({...form,title:e.target.value})} maxLength={160}/></label>
      <label>Slug<input value={form.slug} onChange={e=>setForm({...form,slug:e.target.value.toLowerCase().replace(/[^a-z0-9-]/g,"-")})} maxLength={120}/></label>
      <label>Chamada<input value={form.eyebrow} onChange={e=>setForm({...form,eyebrow:e.target.value})} maxLength={80}/></label>
      <label>Rótulo da edição<input value={form.editionLabel} onChange={e=>setForm({...form,editionLabel:e.target.value})} maxLength={80}/></label>
      <label className="admin-full">Descrição<textarea value={form.description} onChange={e=>setForm({...form,description:e.target.value})} rows={4} maxLength={2000}/></label>
      <label className="admin-full">Imagem de capa<select value={form.coverImageId??""} onChange={e=>setForm({...form,coverImageId:e.target.value||null})}><option value="">Usar primeira imagem editorial</option>{coverOptions.map(option=><option key={option.id} value={option.id}>{option.label}</option>)}</select></label>
    </div></section>
    <section className="admin-panel"><div className="admin-panel-heading"><div><span className="admin-kicker">Ordem da revista</span><h2>Produtos</h2></div><div className="admin-add-product"><select value={candidate} onChange={e=>setCandidate(e.target.value)}><option value="">Selecione um produto</option>{products.filter(product=>!selected.includes(product.id)).map(product=><option key={product.id} value={product.id}>{product.name}</option>)}</select><button type="button" onClick={()=>{if(candidate){setSelected([...selected,candidate]);setCandidate("")}}} disabled={!candidate}><Plus size={16}/>Adicionar</button></div></div>
      {chosen.length?<ol className="admin-product-order">{chosen.map((product,index)=><li key={product.id}><span>{String(index+1).padStart(2,"0")}</span><div><strong>{product.name}</strong><small>{product.category.name}</small></div><button onClick={()=>move(index,-1)} disabled={index===0} aria-label={`Mover ${product.name} para cima`}><ArrowUp/></button><button onClick={()=>move(index,1)} disabled={index===chosen.length-1} aria-label={`Mover ${product.name} para baixo`}><ArrowDown/></button><button onClick={()=>setSelected(selected.filter(id=>id!==product.id))} aria-label={`Remover ${product.name}`}><Trash2/></button></li>)}</ol>:<div className="admin-empty compact"><p>Adicione produtos ativos antes de publicar.</p></div>}
    </section>
    <div className="admin-editor-actions"><button className="admin-secondary" type="button" onClick={()=>setPreview(!preview)} disabled={!chosen.length}><Eye size={17}/>Prévia</button><button type="button" onClick={save} disabled={busy}><Save size={17}/>{busy?"Salvando…":"Salvar"}</button>{catalog&&<button type="button" className={catalog.status==="published"?"admin-danger":""} onClick={publication} disabled={busy}><Send size={17}/>{catalog.status==="published"?"Despublicar":"Publicar"}</button>}</div>
    {message&&<p className="admin-editor-feedback" role="status">{message}</p>}
    {preview&&chosen.length>0&&<section className="admin-catalog-preview"><button type="button" onClick={()=>setPreview(false)}>Fechar prévia</button><MagazineReader category={{slug:form.slug,name:form.title||"Sem título",description:form.description,eyebrow:form.eyebrow,editionLabel:form.editionLabel,coverImageUrl:coverOptions.find(item=>item.id===form.coverImageId)?.id?chosen.flatMap(p=>p.images).find(i=>i.id===form.coverImageId)?.url:""}} products={chosen}/></section>}
  </div>
}
