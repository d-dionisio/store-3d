import type { Metadata } from "next";
import { Cormorant_Garamond, Inter } from "next/font/google";
import { CartProvider } from "@/components/cart/cart-provider";
import "./globals.css";

const editorial = Cormorant_Garamond({ subsets: ["latin"], variable: "--font-editorial", display: "swap", weight: ["400", "500", "600"] });
const sans = Inter({ subsets: ["latin"], variable: "--font-sans", display: "swap" });

export const metadata: Metadata = {
  metadataBase: new URL("http://localhost:3000"),
  title: { default: "Lumé Studio — Objetos autorais", template: "%s | Lumé Studio" },
  description: "Objetos autorais que transformam espaços em expressões de personalidade.",
  openGraph: { title: "Lumé Studio", description: "O belo ganha forma.", type: "website", locale: "pt_BR" },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="pt-BR">
      <body className={`${editorial.variable} ${sans.variable}`}><CartProvider>{children}</CartProvider></body>
    </html>
  );
}

