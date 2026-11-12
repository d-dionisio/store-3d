import Link from "next/link";
import type { EditorialCatalog } from "@/services/editorial-api";

export function ShowroomNav({ catalogs, currentSlug }: { catalogs: EditorialCatalog[]; currentSlug?: string }) {
  return <nav className="showroom-nav" aria-label="Navegação do showroom"><div className="container showroom-nav-inner">
    <Link href="/catalogo" className="showroom-nav-mark">Showroom <span>digital</span></Link>
    <div>{catalogs.map((catalog) => <Link key={catalog.id} href={`/catalogo/${catalog.slug}`} aria-current={catalog.slug === currentSlug ? "page" : undefined}>{catalog.title}</Link>)}</div>
  </div></nav>;
}
