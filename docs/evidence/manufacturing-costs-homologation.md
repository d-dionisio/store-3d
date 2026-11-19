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

### Correção da falha induzida de auditoria — retomada posterior

Na execução manual do teste em `loja_3d_test` (migration 28, `dirty=false`), `TestProductionCostsPostgreSQL` falhou porque a atualização do perfil retornou `nil` mesmo com a falha pretendida. A análise local identificou que o trigger de teste filtrava `NEW.action LIKE 'production.cost.%'`, mas o evento de perfil emitido pelo repository é `production.cost_profile.save`; portanto, o filtro não correspondia e o trigger não chegava a executar. O caminho transacional de produção já inseria a auditoria antes do commit e propagava a falha do insert como `ErrCostAuditUnavailable`; o defeito estava na injeção da falha do teste, não na expectativa nem no tratamento transacional.

O filtro do trigger foi corrigido para corresponder às ações exatas de perfil e snapshots. Permaneceram as verificações de rollback (uma única versão/perfil, ponteiro ainda na versão anterior e contagem de eventos de sucesso inalterada), além da falha de auditoria de snapshot e idempotência. A execução inicial pelo processo de ferramenta não tinha as URLs e não conectou; a validação PostgreSQL foi posteriormente executada pelo operador no PowerShell configurado, conforme evidência abaixo. Testes locais de aplicação/handlers e `go vet ./...` também passaram nesta etapa.

**Resultado informado pelo operador no PowerShell configurado:** a guarda de URLs foi aprovada; `TestProductionCostsPostgreSQL` passou em `loja_3d_test` (0,64 s de teste; 1,086 s pacote). Isso comprova o insert de auditoria atômico, falha induzida no insert, rollback do perfil, preservação da versão anterior/contador de eventos, rollback de snapshot, idempotência e ausência de linhas parciais conforme as asserções do teste. `TestProductionCostsConcurrencyPostgreSQL` também passou em PostgreSQL 17.6 (0,68 s; 1,022 s pacote): dois backends independentes foram observados aguardando lock; exatamente uma atualização de versão venceu, e as duas chamadas concorrentes retornaram o mesmo snapshot com uma linha persistida. As verificações de stale version, FK inválida, chave única e histórico imutável passaram; o schema descartável foi removido pelo próprio teste. O operador iniciou `go vet ./...`; nenhum erro ou interrupção foi apresentado e a execução local anterior desta etapa também passou `go vet ./...`.

## Etapa 10.10.2.2 — homologação de reautenticação administrativa

### Diagnóstico e correção

Fluxo encontrado: `POST` de reautenticação do `AdminHandler` exige sessão administrativa e CSRF, valida senha e TOTP/código de recuperação pelo `identity.Service`, cria token aleatório opaco, persiste somente seu hash associado à sessão e retorna o token na resposta autenticada. O frontend o mantém apenas em memória para enviá-lo no header `X-Reauthentication-Token`; não usa localStorage ou sessionStorage. Operações sensíveis consomem a evidência antes de prosseguir. Sessões administrativas são emitidas com `authentication_level='mfa'`; o TTL padrão é 5 minutos e `AUTH_REAUTH_TTL` aceita valores positivos até 15 minutos.

Foi encontrada uma lacuna concreta no repositório: o consumo conferia token, sessão e expiração da evidência, mas não confirmava que a sessão não havia sido revogada/expirada e que o usuário continuava ativo. A emissão também confiava que os IDs recebidos já correspondiam a sessão e usuário. `IdentityRepository` agora faz validação vinculada e bloqueia identidade/sessão durante emissão e consumo (na ordem compatível com o fluxo de desativação administrativa). Para admin, exige `authentication_level='mfa'` e representação de prova TOTP/recovery na chamada de emissão; tanto admin quanto customer exigem sessão não revogada/não expirada e identidade ativa. A gravação do consumo ocorre na mesma transação. Falhas de persistência são propagadas internamente e a aplicação continua falhando fechado sem expor detalhes SQL.

Arquivos alterados/criados nesta etapa:

