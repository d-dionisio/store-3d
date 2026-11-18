# Homologação do motor de custos — Fases 2, 3 e 4

Data: 2026-11-18. Esta evidência reúne concorrência PostgreSQL (Fase 2), controles HTTP (Fase 3) e regressão (Fase 4). Nenhuma migration foi executada na Fase 4, e nenhuma conexão foi aberta ao banco comercial `loja_3d`.

## Fase 2 — concorrência PostgreSQL

Pré-condições confirmadas pelo operador: PostgreSQL 17.6, banco isolado `loja_3d_test`, migration 28, `dirty=false`, URLs distintas e container healthy. O próprio teste repete a guarda de destino (aceita somente `loja_3d_test` em loopback:5432), consulta versão e migration antes de criar seu schema isolado.

Comando:

```text
go test ./internal/infrastructure/postgres -run '^TestProductionCosts(PostgreSQL|ConcurrencyPostgreSQL)$' -count=1 -v
```

Resultado: PASS (`ok .../internal/infrastructure/postgres 1.315s`). O teste concorrente registrou PostgreSQL 17.6 e backends independentes `pidA=6288` e `pidB=6289`. Antes de liberar cada lock, observou ambos os backends em `pg_stat_activity` esperando por lock.

- Perfil: foi preparada a versão 1; duas gravações concorrentes com `expected_version=1` disputaram o mesmo lock. Uma gravou a versão 2; a outra recebeu conflito. Não houve versão parcial.
- Snapshot: duas chamadas independentes disputaram lock da mesma execução e retornaram o mesmo ID; uma linha `actual` persistiu.
- Unicidade: inserção direta concorrente/duplicada foi rejeitada pela constraint única `(production_run_id,snapshot_type)` (`23505`).
- Rollback: snapshot com impressora incompatível foi rejeitado pelo trigger, sem linha parcial; snapshot bem-sucedido anterior permaneceu intacto.
- Imutabilidade: alteração de snapshot foi rejeitada pelo trigger append-only. Atualização obsoleta do perfil também conflitou sem linha extra.
- Limpeza: as fixtures foram criadas em schema temporário de nome exclusivo; `DROP SCHEMA ... CASCADE` removeu apenas esse schema, e o teste verificou `to_regnamespace` ausente. O teste preexistente `TestProductionCostsPostgreSQL` passou adicionalmente e confirmou suas próprias fixtures transacionais revertidas.

As tabelas temporárias foram clonadas de tabelas da migration 28 com constraints/índices e receberam os triggers/funções reais de validação e append-only. `LIKE INCLUDING ALL` não replica foreign keys; portanto, FK não foi re-homologada nesta corrida. A concorrência, locks, versão otimista, unicidade, triggers e rollback foram exercitados em PostgreSQL real.

## Fase 3 — HTTP/RBAC/CSRF

Foi adicionado `backend/internal/http/handlers/production_costs_security_test.go`. Comando:

```text
go test ./internal/http/handlers -run '^TestCost(EndpointsAnonymousAndRoleMatrix|MutationsFailClosedOnCSRFAndMissingReauthentication|EndpointAuditUsesSensitiveActionNames)$' -count=1 -v
```

Resultado: PASS. Cobertura automatizada com repositórios de sessão/custos injetados (stubs de teste; **não** constituem sessão administrativa real):

- Seis rotas GET (perfil, histórico, snapshots, rentabilidade por produto, por impressora e comparação): anônimo recebeu 401; `quote_viewer` e `production_operator` receberam 403; `production_manager` e `admin` receberam 200.
- PUT de perfil e POSTs de estimativa/custo realizado: CSRF ausente ou inválido recebeu 403 e não chamou repositório de mutação.
- POST de estimativa, POST de custo realizado e PUT de perfil: anônimo recebeu 401.
- Com CSRF correto mas serviço de identidade/reautenticação não injetado, as três mutações falharam fechadas com 503 antes de qualquer gravação.
- Leitura autorizada de rentabilidade chamou a auditoria com ação `production.cost_analytics.read:success`.

`TestCostConfigurationIsManagerOnlyAndVersioned` passou separadamente no pacote de aplicação, confirmando que `quote_viewer` e `production_operator` não podem salvar perfil e que `production_manager`/`admin` podem fazê-lo através da regra de aplicação. A mutação HTTP bem-sucedida não foi simulada como E2E.

Revisão estática do fluxo real: `AdminRepository.GetSession` só retorna sessão ativa não expirada cujo `authentication_level='mfa'`; cookie administrativo é HTTP-only, SameSite Strict e Secure conforme configuração. Handlers de PUT/POST verificam sessão+CSRF e consomem evidência de reautenticação antes de invocar serviço de custo. RBAC de custo em `application/production/costs.go` limita leitura e mutação a `admin`/`production_manager`; negar operador e quote viewer é o comportamento previsto.

