"use client";

export default function CatalogError({ reset }: { error: Error; reset: () => void }) {
  return <main className="page"><section className="container error-page"><p className="eyebrow">Catálogo indisponível</p><h1>Não foi possível abrir esta edição.</h1><p>A conexão com o acervo falhou. Tente novamente em instantes.</p><button className="button button-primary" type="button" onClick={reset}>Tentar novamente</button></section></main>;
}
