# Homologação visual do painel administrativo — Etapa 10.10.2.4.2

## Resultado

**PENDENTE — inspeção visual autenticada não realizada.** Os serviços locais já estavam em execução, mas esta sessão não oferece automação de navegador e não havia sessão legítima do proprietário disponível para inspeção. MFA não foi contornado. Nenhuma captura foi produzida e nenhuma correção visual foi feita.

## Diagnóstico read-only do ambiente

Verificações realizadas em 2026-11-18:

| Serviço/porta | Evidência | Resultado |
|---|---|---|
| Frontend, `127.0.0.1:3000` | `netstat -ano` e PID associado via `Get-Process` | LISTENING, PID 14212, `node` (`C:\nvm4w\nodejs\node.exe`) |
| API, `127.0.0.1:8080` | `netstat -ano` e PID associado via `Get-Process` | LISTENING, PID 6152, executável `api.exe` em cache do Go |
| PostgreSQL, `127.0.0.1:5432` | `netstat -ano` | LISTENING, PID 13444 (`com.docker.backend`) |
| API health | GET `/api/v1/health`, proxy bypass | HTTP 200 |
| API readiness | GET `/api/v1/ready`, proxy bypass | HTTP 200 |
| Frontend route smoke check | GET `/admin/producao/custos`, proxy bypass | HTTP 200; isso não comprova acesso autenticado nem renderização visual correta |
| Docker Compose | `docker context show` e `docker compose ps` | contexto `desktop-linux`; consulta bloqueada por “Acesso negado” ao pipe `dockerDesktopLinuxEngine` |

Nenhum serviço foi iniciado, parado ou reiniciado. O estado `healthy` do container não pôde ser confirmado via Compose por causa da permissão do pipe; a porta local está ocupada e a readiness da API respondeu 200. Não foi feita conexão direta ao PostgreSQL.

O `compose.yaml` declara apenas o serviço `postgres`, com porta publicada em loopback (`127.0.0.1:${POSTGRES_PORT:-5432}:5432`), volume nomeado `postgres_data` e healthcheck `pg_isready`. Frontend e API são iniciados separadamente conforme README. Nenhuma variável, volume ou configuração foi alterada.

## Páginas encontradas no código

Inspeção estática de `frontend/src/app/admin/producao` encontrou:

- `/admin/producao`
- `/admin/producao/planejamento`
- `/admin/producao/otimizacao`
- `/admin/producao/impressoras`
- `/admin/producao/materiais`
- `/admin/producao/compatibilidade`
- `/admin/producao/historico`
- `/admin/producao/desempenho`
- `/admin/producao/historico/[runID]`
- `/admin/producao/custos`
- `/admin/producao/custos/impressoras`
- `/admin/producao/custos/execucoes`
- `/admin/producao/custos/rentabilidade`

## Matriz de homologação visual

| Páginas/fluxos | 1440×1200 | 820×1180 | 390×844 | 320×720 |
|---|---|---|---|---|
| Produção, planejamento e otimização | PENDENTE | PENDENTE | PENDENTE | PENDENTE |
| Impressoras, materiais e compatibilidade | PENDENTE | PENDENTE | PENDENTE | PENDENTE |
| Histórico, detalhe de execução e desempenho | PENDENTE | PENDENTE | PENDENTE | PENDENTE |
| Custos, perfis por impressora, execuções e rentabilidade | PENDENTE | PENDENTE | PENDENTE | PENDENTE |

Overflow, navegação, tabelas, gráficos, formulários, loading, mensagens HTTP, contraste, foco por teclado e acessibilidade não foram avaliados em navegador. O GET de uma rota não substitui essa inspeção.

## Capturas e correções

Não há capturas reais para anexar. O diretório `docs/evidence/production-visual/` não foi preenchido. Não foram realizadas mutações, não foram gerados snapshots de custos e não houve alterações no backend, banco, pedidos, pagamentos ou dados comerciais.

Nenhuma correção visual foi feita: não foi observado defeito visual em navegador e alterar CSS sem evidência renderizada extrapolaria a homologação baseada em observação.

## Como concluir a validação manual

