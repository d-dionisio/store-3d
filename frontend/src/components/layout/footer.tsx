import Link from "next/link";

export function Footer() {
  return <footer className="footer editorial-footer">
    <div className="container footer-intro"><div className="footer-wordmark">LUMÉ</div><p>Objetos autorais que unem tecnologia, matéria e sensibilidade para transformar espaços.</p></div>
    <div className="container footer-grid">
      <div><h2>Studio</h2><Link href="/sobre">Sobre nós</Link><Link href="/personalizados">Peças personalizadas</Link></div>
      <div><h2>Descubra</h2><Link href="/produtos">Todas as peças</Link><Link href="/produtos?category=decoracao">Decoração</Link><Link href="/produtos?category=natal">Coleções</Link></div>
      <div><h2>Sua experiência</h2><Link href="/minha-conta">Minha conta</Link><Link href="/minha-conta/pedidos">Meus pedidos</Link><Link href="/acompanhar-pedido">Acompanhar pedido</Link></div>
    </div>
    <div className="container footer-bottom"><span>© 2026 Lumé Studio.</span><span>Design autoral · Produção sob demanda no Brasil</span></div>
  </footer>;
}