## Pendências registradas na Fase 3

- Não havia sessão administrativa real com MFA disponível para esta rodada. Login real, MFA, cookie, sessão persistida, CSRF e reautenticação não foram exercitados ponta a ponta. Nenhuma credencial, TOTP, cookie ou token real foi solicitado/usado.
- Expiração e uso único de evidência de reautenticação não foram exercitados pelos novos testes HTTP. O handler falha fechado quando evidência/serviço não está disponível, mas isso não substitui teste com Identity Repository e sessão admin real.
- A auditoria bem-sucedida foi observada no teste HTTP usando stub. No código, `ProductionHandler.audit` ignora erros do repositório; portanto, entrega durável da auditoria não foi comprovada. Rejeições de CSRF/reautenticação retornam antes da chamada de auditoria do handler.
- Cookies HTTP-only/Secure/SameSite e a exigência `authentication_level='mfa'` foram revisados no código, não validados por browser/sessão real nesta Fase 3.
- Naquele registro intermediário, a Fase 4 ainda não havia sido executada; os resultados posteriores estão documentados abaixo.

## Fase 4 — regressão e integridade

### Revisão de segurança dos testes PostgreSQL

Antes da suíte, foram inspecionados os testes `*_integration_test.go` e usos de `DATABASE_URL`, `TEST_DATABASE_URL`, `DELETE` e `TRUNCATE`. A busca identificou testes antigos com limpeza ampla (incluindo outbox e editorial) e testes que limpam linhas por identificadores/valores de fixture. `DATABASE_URL` e `TEST_DATABASE_URL` estavam ausentes do processo PowerShell; não foram carregados `.env`, definidos URLs ou fornecidos DSNs aos testes. Assim, as integrações opt-in que requerem `TEST_DATABASE_URL` não conectaram e foram ignoradas pelo próprio código. Nenhum `go test` desta fase acessou PostgreSQL.

### Backend

- `go test ./...`: PASS, exit 0, com ambas as variáveis de banco ausentes. Uma primeira tentativa usou o cache Go padrão e recebeu `Acesso negado` em `AppData\\Local\\go-build`; a repetição com cache temporário no workspace passou para todos os pacotes. O cache temporário foi removido após a execução.
- `go vet ./...`: PASS, exit 0.
- `go test ./internal/domain/production ./internal/application/production ./internal/infrastructure/postgres -run 'Cost|ProductionCosts' -count=1`: PASS, exit 0. Os testes puramente de domínio/aplicação foram executados; os casos PostgreSQL no pacote de infraestrutura foram ignorados, pois `TEST_DATABASE_URL` não estava configurada.
- Cobertura de custos exercitada na regressão inclui cálculo financeiro, arredondamento half-up, energia, overflow/entradas inválidas, consumo e snapshot de material, custos negativos, comparação analítica, autorização/configuração e requisitos de tempos observados.

### Frontend

- `npm run lint`: PASS (ESLint sem warnings/erros).
- `npm test -- --run`: PASS, 11 arquivos e 32 testes. A primeira execução no sandbox falhou com `spawn EPERM` durante resolução de caminhos da configuração Vite/Vitest; a repetição autorizada fora do sandbox passou.
- `npx tsc --noEmit`: PASS, exit 0.
- `npm run build`: PASS fora do sandbox após a tentativa no sandbox compilar e falhar com `spawn EPERM` na etapa TypeScript. Build final Next.js 16.4.5 compilou e gerou as páginas administrativas de produção, histórico, desempenho, impressoras, materiais e catálogo editorial. O build alterou apenas referências geradas em `frontend/next-env.d.ts`; essa alteração foi revertida para o conteúdo anterior.
- Não foi feita revisão visual autenticada nem foram produzidas capturas nesta fase.

### Auditoria e observações de segurança

Nenhuma autenticação, MFA, cookie, sessão, CSRF ou RBAC foi alterada. Permanecem as constatações da revisão estática da Fase 3: métodos de auditoria nos handlers descartam erros de gravação; retornos precoces de CSRF/reautenticação não são auditados por esses handlers. Isso foi registrado como risco/pendência, sem correção nesta fase. Não se coletaram dados financeiros sensíveis e não houve inspeção de respostas de produção real.

### Integridade

Não foram executadas migrations nem consultas/escritas no banco comercial ou no PostgreSQL em geral. Nenhum dado de pedido, pagamento, produto, catálogo ou snapshot comercial foi alterado. Como as integrações PostgreSQL estavam opt-out sem `TEST_DATABASE_URL`, a suíte não criou fixtures persistentes. O cache local temporário de Go foi removido; nenhuma mudança de código funcional foi feita nesta fase, além desta documentação.

