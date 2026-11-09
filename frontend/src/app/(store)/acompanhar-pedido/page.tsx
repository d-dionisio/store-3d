import type { Metadata } from "next"; import { OrderTracker } from "@/components/order/order-tracker";
export const metadata: Metadata = { title: "Acompanhar pedido", description: "Demonstração visual do acompanhamento de um pedido fictício." };
export default function TrackPage() { return <main className="page"><header className="page-hero compact container"><span className="eyebrow dark">Acompanhamento demonstrativo</span><h1>Onde está sua peça?</h1><p>Visualize como funcionará a jornada de produção e entrega.</p></header><section className="container tracking-wrap"><OrderTracker /></section></main>; }