- `backend/internal/infrastructure/postgres/identity_repository.go`
- `backend/internal/application/identity/reauthentication_test.go`
- `backend/internal/infrastructure/postgres/identity_reauthentication_integration_test.go`
- `docs/manufacturing-costs.md`
- `docs/production-api.md`
- `docs/architecture.md`
- `docs/evidence/manufacturing-costs-homologation.md`

### Testes locais executados

- `go test ./internal/application/identity -run '^TestAdminReauthenticationRequiresMFAAndUsesConfiguredTTL$' -count=1 -v`: PASS. Confirma senha + TOTP aceitos na camada de aplicação, TTL configurado aplicado e admin sem fator MFA habilitado rejeitado sem gravar evidência. É teste de aplicação, não sessão/MFA real.
- `go test ./internal/infrastructure/postgres -run '^$' -count=1`: PASS; compila os testes PostgreSQL sem executar fixture/conexão.
- `go test ./internal/application/identity -count=1`: PASS; `go test ./internal/http/handlers -run '^TestCost(EndpointsAnonymousAndRoleMatrix|MutationsFailClosedOnCSRFAndMissingReauthentication|EndpointAuditUsesSensitiveActionNames)$' -count=1`: PASS. Isso cobre contratos HTTP automatizados com sessão/repositórios de teste, não autenticação real.
- `go vet ./...`: PASS.
- Uma tentativa inicial do teste unitário revelou defeito no próprio stub (método `SecurityEvent` herdado com nil); o stub foi completado e a repetição passou.

### Teste PostgreSQL real preparado, ainda pendente

Foi criado `TestAdminReauthenticationPostgreSQL`. O teste exige que ambos os URLs estejam presentes no próprio processo, valida que o URL de teste é loopback:5432 e `loja_3d_test`, que o de desenvolvimento nomeie `loja_3d`, que sejam distintos, e consulta apenas o banco de teste para confirmar PostgreSQL 17.6, migration 28 e `dirty=false`. Cria um schema exclusivo, clona somente as quatro tabelas de autenticação necessárias (`admin_users`, `admin_sessions`, `auth_reauthentication_evidence`, `auth_mfa_factors`), não executa migrations e remove/verifica o schema exato ao final.

Casos preparados no teste: consumo válido/replay; emissão sem prova rejeitada; evidência expirada; tentativa com outra sessão e outra identidade; emissão com sessão de outro usuário; emissão/consumo em sessão revogada; sessão expirada; duas conexões independentes observadas aguardando lock em `pg_stat_activity`, com exatamente um consumo aceito; trigger de falha no UPDATE, erro propagado e evidência permanecendo não consumida. A fixture de repositório carrega fator MFA sintético e contador monotônico para testar a persistência; isso não reproduz nem comprova um TOTP humano real. Esses cenários **não estão aprovados ainda**: a compilação passou, mas o teste não foi executado nesta sessão porque `DATABASE_URL` e `TEST_DATABASE_URL` estavam ausentes ao processo. Nenhuma conexão ou consulta PostgreSQL foi feita pelo agente nesta etapa; migrations e dados comerciais não foram tocados.

### Primeira execução pelo operador e ajuste da fixture

Na primeira execução manual autorizada, as guardas de URL passaram e o teste conectou ao banco de teste. Ele falhou durante a preparação da fixture `expired` com SQLSTATE `23514`, pois a tabela real exige `expires_at > created_at`; o teste tentava retroceder somente `expires_at`. Nenhum cenário de consumo/concorrência foi alcançado. O cleanup do schema descartável está registrado com `t.Cleanup` e verifica a remoção do namespace ao final; a saída apresentada não reportou erro de cleanup. A fixture foi corrigida para retroceder `created_at` e `expires_at` em conjunto, sem alterar constraint ou migration.

