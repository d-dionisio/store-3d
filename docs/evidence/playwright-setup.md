# Playwright — configuração e sessão MFA manual

## Ambiente identificado

- Node.js 22.15.0.
- Next.js 16.4.5.
- TypeScript 5.9.3.
- `@playwright/test` 1.65.1 já constava nas dependências locais antes desta etapa.
- Chromium revision 1255 já estava instalado no cache Playwright.
- Login: `/admin/login`.
- Dashboard de produção: `/admin/producao`.

Nenhum navegador foi instalado novamente. A infraestrutura não configura `webServer`; portanto não inicia nem reinicia frontend ou API.

## Autenticação manual

Com os serviços locais já executando, abra um terminal no diretório `frontend` e rode:

```powershell
npm run test:e2e:auth
```

O Chromium abre visivelmente em `/admin/login`. O operador digita senha e TOTP/código de recuperação diretamente na UI normal. O script não automatiza, lê, registra nem captura esses campos. Depois do MFA, navega ao dashboard e confirma o heading `Produção 3D` antes de chamar `browserContext.storageState()`. Então fecha o Chromium.

O arquivo padrão fica em `%LOCALAPPDATA%\Formo3D\playwright\admin.json`, fora do workspace e do OneDrive/repositório. Pode ser configurado com `PW_AUTH_STATE`; a variável deve apontar para arquivo local protegido. O estado contém cookie de sessão HTTP-only e é segredo operacional: não copie para issues, chats, drives compartilhados ou commits. No Windows, mantenha as permissões herdadas do perfil local restritas ao usuário. O script não imprime o caminho completo nem o conteúdo.

O cookie é capturado pelo storageState oficial do Playwright. O CSRF administrativo fica no `sessionStorage` normal da aplicação e não é exportado pelo storageState; o teste inicial só lê SSR/dashboard e não faz mutações. Nenhuma tentativa de transportar token CSRF para outro mecanismo foi criada.

Se a sessão expirar ou for revogada, o teste não renova a sessão por meios alternativos: retorne ao fluxo normal com MFA executando novamente `npm run test:e2e:auth`.

## Teste inicial

```powershell
npm run test:e2e
```

Executa somente o teste `production-dashboard.spec.ts`, no Chromium em 1440×1200, contra `PW_BASE_URL` ou `http://localhost:3000`. Se a sessão estiver ausente/expirada, o teste informa a necessidade de autenticação manual. Não usa stubs, intercepta API nem altera dados. Confirma rota, título, erros JavaScript e HTTP 5xx; o diagnóstico inclui apenas contagens e omite URLs, headers e corpos.

A captura de sucesso é escrita em `docs/evidence/production-visual/playwright/desktop/production-dashboard.png`. A execução falha em vez de sobrescrever uma captura existente; para outro destino, use `PW_SCREENSHOT_PATH`. Relatório HTML: `frontend/playwright-report/`. Resultados e traces de falha: `frontend/test-results/`.

```powershell
npm run test:e2e:headed
npm run test:e2e:report
```

## Segurança dos artefatos

`frontend/playwright/.auth/`, `frontend/playwright-report/` e `frontend/test-results/` são ignorados pelo Git. O estado padrão fica fora do repositório. O `.gitignore` desta cópia não contém regra que ignore `docs/`; documentos e matriz sob `docs/evidence/` aparecem no `git status` e não foram commitados. Nenhuma regra ampla foi adicionada para publicar ou ocultar evidências.

Screenshots podem revelar filas, métricas e dados administrativos. Traces podem incluir conteúdo de página e detalhes de rede, inclusive cookies/headers. Mantenha-os localmente, revise antes de compartilhar e remova-os após uso conforme a política interna. O helper de login não tira screenshots nem grava trace.

## Estado desta entrega

Infraestrutura e teste inicial implementados. Nenhum serviço foi iniciado, nenhuma sessão foi criada, nenhum teste autenticado foi executado e nenhuma screenshot real foi produzida nesta execução. `npx playwright test --list` encontrou um teste/arquivo. `node --check playwright/authenticate.mjs`, `npm run lint`, `npm test -- --run` (12 arquivos/37 testes), `npx tsc --noEmit`, `npm run build` e `git diff --check` passaram. O primeiro teste do Vitest incluiu indevidamente o spec Playwright; `vitest.config.mts` agora restringe sua descoberta a `src/**`, e a regressão passou após o ajuste. O `spawn EPERM` do runner dentro do sandbox foi resolvido executando as verificações locais após autorização do executor, sem iniciar serviços.

O E2E autenticado e a screenshot permanecem pendentes até o operador concluir MFA pelo helper. O HTML report só será gerado após uma execução do Playwright, no caminho `frontend/playwright-report/`; não há relatório real nesta entrega. A matriz completa de páginas e resoluções pertence à etapa seguinte e não foi iniciada.

## Etapa 10.10.2.4.4 — execução visual autenticada

A suíte completa agora é executada por `npm run test:e2e:visual` (ou `npm run test:e2e:visual:headed`) e gera relatório com `npm run test:e2e:report`. Não há `webServer` no Playwright: frontend e API precisam estar previamente ativos. O global setup abre Chromium headless apenas para validar o estado MFA existente no dashboard; se a sessão estiver expirada, falha e pede renovação manual por `npm run test:e2e:auth`, sem tentar login.

Na rodada real de 2026-11-19 UTC, Chromium 159.0.8154.6 / Playwright 1.65.1 executou 60 casos: 24 tiveram avaliação completa (22 aprovados e 2 reprovados por overflow tablet), 36 ficaram bloqueados e nenhum ficou não executado. Foram salvas 56 capturas nas quatro viewports para as combinações das páginas estáticas e dinâmicas. O smoke test legado também passou (1/1), com uma captura adicional. Os caminhos são `frontend/test-results/visual-homologation/etapa-10.10.2.4.4/` e `frontend/playwright-report/visual-homologation-10.10.2.4.4/`; não compartilhar traces/capturas fora do ambiente autorizado. O JSON não registra IDs dinâmicos, URLs completas, mensagens de erro, cookies ou dados de requisição.

As falhas conhecidas são overflow em `/admin/producao/historico` e `/admin/producao/jobs/[id]` na viewport 820×1180. Rotas de planejamento/otimização/recursos operacionais e alguns relatórios mostraram estados de indisponibilidade e permanecem bloqueados. A rota de detalhe de execução não tinha item real disponível. Consultar a matriz em `docs/evidence/production-visual-matrix.md` e o relatório em `docs/evidence/production-visual-homologation.md` antes de declarar homologação visual concluída.
