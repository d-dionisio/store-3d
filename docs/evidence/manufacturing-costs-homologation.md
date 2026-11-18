# Homologação do motor de custos — Fases 2 e 3

Data: 2026-11-18. Escopo desta atualização: concorrência PostgreSQL (Fase 2) e controles HTTP (Fase 3). Não foram executadas migrations nesta rodada, nem foi aberta conexão ao banco comercial `loja_3d`.

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

## O que permanece não homologado

- Não havia sessão administrativa real com MFA disponível para esta rodada. Login real, MFA, cookie, sessão persistida, CSRF e reautenticação não foram exercitados ponta a ponta. Nenhuma credencial, TOTP, cookie ou token real foi solicitado/usado.
- Expiração e uso único de evidência de reautenticação não foram exercitados pelos novos testes HTTP. O handler falha fechado quando evidência/serviço não está disponível, mas isso não substitui teste com Identity Repository e sessão admin real.
- A auditoria bem-sucedida foi observada no teste HTTP usando stub. No código, `ProductionHandler.audit` ignora erros do repositório; portanto, entrega durável da auditoria não foi comprovada. Rejeições de CSRF/reautenticação retornam antes da chamada de auditoria do handler.
- Cookies HTTP-only/Secure/SameSite e a exigência `authentication_level='mfa'` foram revisados no código, não validados por browser/sessão real nesta Fase 3.
- Esta rodada não avançou à Fase 4: suíte de regressão completa e demais verificações permanecem pendentes.

Conclusão: concorrência do motor de custos aprovada em PostgreSQL real. Os contratos HTTP/RBAC/CSRF passaram em testes automatizados unitários, mas homologação ponta a ponta de autenticação MFA, reautenticação, auditoria durável e sessão real permanece pendente.