1. O operador pode resolver o acesso ao Docker Desktop no próprio Windows, se precisar confirmar o estado do container; não é necessário reiniciar serviços.
2. Em seu navegador, abrir o frontend local e autenticar pelo fluxo normal com senha e MFA. Não compartilhar credenciais, códigos, cookies ou tokens.
3. Verificar as rotas listadas nas quatro resoluções, usando o emulador responsivo do navegador ou janelas nas dimensões solicitadas.
4. Registrar apenas comportamentos observados, sem executar ações que criem snapshots ou alterem dados comerciais.
5. Salvar capturas diretamente do navegador em `docs/evidence/production-visual/`, identificando rota e resolução. Não incluir dados pessoais ou segredos.
6. Compartilhar as capturas e os defeitos observados para que correções visuais restritas possam ser feitas e então revalidadas.

## Classificação desta etapa

- Processos e portas: **PARCIALMENTE VERIFICADOS** — listeners e processos de frontend/API identificados; consulta Compose bloqueada.
- Health/readiness HTTP local: **APROVADO** — ambos HTTP 200.
- Inspeção estática de rotas: **APROVADA**.
- Inspeção visual autenticada: **PENDENTE**.
- Resoluções e navegação em navegador: **NÃO EXECUTADAS**.
- Capturas reais: **NENHUMA**.
- Correções visuais: **NENHUMA**.
- Testes automatizados: **NÃO EXECUTADOS nesta etapa**, pois nenhum código foi alterado.
- Homologação da Etapa 10.10.2.4.2: **PENDENTE**.

## Complemento 10.10.2.4.3 — infraestrutura Playwright

Foi adicionada infraestrutura local Playwright com Chromium, helper para abrir `/admin/login` e aguardar autenticação manual MFA, persistência de estado via `storageState` no perfil local do usuário e teste read-only de `/admin/producao` em 1440×1200. O caminho em `docs/evidence/production-visual/playwright/desktop/production-dashboard.png` era a configuração inicial da captura; essa execução não a produziu. Sessão, relatórios, resultados e traces ficam em paths locais ignorados pelo Git. O `.gitignore` desta cópia não contém regra que ignore `docs/`, portanto documentos ali aparecem no status e não foram commitados.

Playwright Test 1.65.1, Next.js 16.4.5, TypeScript 5.9.3, Node 22.15.0 e Chromium revision 1255 foram encontrados instalados antes da implementação. Não foi aberta janela de navegador nem salvo estado autenticado nesta etapa; portanto nenhum teste E2E autenticado ou screenshot real foi produzido. A homologação visual e o fluxo real MFA continuam pendentes. Ver `docs/evidence/playwright-setup.md` para procedimento.

## Complemento — Etapa 10.10.2.4.4: Playwright visual autenticado

### Execução e segurança

Em 2026-11-19 (UTC), a suíte Playwright 1.65.1 executou Chromium 159.0.8154.6 contra o frontend local já ativo. O global setup validou a sessão administrativa guardada localmente acessando o dashboard; nenhuma renovação, senha, MFA, cookie ou conteúdo do storageState foi automatizado, lido ou impresso. Não foram iniciados/reiniciados serviços, acessado PostgreSQL, executadas mutações pela UI ou alterados dados comerciais.

Inventário baseado no código: 12 rotas estáticas e 2 dinâmicas. Foram planejadas 48 combinações estáticas, 8 dinâmicas e 4 sequências de navegação: 60 casos. As rotas dinâmicas usaram links reais; `/admin/producao/historico/[runID]` ficou bloqueada nas quatro dimensões porque a lista não tinha execução com link real. Não foram inventados IDs nem criados dados.

### Resultado real

| Resultado | Casos |
|---|---:|
| Executados (inclui bloqueados) | 60 |
| Avaliados integralmente | 24 |
| Aprovados | 22 |
| Reprovados | 2 |
| Bloqueados | 36 |
| Não executados | 0 |
| Total | 60 |

