import Link from "next/link";
import { Box, Camera, Mail } from "lucide-react";

export function Footer() {
  return (
    <footer className="footer">
      <div className="container footer-grid">
        <div><div className="logo light"><span>3D</span> LOJA</div><p>Design que ganha forma, camada por camada.</p></div>
        <div><h2>Descubra</h2><Link href="/produtos">Produtos</Link><Link href="/personalizados">Personalizados</Link><Link href="/sobre">Sobre</Link></div>
        <div><h2>Ajuda</h2><Link href="/acompanhar-pedido">Acompanhar pedido</Link><Link href="/carrinho">Carrinho</Link><Link href="/checkout">Checkout protótipo</Link></div>
        <div><h2>Contato</h2><span><Mail size={16} /> contato@loja3d.local</span><span><Camera size={16} /> @loja3d</span><span><Box size={16} /> Produção sob demanda</span></div>
      </div>
      <div className="container footer-bottom"><span>© 2026 Loja 3D. Protótipo demonstrativo.</span><span>Feito com precisão no Brasil.</span></div>
    </footer>
  );
}

