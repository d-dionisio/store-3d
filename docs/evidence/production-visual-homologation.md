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

## Matriz visual automatizada

A matriz por rota/resolução, estado de cada caso e links para capturas locais está em [`production-visual-matrix.md`](production-visual-matrix.md). A regra `.gitignore` não foi alterada para `docs/`; nesta cópia os documentos aparecem como modificados/untracked no `git status` e não foram commitados. Os artefatos sob `frontend/test-results/` e `frontend/playwright-report/` estão ignorados.
