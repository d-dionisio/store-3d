"use client";
import Link from "next/link";
import {FormEvent,useState} from "react";
import {useRouter,useSearchParams} from "next/navigation";
import {completeCustomerMFA,forgotPassword,passwordLogin,registerCustomer,resetPassword,verifyEmailToken} from "@/services/customer-api";

export function CustomerAuthForm({mode}:{mode:"register"|"login"|"forgot"|"reset"|"verify"}){
 const router=useRouter(),query=useSearchParams();const[busy,setBusy]=useState(false),[message,setMessage]=useState(""),[challenge,setChallenge]=useState("");
 async function submit(e:FormEvent<HTMLFormElement>){e.preventDefault();if(busy)return;setBusy(true);setMessage("");const f=new FormData(e.currentTarget);try{
  if(challenge){const result=await completeCustomerMFA(challenge,String(f.get("code")||""),String(f.get("recovery_code")||""));sessionStorage.setItem("loja3d_customer_csrf",result.csrf_token);router.push("/minha-conta");router.refresh();return}
  if(mode==="register"){const password=String(f.get("password"));if(password!==String(f.get("confirm_password")))throw new Error("As senhas não coincidem.");const result=await registerCustomer(String(f.get("full_name")),String(f.get("email")),password);setMessage(result.message)}
  if(mode==="login"){const result=await passwordLogin(String(f.get("email")),String(f.get("password")));if(result.action==="mfa_required"){setChallenge(result.challenge_token||"");setMessage("Informe o código do autenticador ou um código de recuperação.")}else{if(result.csrf_token)sessionStorage.setItem("loja3d_customer_csrf",result.csrf_token);router.push("/minha-conta");router.refresh()}}
  if(mode==="forgot"){const result=await forgotPassword(String(f.get("email")));setMessage(result.message)}
  if(mode==="reset"){const password=String(f.get("password"));if(password!==String(f.get("confirm_password")))throw new Error("As senhas não coincidem.");await resetPassword(query.get("token")||"",password);setMessage("Senha atualizada. Entre novamente.")}
  if(mode==="verify"){await verifyEmailToken(query.get("token")||"");setMessage("E-mail verificado. Você já pode entrar.")}
 }catch(error){setMessage(error instanceof Error?error.message:"Não foi possível concluir.")}finally{setBusy(false)}}
 const title={register:"Criar sua conta",login:"Acessar sua conta",forgot:"Recuperar acesso",reset:"Definir nova senha",verify:"Confirmar e-mail"}[mode];
 return <form className="auth-card" onSubmit={submit}><span className="eyebrow">Conta Lumé</span><h1>{title}</h1>{mode==="register"&&<label>Nome completo<input name="full_name" autoComplete="name" required maxLength={150}/></label>}{!challenge&&(mode==="register"||mode==="login"||mode==="forgot")&&<label>E-mail<input name="email" type="email" autoComplete="email" required/></label>}{!challenge&&(mode==="register"||mode==="login"||mode==="reset")&&<label>Senha<input name="password" type="password" autoComplete={mode==="login"?"current-password":"new-password"} minLength={12} maxLength={1024} required/></label>}{!challenge&&(mode==="register"||mode==="reset")&&<label>Confirmar senha<input name="confirm_password" type="password" autoComplete="new-password" minLength={12} maxLength={1024} required/></label>}{challenge&&<><label>Código TOTP<input name="code" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6}/></label><label>Código de recuperação<input name="recovery_code" autoComplete="off"/></label></>}<button disabled={busy}>{busy?"Processando…":"Continuar"}</button>{message&&<p role="status" className="auth-message">{message}</p>}<nav>{mode!=="login"&&<Link href="/login">Entrar</Link>}{mode==="login"&&<><Link href="/cadastro">Criar conta</Link><Link href="/esqueci-minha-senha">Esqueci a senha</Link></>}</nav></form>
}
