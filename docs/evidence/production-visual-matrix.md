# Matriz de homologação visual — produção e custos

- Gerado em: 2026-11-20T13:23:05.537Z
- Playwright: 1.65.1; navegador: chromium 159.0.8154.6
- Origem: http://localhost:3000
- Autenticação: arquivo storageState local validado pelo global setup; senha/MFA/cookies não foram lidos ou publicados.
- Resultado geral: failed
- Casos: planejados 62; executados 62; avaliados integralmente 54; aprovados 52; reprovados 2; bloqueados 8; não executados 0.
- Combinações previstas: 12 rotas estáticas × 4, 2 rotas dinâmicas × 4 e 4 verificações de navegação (60 testes).

## Inventário de rotas

- Estática: `/admin/producao` — Produção.
- Estática: `/admin/producao/planejamento` — Planejamento.
- Estática: `/admin/producao/otimizacao` — Otimização.
- Estática: `/admin/producao/impressoras` — Impressoras.
- Estática: `/admin/producao/materiais` — Materiais.
- Estática: `/admin/producao/compatibilidade` — Compatibilidade.
- Estática: `/admin/producao/historico` — Histórico.
- Estática: `/admin/producao/desempenho` — Desempenho.
- Estática: `/admin/producao/custos` — Custos.
- Estática: `/admin/producao/custos/impressoras` — Perfis de custos.
- Estática: `/admin/producao/custos/execucoes` — Execuções de custos.
- Estática: `/admin/producao/custos/rentabilidade` — Rentabilidade.
- Dinâmica: `/admin/producao/jobs/[id]` — Detalhe de job (ID real via fila); origem: `/admin/producao`, usando link real existente.
- Dinâmica: `/admin/producao/historico/[id]` — Detalhe de execução (ID real via histórico); origem: `/admin/producao/historico`, usando link real existente.

| Página | Desktop 1440×1200 | Tablet 820×1180 | Mobile 390×844 | Compacto 320×720 |
|---|---|---|---|---|
| Produção | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-dashboard.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-dashboard.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-dashboard.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-dashboard.png) |
| Planejamento | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-planning.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-planning.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-planning.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-planning.png) |
| Otimização | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-optimization.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-optimization.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-optimization.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-optimization.png) |
| Impressoras | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-printers.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-printers.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-printers.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-printers.png) |
| Materiais | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-materials.png) | REPROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-materials.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-materials.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-materials.png) |
| Compatibilidade | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-compatibility.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-compatibility.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-compatibility.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-compatibility.png) |
| Histórico | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-history.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-history.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-history.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-history.png) |
| Desempenho | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-performance.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-performance.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-performance.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-performance.png) |
| Custos | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/costs-overview.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/costs-overview.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/costs-overview.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/costs-overview.png) |
| Perfis de custos | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/costs-printer-profiles.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/costs-printer-profiles.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/costs-printer-profiles.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/costs-printer-profiles.png) |
| Execuções de custos | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/costs-executions.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/costs-executions.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/costs-executions.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/costs-executions.png) |
| Rentabilidade | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/costs-profitability.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/costs-profitability.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/costs-profitability.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/costs-profitability.png) |
| Detalhe de job (ID real via fila) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-job-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-job-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-job-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-job-detail.png) |
| Detalhe de execução (ID real via histórico) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-run-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-run-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-run-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-run-detail.png) |

## Navegação

- Desktop 1440×1200: APROVADO
- Tablet 820×1180: APROVADO
- Mobile 390×844: APROVADO
- Mobile compacto 320×720: APROVADO

## Problemas e limitações

- REPROVADO: unknown / unknown; erros=1; ver HTML report; mensagens e URLs sensíveis foram omitidas.
- REPROVADO: production-materials / Tablet 820×1180; erros=1; ver HTML report; mensagens e URLs sensíveis foram omitidas.
- BLOQUEADO: production-job-detail / Desktop 1440×1200; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-job-detail / Tablet 820×1180; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-job-detail / Mobile 390×844; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-job-detail / Mobile compacto 320×720; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-run-detail / Desktop 1440×1200; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-run-detail / Tablet 820×1180; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-run-detail / Mobile 390×844; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-run-detail / Mobile compacto 320×720; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..

Capturas e relatório JSON ficam em `frontend/test-results/visual-homologation/etapa-10.10.2.4.4/` (artefatos locais ignorados pelo Git). Os documentos em `docs/` não são ignorados pelo `.gitignore` atual e aparecem no status do repositório; nenhum commit foi criado.

## Matriz final — Etapa 10.10.2.4.5.12 (2026-11-20)

Esta execução mais recente substitui os resultados antigos da matriz para fins de status atual; as capturas e relatórios históricos acima permanecem preservados.

| Página estática | Desktop 1440×1200 | Tablet 820×1180 | Mobile 390×844 | Compacto 320×720 |
|---|---|---|---|---|
| Produção/dashboard | APROVADO | APROVADO | APROVADO | APROVADO |
| Planejamento | APROVADO | APROVADO | APROVADO | APROVADO |
| Otimização | APROVADO | APROVADO | APROVADO | APROVADO |
| Impressoras | APROVADO | APROVADO | APROVADO | APROVADO |
| Materiais | APROVADO | APROVADO | APROVADO | APROVADO |
| Compatibilidade | APROVADO | APROVADO | APROVADO | APROVADO |
| Histórico | APROVADO | APROVADO | APROVADO | APROVADO |
| Desempenho | APROVADO | APROVADO | APROVADO | APROVADO |
| Custos | APROVADO | APROVADO | APROVADO | APROVADO |
| Perfis de custos | APROVADO | APROVADO | APROVADO | APROVADO |
| Execuções de custos | APROVADO | APROVADO | APROVADO | APROVADO |
| Rentabilidade | APROVADO | APROVADO | APROVADO | APROVADO |
| Detalhe de job (link real) | BLOQUEADO | BLOQUEADO | BLOQUEADO | BLOQUEADO |
| Detalhe de execução (link real) | BLOQUEADO | BLOQUEADO | BLOQUEADO | BLOQUEADO |

Resumo: 61 casos planejados e executados; 53 aprovados, 0 reprovados, 8 bloqueados e 0 não executados. Os bloqueios dinâmicos ocorreram porque não havia links reais “Detalhes” nas listas; nenhum identificador foi inventado e nenhum fixture foi criado. As quatro verificações de navegação e o teste dedicado de dashboard passaram. Capturas estáticas e das telas bloqueadas (56 no total) estão em `frontend/test-results/visual-homologation/runs/2026-11-20T19-19-05.054Z-e4fd8d7f-fe88-4f8b-a27a-26ab29d5c027/screenshots/`.

Artefatos desta execução: [relatório JSON e matriz gerada](../../frontend/test-results/visual-homologation/runs/2026-11-20T19-19-03.753Z-b6bb9629-36d4-4b98-a15a-5febb5faee8f/report.json) e [relatório HTML](../../frontend/playwright-report/visual-homologation-runs/2026-11-20T19-19-03.753Z-b6bb9629-36d4-4b98-a15a-5febb5faee8f/index.html). Nenhuma falha ocorreu; portanto esta rodada não criou traces novos. As capturas, traces e relatórios de execuções anteriores não foram removidos nem sobrescritos.