Na repetição manual pelo operador, as guardas passaram e `TestAdminReauthenticationPostgreSQL` **PASSOU** em PostgreSQL real: 0,59 s de teste (1,136 s do pacote). A saída confirmou: evidência válida aceita uma única vez; replay rejeitado; evidência expirada, sessão diferente/usuário diferente, sessão revogada e sessão expirada rejeitadas; dois consumidores por conexões PostgreSQL independentes com exatamente um consumo confirmado; falha induzida no UPDATE propagada e rollback mantendo a evidência não consumida. O próprio teste também valida PostgreSQL 17.6, migration 28 (`dirty=false`) antes de criar o schema temporário e remove/verifica o schema ao terminar. Nenhum outro teste PostgreSQL foi incluído nessa execução. Isso é homologação real do repositório/transações, não prova de uma sessão ou TOTP humano real.

Comando para o operador executar no PowerShell onde as URLs já estão configuradas (não compartilhe os valores):

```powershell
$dev = [uri]$env:DATABASE_URL
$test = [uri]$env:TEST_DATABASE_URL
if (-not $env:DATABASE_URL -or -not $env:TEST_DATABASE_URL -or
    $dev.AbsolutePath.TrimStart('/') -cne 'loja_3d' -or
    $test.AbsolutePath.TrimStart('/') -cne 'loja_3d_test' -or
    $env:DATABASE_URL -ceq $env:TEST_DATABASE_URL -or
    $dev.Host -notin @('localhost', '127.0.0.1', '::1') -or $dev.Port -ne 5432 -or
    $test.Host -notin @('localhost', '127.0.0.1', '::1') -or $test.Port -ne 5432) {
  throw 'Proteção de banco falhou; teste não executado.'
}
go test ./internal/infrastructure/postgres -run '^TestAdminReauthenticationPostgreSQL$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'Homologação PostgreSQL de reautenticação falhou.' }
```

O teste lê `schema_migrations` no banco de teste; não executa migrations e não abre conexão com `DATABASE_URL`.

### Classificação nesta retomada

- Diagnóstico de código e vínculo/hash/TTL: **APROVADO** por inspeção e teste unitário direcionado.
- MFA obrigatório e TTL configurável na aplicação: **APROVADO** no teste unitário, sem alegar E2E.
- Teste PostgreSQL para expiração, replay, vínculo, revogação/expiração de sessão, concorrência e falha de persistência: **APROVADO** pela execução manual acima em `loja_3d_test`, PostgreSQL 17.6, migration 28 limpa.
- Sessão administrativa legítima com MFA, CSRF por navegador e operação sensível ponta a ponta: **PENDENTE**; nenhuma sessão ou fator real foi fornecido, nenhum bypass foi criado e nenhum MFA foi simulado como evidência real.
- Regressão específica desta mudança: teste unitário e `go vet` passaram; não foi executada suíte geral com URLs nem testes de integração antigos potencialmente destrutivos.

## Etapa 10.10.2.3 — MFA e sessão administrativa real

### Fluxo e correção

`AdminHandler.identityLogin` chama `identity.Service.StartLogin`: senha correta retorna HTTP 202 com desafio, sem cookie/sessão. `CompleteIdentityMFA` valida TOTP ou consome recuperação e, somente após sucesso, grava sessão MFA e define cookie. `AdminService.Authenticate`/`AdminRepository.GetSession` rejeitam sessão ausente, expirada, revogada, usuário inativo ou nível diferente de `mfa`. Logout exige CSRF, revoga e registra `admin.logout`. Reautenticação sensível permanece no fluxo previamente homologado, sem novo sistema.

Falha concreta corrigida: qualquer erro de `GetFactor` era interpretado como ausência. Em admin, isso chamava `UpsertPendingFactor`; para cliente, podia criar sessão apenas por senha. Agora só `ErrUnauthorized` (fator realmente ausente) permite o ramo de setup/login opcional; falhas inesperadas viram indisponibilidade. A verificação durante MFA também distingue erro de infraestrutura de prova inválida. Registros de senha negada, desafio, TOTP negado, MFA concluído e logout usam eventos existentes e não incluem senha, código, token ou segredo. Evento de desafio é transacional com o desafio; sucesso MFA e sessão são atômicos.