Foram geradas 56 capturas Chromium locais (todas as 48 combinações estáticas e as 8 dinâmicas, inclusive telas bloqueadas e reprovadas). As quatro verificações de navegação não são capturas de rota. O smoke test legado também passou e produziu uma captura adicional, `screenshots/desktop/production-dashboard-legacy-smoke.png`. Os artefatos estão em `frontend/test-results/visual-homologation/etapa-10.10.2.4.4/`; o relatório HTML fica em `frontend/playwright-report/visual-homologation-10.10.2.4.4/`. Ambos são locais/ignorados pelo Git. A matriz detalhada encontra-se em `docs/evidence/production-visual-matrix.md`.

Resultados por página: dashboard aprovado nas quatro dimensões; histórico aprovado em desktop, mobile e compacto, mas reprovado em tablet; desempenho, perfis de custos e execuções de custos aprovados nas quatro dimensões. O detalhe de job real passou em desktop, mobile e compacto, e reprovou em tablet. Planejamento, otimização, impressoras, materiais e compatibilidade ficaram bloqueados nas quatro dimensões porque a interface exibiu “Produção indisponível”. Custos gerais e rentabilidade ficaram bloqueados nas quatro dimensões por estado visível de relatório indisponível. As quatro navegações foram bloqueadas ao encontrar essa indisponibilidade. O detalhe de execução ficou bloqueado por ausência de registros reais.

### Defeitos e diagnósticos

Dois defeitos reproduzíveis de overflow horizontal global em 820×1180:

- Histórico: `scrollWidth=899`, viewport/client width `820`, zero containers com rolagem horizontal intencional e dois controles (`select`) fora da viewport.
- Detalhe de job: `scrollWidth=839`, viewport/client width `820`, sem controles fora da viewport. A tabela de atributos ultrapassa a largura disponível.

Não foram feitas correções CSS nesta etapa; as capturas tablet registram os defeitos. O detector viu estados reais de indisponibilidade da API operacional/relatórios. Nas páginas operacionais bloqueadas o browser registrou um erro de console por viewport e respostas observadas pelo browser em HTTP 200; isso não prova o status das chamadas servidor-a-servidor entre Next.js e Go. Nos casos de custos bloqueados não houve erro de console/requisição do browser e as respostas do browser foram HTTP 200, mas a própria página apresentou seu estado de relatório indisponível. URLs, headers, corpos e mensagens sensíveis não foram copiados para o JSON consolidado.

O resultado geral é **REPROVADO/PARCIAL**: a infraestrutura foi executada e produziu evidência para todos os casos de página, mas 2 casos falharam responsividade e 36 estão bloqueados, portanto nenhuma dessas páginas bloqueadas pode ser declarada homologada. Inspeção humana das capturas ainda é recomendada; capturas não substituem validação funcional completa.

## Complemento — Etapa 10.10.2.4.5: responsividade e investigação de bloqueios

### Defeitos corrigidos no código

As capturas da rodada anterior mostram que, em 820×1180, o formulário do histórico excedia a coluna de conteúdo e dois `select`s alcançavam a borda da viewport. A causa no CSS era o grid de três colunas aplicado em `max-width:1000px` aos oito controles do formulário, combinado com inputs/selects sem `width:100%`, `min-width:0` e `box-sizing:border-box`. A correção mantém todos os filtros, usa duas colunas com tracks mínimos zero até 1000px, limita os controles à célula e faz o botão ocupar a largura disponível; abaixo de 560px retorna a uma coluna.

No detalhe de job, a captura não mostra uma tabela excedendo o container: o overflow vem do resumo de seis indicadores (`.production-job-summary`), cujos três tracks `1fr` no breakpoint tablet e filhos sem `min-width:0` permitiam que referências longas aumentassem o grid. A correção usa `minmax(0,1fr)`, duas colunas em tablet, `overflow-wrap:anywhere` para valores e uma coluna em mobile. A tabela operacional continua dentro de `.admin-table-wrap { overflow:auto }`, preservando rolagem interna e colunas.

### Investigação das indisponibilidades

Inspeção estática confirmou que `production-api.ts` chama as rotas administrativas de planejamento, capacidade/material, otimização, impressoras, materiais, histórico, produtividade e custos; os caminhos correspondentes estão registrados em `backend/internal/http/router_admin.go`. `canRead` permite `admin`, `production_manager` e `production_operator`; endpoints financeiros usam `canManage` (`admin` e `production_manager`). O código do handler converte autenticação em 401, autorização em 403 e erros inesperados em 500.

