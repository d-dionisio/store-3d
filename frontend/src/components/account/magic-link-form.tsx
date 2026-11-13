"use client";
import {FormEvent,useId,useState} from "react";
import {ArrowRight,Check,LoaderCircle} from "lucide-react";
import {requestMagicLink} from "@/services/customer-api";
import styles from "@/app/(store)/minha-conta/account-access.module.css";

type Feedback={kind:"success"|"error";message:string}|null;
export function MagicLinkForm({titleId}:{titleId?:string}){
  const[email,setEmail]=useState(""),[feedback,setFeedback]=useState<Feedback>(null),[busy,setBusy]=useState(false),inputId=useId();
  async function submit(event:FormEvent<HTMLFormElement>){event.preventDefault();if(busy)return;const input=event.currentTarget.elements.namedItem("email") as HTMLInputElement,normalized=email.trim();if(!normalized||!input.validity.valid){setFeedback({kind:"error",message:normalized?"Informe um endereço de e-mail válido.":"Informe seu e-mail para continuar."});input.focus();return}setBusy(true);setFeedback(null);try{const value=await requestMagicLink(normalized);setFeedback({kind:"success",message:value.message||"Se o endereço puder acessar a conta, enviaremos um link."})}catch{setFeedback({kind:"success",message:"Se o endereço puder acessar a conta, enviaremos um link."})}finally{setBusy(false)}}
  return <form onSubmit={submit} className={styles.accessForm} noValidate><h1 id={titleId}>Seu espaço,<br/><em>sua essência.</em></h1><p className={styles.subtitle}>Acompanhe seus orçamentos e cada detalhe das suas criações autorais.</p><label htmlFor={inputId}>Seu e-mail</label><div className={styles.inputWrap}><input id={inputId} name="email" type="email" inputMode="email" autoComplete="email" required disabled={busy} aria-invalid={feedback?.kind==="error"} aria-describedby={`${inputId}-support ${feedback?`${inputId}-feedback`:""}`} value={email} onChange={event=>{setEmail(event.target.value);if(feedback?.kind==="error")setFeedback(null)}} placeholder="voce@exemplo.com"/></div><button type="submit" disabled={busy}>{busy?<><LoaderCircle className={styles.spinner} aria-hidden/>Enviando link…</>:<>Receber link de acesso <ArrowRight aria-hidden/></>}</button><p id={`${inputId}-support`} className={styles.support}>Acesso seguro, sem necessidade de senha.</p><div className={styles.feedback} aria-live="polite">{feedback&&<p id={`${inputId}-feedback`} data-kind={feedback.kind}>{feedback.kind==="success"&&<Check aria-hidden/>}{feedback.message}</p>}</div></form>
}