Arquivos criados/modificados nesta etapa: `backend/internal/application/identity/service.go`, `backend/internal/application/identity/service_login_test.go`, `backend/internal/infrastructure/postgres/identity_repository.go`, novo `backend/internal/infrastructure/postgres/admin_mfa_flow_integration_test.go` e os documentos de custos/API/arquitetura/evidência. Nenhuma migration foi alterada.

### Testes automatizados executados localmente

- Testes focados de `internal/application/identity` e `internal/http/handlers` (login exige TOTP, lookup indisponível falha fechado, mapeamento de erro e CSRF/reautenticação): **PASS**.
- `go test ./internal/infrastructure/postgres -run '^$' -count=1`: **PASS**, compilação sem executar fixtures ou conexão.
- `go vet ./...`: **PASS**.

### PostgreSQL/HTTP real: aprovado

`TestAdminMFASessionPostgreSQL` valida internamente URLs distintos (`DATABASE_URL`→`loja_3d`, `TEST_DATABASE_URL`→`loja_3d_test`), loopback:5432, PostgreSQL 17.6 e migration 28 limpa. Cria schema efêmero e clona as tabelas necessárias; cobre senha inválida, ausência de sessão antes do MFA, TOTP inválido/válido, cookie HTTP-only/Secure/SameSite Strict/Path/MaxAge, sessão persistida e acesso autenticado, expiração, recuperação de uso único, CSRF e logout/revogação, auditoria e remoção do schema. Não executa migrations nem conecta ao URL comercial.

O operador executou o comando no PowerShell com as URLs validadas. `TestAdminMFASessionPostgreSQL` **PASSOU** (1,14 s; pacote 1,634 s), exclusivamente em `loja_3d_test`. O teste validou PostgreSQL 17.6, migration 28 (`dirty=false`), utilizou schema descartável e confirmou sua remoção. Não executou migrations nem conectou ao banco comercial. A saída não reportou resíduos ou erro de cleanup.

Comando reproduzível usado no PowerShell, dentro de `backend`:

```powershell
$dev = [uri]$env:DATABASE_URL
$test = [uri]$env:TEST_DATABASE_URL
if (-not $env:DATABASE_URL -or -not $env:TEST_DATABASE_URL -or
    $dev.AbsolutePath.TrimStart('/') -cne 'loja_3d' -or
    $test.AbsolutePath.TrimStart('/') -cne 'loja_3d_test' -or
    $env:DATABASE_URL -ceq $env:TEST_DATABASE_URL -or
    $dev.Host -notin @('localhost', '127.0.0.1', '::1') -or $dev.Port -ne 5432 -or
    $test.Host -notin @('localhost', '127.0.0.1', '::1') -or $test.Port -ne 5432) {
  throw 'Proteção de banco falhou; teste não executado.'
}
go test ./internal/infrastructure/postgres -run '^TestAdminMFASessionPostgreSQL$' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'Homologação PostgreSQL/HTTP MFA falhou.' }
```

### Conta real/navegador e classificação

Não havia navegador autenticado com MFA legítimo disponível ao agente. Nenhuma senha, TOTP, código de recuperação, cookie ou token real foi solicitado ou usado; fixtures sintéticas não comprovam a conta do operador. Manualmente, o operador deve entrar normalmente em `/admin/login`, confirmar acesso com seu TOTP, inspecionar apenas atributos do cookie sem copiar o valor, testar uma mutação sensível pelo fluxo normal com CSRF/reautenticação e sair; nunca compartilhar valores secretos. Revisão visual autenticada permanece pendente.

- Correção fail-closed e testes focados de aplicação/handlers: **APROVADO**.
- Compilação PostgreSQL e `go vet ./...`: **APROVADO**.
- PostgreSQL/HTTP real com fixture isolada: **APROVADO** pela execução informada acima.
- Sessão humana real, MFA no navegador e inspeção visual autenticada: **PENDENTE**.
- Nenhuma migration ou conexão/dado comercial foi tocado nesta sessão.

Naquele momento, não iniciar a Etapa 10.10.2.3 antes da evidência PostgreSQL. O teste de reautenticação foi depois executado e aprovado pelo operador; a etapa 10.10.2.3 está registrada a seguir.
