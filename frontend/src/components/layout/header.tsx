"use client";

import { Heart, Menu, Search, ShoppingBag, X } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { useCart } from "@/components/cart/cart-provider";

const links = [
  ["Início", "/"], ["Produtos", "/produtos"], ["Coleções", "/produtos?category=natal"], ["Personalizados", "/personalizados"], ["Sobre", "/sobre"],
];

export function Header() {
  const [open, setOpen] = useState(false);
  const { count } = useCart();
  return (
    <header className="site-header">
      <div className="container header-inner">
        <Link href="/" className="logo" aria-label="Loja 3D — início"><span>3D</span> LOJA</Link>
        <nav className="desktop-nav" aria-label="Navegação principal">
          {links.map(([label, href]) => <Link key={href} href={href}>{label}</Link>)}
        </nav>
        <div className="header-actions">
          <Link href="/produtos" aria-label="Buscar produtos"><Search size={19} /></Link>
          <button aria-label="Favoritos demonstrativos"><Heart size={19} /></button>
          <Link href="/carrinho" aria-label={`Carrinho com ${count} itens`} className="cart-link"><ShoppingBag size={19} />{count > 0 && <span>{count}</span>}</Link>
          <button className="menu-button" onClick={() => setOpen(!open)} aria-expanded={open} aria-controls="mobile-menu" aria-label="Abrir menu">{open ? <X /> : <Menu />}</button>
        </div>
      </div>
      <nav id="mobile-menu" className={`mobile-nav ${open ? "open" : ""}`} aria-label="Navegação mobile">
        {links.map(([label, href]) => <Link key={href} href={href} onClick={() => setOpen(false)}>{label}</Link>)}
      </nav>
    </header>
  );
}

