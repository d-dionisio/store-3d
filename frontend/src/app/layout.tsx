import type { Metadata } from "next";
import { CartProvider } from "@/components/cart/cart-provider";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("http://localhost:3000"),
  title: { default: "Loja 3D — Design que ganha forma", template: "%s | Loja 3D" },
  description: "Objetos autorais e impressão 3D personalizada, produzidos sob demanda.",
  openGraph: { title: "Loja 3D", description: "Design que ganha forma, camada por camada.", type: "website", locale: "pt_BR" },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="pt-BR">
      <body><CartProvider>{children}</CartProvider></body>
    </html>
  );
}

