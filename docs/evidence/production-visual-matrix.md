# Matriz de homologação visual — produção e custos

- Gerado em: 2026-11-19T01:27:01.983Z
- Playwright: 1.65.1; navegador: chromium 159.0.8154.6
- Origem: http://localhost:3000
- Autenticação: arquivo storageState local validado pelo global setup; senha/MFA/cookies não foram lidos ou publicados.
- Resultado geral: failed
- Casos: planejados 60; executados 60; avaliados integralmente 24; aprovados 22; reprovados 2; bloqueados 36; não executados 0.
- Combinações previstas: 12 rotas estáticas × 4, 2 rotas dinâmicas × 4 e 4 verificações de navegação (60 testes).

## Inventário de rotas

- Estáticas: `/admin/producao`, `/admin/producao/planejamento`, `/admin/producao/otimizacao`, `/admin/producao/impressoras`, `/admin/producao/materiais`, `/admin/producao/compatibilidade`, `/admin/producao/historico`, `/admin/producao/desempenho`, `/admin/producao/custos`, `/admin/producao/custos/impressoras`, `/admin/producao/custos/execucoes`, `/admin/producao/custos/rentabilidade`.
- Dinâmicas: `/admin/producao/jobs/[id]` (link real da fila) e `/admin/producao/historico/[runID]` (link real do histórico). Nenhum identificador foi inventado.

| Página | Desktop 1440×1200 | Tablet 820×1180 | Mobile 390×844 | Compacto 320×720 |
|---|---|---|---|---|
| Produção | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-dashboard.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-dashboard.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-dashboard.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-dashboard.png) |
| Planejamento | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-planning.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-planning.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-planning.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-planning.png) |
| Otimização | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-optimization.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-optimization.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-optimization.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-optimization.png) |
| Impressoras | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-printers.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-printers.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-printers.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-printers.png) |
| Materiais | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-materials.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-materials.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-materials.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-materials.png) |
| Compatibilidade | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-compatibility.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-compatibility.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-compatibility.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-compatibility.png) |
| Histórico | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-history.png) | REPROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-history.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-history.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-history.png) |
| Desempenho | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-performance.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-performance.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-performance.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-performance.png) |
| Custos | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/costs-overview.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/costs-overview.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/costs-overview.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/costs-overview.png) |
| Perfis de custos | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/costs-printer-profiles.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/costs-printer-profiles.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/costs-printer-profiles.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/costs-printer-profiles.png) |
| Execuções de custos | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/costs-executions.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/costs-executions.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/costs-executions.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/costs-executions.png) |
| Rentabilidade | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/costs-profitability.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/costs-profitability.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/costs-profitability.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/costs-profitability.png) |
| Detalhe de job (ID real via fila) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-job-detail.png) | REPROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-job-detail.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-job-detail.png) | APROVADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-job-detail.png) |
| Detalhe de execução (ID real via histórico) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/desktop/production-run-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/tablet/production-run-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile/production-run-detail.png) | BLOQUEADO [captura](../../frontend/test-results/visual-homologation/etapa-10.10.2.4.4/screenshots/mobile-compact/production-run-detail.png) |

## Navegação

- Desktop 1440×1200: BLOQUEADO
- Tablet 820×1180: BLOQUEADO
- Mobile 390×844: BLOQUEADO
- Mobile compacto 320×720: BLOQUEADO

## Problemas e limitações

- BLOQUEADO: production-costs-history / Desktop 1440×1200; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: production-costs-history / Tablet 820×1180; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: production-costs-history / Mobile 390×844; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: production-costs-history / Mobile compacto 320×720; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: production-planning / Desktop 1440×1200; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-planning / Tablet 820×1180; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-planning / Mobile 390×844; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-planning / Mobile compacto 320×720; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-optimization / Desktop 1440×1200; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-optimization / Tablet 820×1180; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-optimization / Mobile 390×844; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-optimization / Mobile compacto 320×720; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-printers / Desktop 1440×1200; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-printers / Tablet 820×1180; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-printers / Mobile 390×844; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-printers / Mobile compacto 320×720; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-materials / Desktop 1440×1200; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-materials / Tablet 820×1180; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-materials / Mobile 390×844; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-materials / Mobile compacto 320×720; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-compatibility / Desktop 1440×1200; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-compatibility / Tablet 820×1180; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-compatibility / Mobile 390×844; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- BLOQUEADO: production-compatibility / Mobile compacto 320×720; erros=0; A interface exibiu o estado de indisponibilidade da API operacional..
- REPROVADO: production-history / Tablet 820×1180; erros=1; ver HTML report; mensagens e URLs sensíveis foram omitidas.
- BLOQUEADO: costs-overview / Desktop 1440×1200; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: costs-overview / Tablet 820×1180; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: costs-overview / Mobile 390×844; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: costs-overview / Mobile compacto 320×720; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: costs-profitability / Desktop 1440×1200; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: costs-profitability / Tablet 820×1180; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: costs-profitability / Mobile 390×844; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- BLOQUEADO: costs-profitability / Mobile compacto 320×720; erros=0; A interface exibiu o estado de indisponibilidade dos relatórios ou recursos..
- REPROVADO: production-job-detail / Tablet 820×1180; erros=1; ver HTML report; mensagens e URLs sensíveis foram omitidas.
- BLOQUEADO: production-run-detail / Desktop 1440×1200; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-run-detail / Tablet 820×1180; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-run-detail / Mobile 390×844; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..
- BLOQUEADO: production-run-detail / Mobile compacto 320×720; erros=0; Nenhum registro real disponível na lista; nenhum ID foi inventado e nenhum dado foi criado..

Capturas e relatório JSON ficam em `frontend/test-results/visual-homologation/etapa-10.10.2.4.4/` (artefatos locais ignorados pelo Git). Os documentos em `docs/` não são ignorados pelo `.gitignore` atual e aparecem no status do repositório; nenhum commit foi criado.