Não foi possível atribuir os estados bloqueados anteriores a uma causa HTTP específica. O relatório de 60 casos registrou apenas o estado visual, erros do navegador e status da navegação HTML; não registrou status/corpo das chamadas Next.js→Go. Como as páginas fazem consultas server-side que acessam repositórios, consultar os endpoints nesta retomada poderia ler `loja_3d`; isso foi evitado conforme a proibição de acesso ao banco comercial. A checagem local não encontrou listeners em 3000/8080 e não foi permitido iniciar serviços. Portanto autenticação atual, status HTTP, banco configurado no processo Go e causas de infraestrutura/configuração permanecem **não determinados**, não presumidos como defeito de backend ou RBAC.

No dashboard/rentabilidade de custos, as páginas já diferenciam 403 como “Acesso não autorizado”; arrays vazios retornados com sucesso seguem para a renderização normal, enquanto falha de serviço/contrato é apresentada como indisponibilidade. Sem observar respostas atuais, não é possível afirmar qual ramo ocorreu na rodada anterior. A causa visível do bloqueio anterior continua pendente de telemetria HTTP sanitizada em ambiente comprovadamente isolado.

### Navegação e execução da matriz

O histórico já constrói o link de detalhe usando o `item.id` retornado pela API. A rota dinâmica foi bloqueada anteriormente porque não havia execução real na lista; não se inventou ID nem se criou fixture. A navegação existente só pode ser homologada quando houver execução real acessível.

As correções foram cobertas por lint, testes frontend, TypeScript e build; não houve navegador autenticado nem screenshots novos nesta retomada. A sessão anterior permaneceu preservada, mas não foi reutilizada porque a origem de dados não pôde ser comprovada como banco de teste. A matriz da etapa 10.10.2.4.5 fica pendente de execução após os serviços locais serem disponibilizados e sua API apontar a ambiente isolado ou após autorização para leitura operacional segura. Não foram iniciados/parados serviços, executadas mutações, chamadas PostgreSQL, SQL ou migrations.

### Regressão da etapa 10.10.2.4.5

- `npm run lint`: PASS.
- `npm test -- --run`: PASS, 12 arquivos e 37 testes. A primeira tentativa no sandbox falhou com `spawn EPERM`; repetida com permissão de processo, sem API/banco, passou.
- `npx tsc --noEmit`: PASS.
- `npm run build`: PASS; compilação, TypeScript e geração estática concluídas.
- Playwright MFA isolation e matriz E2E: NÃO EXECUTADOS; listeners 3000/8080 indisponíveis e inicialização de serviços proibida.
- `git diff --check`: pendente de execução ao fechamento documental.

Resultado da etapa: defeitos corrigidos no CSS, mas homologação visual e diagnóstico de disponibilidade permanecem **PARCIAIS/PENDENTES**. Nenhuma captura nova foi produzida; os 56 screenshots reais e resultados da rodada 10.10.2.4.4 permanecem intactos.

## Matriz visual automatizada

A matriz por rota/resolução, estado de cada caso e links para capturas locais está em [`production-visual-matrix.md`](production-visual-matrix.md). A regra `.gitignore` não foi alterada para `docs/`; nesta cópia os documentos aparecem como modificados/untracked no `git status` e não foram commitados. Os artefatos sob `frontend/test-results/` e `frontend/playwright-report/` estão ignorados.

## Complemento — Etapa 10.10.2.4.5.1: revalidação controlada

### Pré-verificação somente leitura

Em 2026-11-19, foram consultados os listeners locais e a rota de login sem iniciar ou alterar serviços:

| Serviço/checagem | Resultado |
|---|---|
| Frontend, porta 3000 | Sem listener |
| API, porta 8080 | Sem listener |
| PostgreSQL, porta 5432 | Sem listener local |
| `GET /admin/login` | Indisponível; conexão não alcançável |
| Consulta dos processos Windows | Bloqueada pelo sistema operacional (`Acesso negado`); processo/executável não identificado |
| Health/readiness | Rotas identificadas estaticamente: `GET /api/v1/health` e `GET /api/v1/ready`; não chamadas |
| Destino efetivo do banco da API | Não determinado. A configuração estática usa `DATABASE_URL`, mas sem processo/API em execução não se comprovou o valor efetivo |