Conclusão documental: regressões Go e frontend aprovadas; concorrência e cálculos já validados em PostgreSQL real em rodadas anteriores. Testes PostgreSQL não foram repetidos na Fase 4 por exigência de manter URLs ausentes. A homologação HTTP com sessão/MFA real, reautenticação ponta a ponta e auditoria durável segue pendente. A classificação formal requisito a requisito está em `docs/evidence/manufacturing-costs-final-checklist.md`.

## Etapa 10.10.2.1 — hardening de auditoria e segurança administrativa

### Diagnóstico e implementação

A revisão dos handlers `production_costs.go` confirmou que o helper compartilhado `ProductionHandler.audit` ignorava erros de persistência. As mutações de perfil e snapshot eram confirmadas pelo repository antes da auditoria posterior do handler, impossibilitando rollback. Além disso, as saídas antecipadas de CSRF e reautenticação não geravam eventos. O serviço e repositório existentes (`AdminService.Audit` e `admin_audit_events`) foram reutilizados; nenhuma migration, autenticação ou política MFA foi alterada.

Agora `production_cost_profiles` e `production_run_cost_snapshots` mais o evento de sucesso são confirmados na mesma transação do repository. Falha ao inserir o evento retorna `ErrCostAuditUnavailable` e deixa o perfil/ponteiro/snapshot sem alteração. O snapshot idempotente também audita cada operação aceita sem duplicar a linha financeira. Para leituras sensíveis, o handler só escreve a resposta após auditoria; para recusas/falhas, persistência de auditoria é exigida antes de responder. Sink indisponível retorna 503 genérico. A ação de segurança é uma lista fechada; IDs são filtrados como UUID, IP é parseado e canonicalizado, e nenhum dado bruto de erro, senha, TOTP, cookie, token ou hash de evidência é copiado ao evento.

Eventos cobertos: leitura de perfil, histórico, lista de snapshots, rentabilidade e comparação; sucesso transacional de salvamento de perfil e snapshot; falhas e recusas RBAC das operações; sessão ausente/inválida; CSRF ausente/inválido; reautenticação ausente, indisponível, inválida/expirada/reutilizada (estas três últimas permanecem indistinguíveis pela resposta do serviço de identidade). Os handlers não duplicam evento de sucesso de mutação: ele é escrito pelo repository na transação.

### Testes desta rodada

- `go test ./internal/http/handlers ./internal/application/production ./internal/application/admin ./internal/infrastructure/postgres`: PASS. Não houve URL de banco no processo; integração PostgreSQL não conectou.
- Regressão de rota protegida após injeção do serviço de auditoria no fixture de router: PASS.
- `go test ./internal/http/handlers ./internal/application/production -count=1`: PASS.
- `go test ./internal/infrastructure/postgres -run '^TestProductionCosts(PostgreSQL|ConcurrencyPostgreSQL)$' -count=1 -v`: os dois testes foram explicitamente `SKIP` porque `TEST_DATABASE_URL` estava ausente. Portanto, os novos casos PostgreSQL de rollback provocado por falha de auditoria não foram executados nesta rodada.
- `go test ./...`: PASS, com `DATABASE_URL` e `TEST_DATABASE_URL` ausentes; integrações PostgreSQL opt-in foram ignoradas e nenhum banco foi acessado. Antes da aprovação, a revisão estática encontrou uma falha em `TestManufacturingCostRoutesAreRegisteredAndProtected` causada pelo fixture não fornecer auditoria; o fixture foi corrigido para usar serviço de auditoria de teste e a regressão foi repetida com sucesso.
- `go vet ./...`: PASS.
- Nenhum teste frontend foi executado: esta subetapa só alterou Go e documentação.

### Limites e classificação

Testes unitários/HTTP automatizados são evidência de contrato com dependências injetadas, não de login/MFA real. Os testes PostgreSQL com transação rollback e falha do sink estão implementados/atualizados, mas sem DSN de teste não há evidência PostgreSQL nesta rodada. `DATABASE_URL` e `TEST_DATABASE_URL` estavam ausentes; não foram carregados arquivos `.env`, executadas migrations, consultas ou escritas em banco. Não houve acesso ao banco `loja_3d` nem alteração de dados comerciais.

O helper best-effort compartilhado em `production.go` continua usado por endpoints gerais de produção fora do motor de custos; esta etapa não o ampliou para todo o módulo. Eventos de recusa podem ser volumosos sob abuso; não foi adicionado rate limiter específico nem retenção, portanto o ingresso externo/Cloudflare e políticas operacionais continuam relevantes. Sessão administrativa real com MFA, reautenticação ponta a ponta e visualização autenticada continuam pendentes. A Etapa 10.10.2.1 tem correções técnicas implementadas e regressão Go aprovada, porém não deve ser declarada integralmente homologada até executar os testes PostgreSQL isolados e, se exigido como homologação, uma sessão legítima sem contornar MFA.
