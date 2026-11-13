import type {Metadata} from "next";
import Link from "next/link";
import {cookies} from "next/headers";
import {ArrowUpRight,FileText,Package,ShieldCheck} from "lucide-react";
import {MagicLinkForm} from "@/components/account/magic-link-form";
import {CustomerApiError,profile} from "@/services/customer-api";
import styles from "./account-access.module.css";

export const dynamic="force-dynamic";
export const metadata:Metadata={title:"Minha conta",description:"Acesse seus orçamentos, pedidos e criações autorais Lumé."};

export default async function AccountPage(){
  const cookie=(await cookies()).toString();
  let customer:null|Awaited<ReturnType<typeof profile>>=null;
  try{customer=await profile(cookie)}catch(error){if(!(error instanceof CustomerApiError)||error.status!==401)throw error}
  if(customer)return <main className={styles.accountHub}><div className={styles.hubInner}><span className={styles.kicker}>Sua área Lumé</span><h1>Bem-vindo de volta,<br/><em>{customer.FullName.split(" ")[0]}.</em></h1><p className={styles.hubIntro}>Acompanhe suas criações, decisões e pedidos em um espaço reservado para você.</p><div className={styles.hubGrid}><Link href="/minha-conta/orcamentos"><FileText/><span><small>Propostas autorais</small><strong>Meus orçamentos</strong></span><ArrowUpRight/></Link><Link href="/minha-conta/pedidos"><Package/><span><small>Da criação à entrega</small><strong>Meus pedidos</strong></span><ArrowUpRight/></Link><Link href="/minha-conta/seguranca"><ShieldCheck/><span><small>Privacidade e acesso</small><strong>Segurança da conta</strong></span><ArrowUpRight/></Link></div><p className={styles.identity}>Sessão verificada para <strong>{customer.Email}</strong></p></div></main>;
  return <main className={styles.accessPage}><section className={styles.accessLayout} aria-labelledby="account-access-title"><div className={styles.formSide}><div className={styles.formInner}><span className={styles.kicker}>Área reservada</span><MagicLinkForm titleId="account-access-title"/><div className={styles.alternative}><span>Prefere outro acesso?</span><Link href="/login">Entrar com senha <ArrowUpRight size={15}/></Link></div><p className={styles.privacy}><ShieldCheck size={15}/>Seu acesso é temporário e protegido. Nunca informaremos se um e-mail está cadastrado.</p></div></div></section></main>
}