Nenhuma URL completa, credencial ou segredo foi exibido. Não houve conexão PostgreSQL, SQL, chamada à API, inicialização de serviço ou alteração de variável. Como o banco efetivo não pôde ser comprovado, todas as verificações que poderiam carregar dados foram interrompidas.

### Revalidação visual

Playwright não foi executado: o frontend estava indisponível e a sessão previamente armazenada não pôde ser validada contra a origem/API desta rodada. Não houve login, renovação de sessão ou automação de MFA.

| Página | Resoluções | Resultado | Captura nova |
|---|---|---|---|
| `/admin/producao/historico` | 1440×1200, 820×1180, 390×844, 320×720 | NÃO EXECUTADO | Não |
| `/admin/producao/jobs/{id}` via navegação por ID real | 1440×1200, 820×1180, 390×844, 320×720 | NÃO EXECUTADO | Não |

Overflow global, filtros, tabela e scroll interno, resumo do job, sidebar, conteúdo, botões e navegação não foram medidos nesta rodada. As correções CSS da etapa anterior permanecem sem revalidação visual atual. Nenhuma captura anterior foi removida ou sobrescrita; foram produzidas **zero** capturas novas.

### Diagnóstico HTTP estático e status observado

Os endpoints a seguir foram mapeados na implementação frontend. Nenhuma chamada foi realizada nesta rodada; status HTTP, payload, redirecionamento e erro anterior não podem ser inferidos pelo código. Um estado visual de indisponibilidade não comprova 401, 403, 404, 409, 500 ou ausência legítima de dados.

| Página | Endpoint(s) mapeado(s) | Status nesta rodada | Causa/classificação |
|---|---|---|---|
| Planejamento | `GET /api/v1/admin/production/planning`, `/capacity/printers`, `/planning/materials` | NÃO CONSULTADO | Infraestrutura (sem listener nesta rodada); causa anterior: Não determinado |
| Otimização | `GET /api/v1/admin/production/optimization` | NÃO CONSULTADO | Infraestrutura (sem listener nesta rodada); causa anterior: Não determinado |
| Impressoras | `GET /api/v1/admin/production/printers`, `/materials`, `/printer-technical-specs`; por impressora `/printers/{id}/monitoring` e `/printers/{id}/connection` | NÃO CONSULTADO | Infraestrutura (sem listener nesta rodada); causa anterior: Não determinado |
| Materiais | `GET /api/v1/admin/production/spools`, `/materials`, `/printers`, `/spool-installations` | NÃO CONSULTADO | Infraestrutura (sem listener nesta rodada); causa anterior: Não determinado |
| Compatibilidade | `GET /api/v1/admin/production/printers`, `/materials`; simulação usa `POST /compatibility/simulate` | NÃO CONSULTADO | Infraestrutura (sem listener); endpoint de simulação omitido por inspeção read-only; causa anterior: Não determinado |
| Custos | `GET /api/v1/admin/production/analytics/costs`, `/analytics/cost-profitability`, `/analytics/cost-profitability/printers`, `/analytics/cost-estimate-comparison` | NÃO CONSULTADO | Infraestrutura (sem listener nesta rodada); causa anterior: Não determinado |
| Rentabilidade | `GET /api/v1/admin/production/analytics/cost-profitability`, `/analytics/cost-profitability/printers`, `/analytics/cost-estimate-comparison` | NÃO CONSULTADO | Infraestrutura (sem listener nesta rodada); causa anterior: Não determinado |
| Detalhes de execução/custos | `GET /api/v1/admin/production/history?page=1&limit=100`; para ID real `/runs/{runID}/costs` e `/history/{runID}` | NÃO CONSULTADO | Infraestrutura (sem listener); sem ID ou dados consultados; causa anterior: Não determinado |
| Histórico | `GET /api/v1/admin/production/history` e lista de impressoras | NÃO CONSULTADO | Infraestrutura (sem listener nesta rodada); causa anterior: Não determinado |
| Detalhe de job | `GET /api/v1/admin/production/jobs/{id}`, impressoras e bobinas | NÃO CONSULTADO | Infraestrutura (sem listener); sem ID real obtido; causa anterior: Não determinado |

