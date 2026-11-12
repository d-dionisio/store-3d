"use client";

export default function CatalogError({ reset }: { error: Error; reset: () => void }) {
  return <main className="showroom-page"><section className="container showroom-empty showroom-error"><span>Showroom indisponível</span><h1>Não foi possível abrir esta edição.</h1><p>A conexão com o acervo falhou. Tente novamente em instantes.</p><button type="button" onClick={reset}>Tentar novamente</button></section></main>;
}
