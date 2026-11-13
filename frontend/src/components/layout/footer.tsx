import Link from "next/link";
import styles from "./footer.module.css";

const navigation = [
  { title: "Explore", links: [
    { label: "Todas as peças", href: "/produtos" },
    { label: "Decoração", href: "/produtos?category=decoracao" },
    { label: "Coleções", href: "/produtos?category=natal" },
    { label: "Personalizados", href: "/personalizados" },
  ]},
  { title: "LUMÉ Studio", links: [
    { label: "Nossa história", href: "/sobre" },
    { label: "Minha conta", href: "/minha-conta" },
    { label: "Meus pedidos", href: "/minha-conta/pedidos" },
  ]},
  { title: "Atendimento", links: [
    { label: "Acompanhar pedido", href: "/acompanhar-pedido" },
  ]},
] as const;

export function Footer() {
  return <footer id="site-footer" className={styles.footer}>
    <div className={`container ${styles.inner}`}>
      <section className={styles.identity} aria-labelledby="footer-statement">
        <div className={styles.brand}>
          <Link href="/" className={styles.wordmark} aria-label="Lumé Studio — início">LUMÉ</Link>
          <p className={styles.signature}>STUDIO · OBJETOS AUTORAIS</p>
          <p className={styles.description}>Objetos autorais que unem design, matéria e sensibilidade para transformar espaços.</p>
        </div>
        <h2 id="footer-statement">A beleza está<br/><em>nos detalhes.</em></h2>
      </section>
      <section className={styles.navigation} aria-label="Navegação do rodapé">
        <p className={styles.navigationLabel}>Navegue pelo studio</p>
        <div className={styles.navigationGrid}>
          {navigation.map(group => <nav key={group.title} aria-label={group.title}>
            <h2>{group.title}</h2>
            <ul>{group.links.map(link => <li key={link.href}><Link href={link.href}>{link.label}</Link></li>)}</ul>
          </nav>)}
        </div>
      </section>
      <div className={styles.bottom}>
        <p>© 2026 LUMÉ Studio.</p>
        <p>Design autoral · Produção sob demanda no Brasil</p>
      </div>
    </div>
  </footer>;
}