O frontend resolve a API via `API_URL`/`NEXT_PUBLIC_API_URL` no servidor e `NEXT_PUBLIC_API_URL`/origem no cliente; o valor efetivo do processo não estava acessível. As rotas de saúde encontradas estaticamente são `/api/v1/health` (health) e `/api/v1/ready` (readiness com ping PostgreSQL); status e corpo não observados.

### Classificação desta retomada

- Portas locais: **verificado somente por leitura** — sem listeners em 3000/8080/5432.
- Identidade/função dos processos: **não determinada**, consulta bloqueada pelo Windows.
- Destino efetivo da API: **não determinado**; nenhuma chamada a endpoints de dados foi feita.
- Causa das indisponibilidades anteriores: **não determinada**. A falta de listener agora é bloqueio de infraestrutura para a revalidação, não prova da causa HTTP/visual anterior.
- Revalidação visual e screenshots: **não executadas**; zero evidências novas.
- Testes automatizados: **não executados**; nenhum código foi alterado.
- Correções: **nenhuma**.

### Pendências para retomada

1. Disponibilizar frontend/API pelo procedimento operacional do responsável, sem inicialização automatizada nesta etapa.
2. Tornar verificável, sem revelar credenciais, que a API aponta para ambiente de dados autorizado antes de abrir páginas que fazem consultas server-side.
3. Validar uma sessão administrativa legítima já existente; se expirada, o operador deve autenticar manualmente com MFA pelo fluxo normal.
4. Reexecutar as duas rotas nos quatro viewports, usando navegação/IDs reais e salvando capturas novas sem dados pessoais.
5. Coletar status HTTP e erros sanitizados de cada endpoint, sem POST ou mutações, para classificar as causas anteriores.

## Complemento — Etapa 10.10.2.4.5.11: overflow de materiais em tablet

### Reprodução e causa raiz

Em 2026-11-20, a sessão Playwright administrativa existente foi validada pelo global setup e a página `/admin/producao/materiais` foi aberta em Chromium a 820×1180. O teste inicial reproduziu o defeito: `window.innerWidth=820`, `documentElement.clientWidth=820`, `documentElement.scrollWidth=888` e `body.scrollWidth=888`.

A inspeção geométrica identificou o elemento responsável como o formulário de instalação de bobina (`form.production-inline-form`), dentro da seção “Bobinas instaladas”. A grade CSS `repeat(4, 1fr)` usava mínimos intrínsecos automáticos: tracks medidos em 129,375 / 97,375 / 239,375 / 55,375 px em uma área interna de 432 px. O input ultrapassava a borda direita em 1,9 px (terminava em x=821,9) e o botão ficava em x=832,3–887,7. O formulário pai terminava em x=751, portanto eram os tracks/controles da grade, não a sidebar, que causavam o overflow global. Não era um container de rolagem intencional.

### Correção e validação

Foi adicionada uma regra somente para tablet (641–1100 px): `production-inline-form` passa a duas colunas `minmax(0,1fr)`, filhos podem encolher e inputs/selects ocupam no máximo o próprio track. O breakpoint mobile existente (até 640 px, uma coluna) e o layout desktop não foram alterados; nenhuma regra global de clipping foi adicionada.

Medição após a correção em 820×1180: `innerWidth=820`, `clientWidth=820`, `documentElement.scrollWidth=820`, `body.scrollWidth=820`. Os tracks mediram aproximadamente 194,8 px; todos os controles ficaram entre x=335 e x=735.

| Viewport | Caso Playwright | Resultado |
|---|---|---|
| 1440×1200 | `static | production-materials | desktop` | APROVADO |
| 820×1180 | `static | production-materials | tablet` | APROVADO |
| 390×844 | `static | production-materials | mobile` | APROVADO |
| 320×720 | `static | production-materials | mobile-compact` | APROVADO |

