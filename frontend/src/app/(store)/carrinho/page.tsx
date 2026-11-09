import type { Metadata } from "next"; import { CartView } from "@/components/cart/cart-api-view";
export const metadata: Metadata = { title: "Carrinho", description: "Carrinho demonstrativo da Loja 3D." };
export default function CartPage() { return <main className="page"><header className="page-hero compact container"><span className="eyebrow dark">Seleção atual</span><h1>Seu carrinho</h1><p>Itens salvos localmente neste navegador para demonstração.</p></header><section className="container cart-section"><CartView /></section></main>; }

