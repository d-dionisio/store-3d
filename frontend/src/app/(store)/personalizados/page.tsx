import type { Metadata } from "next";
import { Box, FileUp, MessageSquareText, Sparkles } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { QuoteForm } from "@/components/custom-print/quote-request-form";
export const metadata: Metadata = { title: "Impressão personalizada", description: "Solicite um orçamento para impressão 3D sob medida." };
const steps: { icon: LucideIcon; number: string; title: string; description: string }[] = [{ icon: FileUp, number: "01", title: "Envie sua ideia", description: "Envie com segurança um modelo STL ou 3MF." }, { icon: MessageSquareText, number: "02", title: "Defina os detalhes", description: "Escolha material, cor, quantidade e descreva o uso." }, { icon: Box, number: "03", title: "Análise técnica", description: "Analisaremos o modelo antes de informar valor e prazo." }];
export default function CustomPage() { return <main className="dark-page"><section className="container dark-hero"><span className="tech-label"><Sparkles size={16} /> Laboratório de ideias</span><h1>Sua ideia merece<br /><em>ganhar forma.</em></h1><p>Uma experiência preparada para transformar modelos digitais em objetos físicos — com orientação de material, acabamento e produção.</p></section><section className="container process-grid">{steps.map(({ icon: Icon, number, title, description }) => <article key={number}><span>{number}</span><Icon /><h2>{title}</h2><p>{description}</p></article>)}</section><section className="container quote-wrap"><QuoteForm /></section></main>; }

