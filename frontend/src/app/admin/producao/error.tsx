"use client";
export default function ProductionError({reset}:{error:Error;reset:()=>void}){return <main className="admin-main"><div className="admin-empty"><h1>Produção indisponível</h1><p>Não foi possível consultar a API operacional. Nenhum dado local foi usado como substituto.</p><button onClick={reset}>Tentar novamente</button></div></main>}
