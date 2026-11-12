import type { Metadata } from "next";
import { CartView } from "@/components/cart/cart-api-view";

export const metadata: Metadata = {
  title: "Carrinho",
  description: "Revise sua seleção antes de finalizar a compra.",
};

export default function CartPage() {
  return (
    <main className="page">
      <header className="page-hero compact container">
        <span className="eyebrow dark">Seleção atual</span>
        <h1>Seu carrinho</h1>
        <p>Preços e disponibilidade são confirmados pela loja antes do checkout.</p>
      </header>
      <section className="container cart-section"><CartView /></section>
    </main>
  );
}

