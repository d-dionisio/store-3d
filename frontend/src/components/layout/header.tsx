"use client";

import { Menu, Search, ShoppingBag, UserRound, X } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useCart } from "@/components/cart/cart-provider";

const links = [
  ["Início", "/"],
  ["Decoração", "/produtos?category=decoracao"],
  ["Catálogos", "/catalogo"],
  ["Personalizados", "/personalizados"],
  ["Sobre nós", "/sobre"],
];

export function Header() {
  const pathname = usePathname();
  const isHome = pathname === "/";
  const [open, setOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const panel = useRef<HTMLElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const { count, openCart } = useCart();

  useEffect(() => {
    const update = () => setScrolled(window.scrollY > 28);
    update(); window.addEventListener("scroll", update, { passive: true });
    return () => window.removeEventListener("scroll", update);
  }, []);
  useEffect(() => {
    if (!open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const focusable = panel.current?.querySelectorAll<HTMLElement>("a,button");
    focusable?.[0]?.focus();
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") { setOpen(false); trigger.current?.focus(); }
      if (event.key !== "Tab" || !focusable?.length) return;
      const first = focusable[0], last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", keyboard);
    return () => { document.body.style.overflow = previous; document.removeEventListener("keydown", keyboard); };
  }, [open]);

  const appearance = isHome && !scrolled && !open ? "header-transparent" : "header-solid";
  return <header className={`site-header ${appearance} ${scrolled ? "header-scrolled" : ""}`}>
    <div className="container header-inner">
      <Link href="/" className="logo lume-logo" aria-label="Lumé Studio — início">LUMÉ <small>STUDIO</small></Link>
      <nav className="desktop-nav" aria-label="Navegação principal">
        {links.map(([label, href]) => <Link key={href} href={href}>{label}</Link>)}
      </nav>
      <div className="header-actions">
        <Link href="/produtos" aria-label="Buscar produtos"><Search size={18} /></Link>
        <Link href="/minha-conta" aria-label="Minha conta"><UserRound size={18} /></Link>
        <button type="button" onClick={() => { setOpen(false); openCart(); }} aria-label={`Abrir carrinho com ${count} itens`} className="cart-link"><ShoppingBag size={18} />{count > 0 && <span>{count}</span>}</button>
        <button ref={trigger} className="menu-button" onClick={() => setOpen(!open)} aria-expanded={open} aria-controls="mobile-menu" aria-label={open ? "Fechar menu" : "Abrir menu"}>{open ? <X /> : <Menu />}</button>
      </div>
    </div>
    <nav ref={panel} id="mobile-menu" className={`mobile-nav ${open ? "open" : ""}`} aria-label="Navegação mobile" aria-hidden={!open}>
      <div className="mobile-nav-brand">LUMÉ <span>STUDIO</span></div>
      {links.map(([label, href], index) => <Link key={href} href={href} onClick={() => setOpen(false)}><small>0{index + 1}</small>{label}</Link>)}
      <Link href="/minha-conta" onClick={() => setOpen(false)}><small>06</small>Minha conta</Link>
    </nav>
  </header>;
}