O teste direcionado tablet passou (1/1); a matriz dos quatro viewports passou (4/4). As capturas reais desta matriz foram salvas sem sobrescrever execuções anteriores em `frontend/test-results/visual-homologation/runs/2026-11-20T19-00-16.174Z-60340d41-b983-4a55-a573-28e0d38ce73a/screenshots/` (`desktop`, `tablet`, `mobile`, `mobile-compact`, cada uma com `production-materials.png`). Os artefatos Playwright da execução permanecem no diretório exclusivo por execução.

### Regressão e limites

- `npm run lint`: PASS.
- `npm test -- --run`: PASS, 12 arquivos e 37 testes. A primeira tentativa dentro do sandbox falhou com `spawn EPERM`; a repetição autorizada passou.
- `npx tsc --noEmit`: PASS.
- `npm run build`: PASS.
- `git diff --check`: PASS; Git apenas emitiu avisos de conversão LF/CRLF para arquivos já modificados.

Nenhum serviço foi iniciado/parado, nenhum banco foi consultado, não houve SQL, migration ou mutação pela UI. Esta validação cobre apenas responsividade da página de materiais; não altera a classificação de outras rotas ou homologações pendentes.

## Complemento — Etapa 10.10.2.4.5.12: revalidação e matriz visual final

Em 2026-11-20, frontend, health e readiness responderam HTTP 200. A attestation de homologação PostgreSQL descrita como pré-condição pelo operador não foi consultada novamente nesta fase; nenhuma conexão direta ao banco foi feita. O global setup Playwright validou a sessão administrativa local pelo acesso ao dashboard; senha, MFA e conteúdo do storage state não foram lidos nem automatizados.

Foi executada a matriz consolidada com `production-pages.spec.ts`, `production-dashboard.spec.ts` e `navigation.spec.ts`, sem o spec de login/MFA. Resultado: **61 planejados, 61 executados, 53 aprovados, 0 reprovados, 8 bloqueados, 0 não executados**. Os 53 aprovados são 48 combinações estáticas (12 páginas × 4 viewports), 4 verificações de navegação e 1 teste autenticado do dashboard. Os 8 bloqueados correspondem aos detalhes de job e execução em quatro viewports cada: as listas não ofereceram link “Detalhes” real. Nenhum ID foi inventado e nenhum dado foi criado.

Todas as páginas estáticas passaram em desktop 1440×1200, tablet 820×1180, mobile 390×844 e compacto 320×720: produção/dashboard, planejamento, otimização, impressoras, materiais, compatibilidade, histórico, desempenho, custos, perfis, execuções e rentabilidade. O histórico passou nos quatro viewports; testes verificaram overflow horizontal global, controles fora da viewport, foco por teclado e erros de servidor observados pelo browser. Navegação entre produção, custos, perfis, execuções, rentabilidade, histórico e desempenho passou nas quatro dimensões. O teste dedicado confirmou o dashboard autenticado.

Foram produzidas **56 capturas reais** em Chromium: 48 estáticas e 8 telas dos estados dinâmicos bloqueados. Pasta de screenshots: `frontend/test-results/visual-homologation/runs/2026-11-20T19-19-05.054Z-e4fd8d7f-fe88-4f8b-a27a-26ab29d5c027/screenshots/`. Relatório JSON e matriz automatizada: `frontend/test-results/visual-homologation/runs/2026-11-20T19-19-03.753Z-b6bb9629-36d4-4b98-a15a-5febb5faee8f/`; relatório HTML: `frontend/playwright-report/visual-homologation-runs/2026-11-20T19-19-03.753Z-b6bb9629-36d4-4b98-a15a-5febb5faee8f/index.html`. A suíte não gerou traces novos porque não houve falhas; traces e relatórios anteriores foram preservados.

Não foram feitas alterações de código, CSS, autenticação, MFA, CSRF ou RBAC nesta etapa. Não houve migrations, SQL, acesso direto ao banco, submissão de mutações pela UI, alteração de dados, nem inicialização/parada de serviços. Resultado final da matriz estática: **APROVADA**. Homologação dinâmica das páginas de detalhe: **PENDENTE**, dependente de jobs e execuções reais navegáveis. A avaliação é automatizada de layout/navegação e não substitui revisão humana de conteúdo/estética.
