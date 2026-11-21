# Isolamento dos testes PostgreSQL — Formô 3D

## Ambientes distintos

| Uso | Variável | Destino | Persistência |
| --- | --- | --- | --- |
| Desenvolvimento | `DATABASE_URL` | Ambiente local de desenvolvimento, atualmente associado a `loja_3d`/5432 | Dados de trabalho; não descartável. |
| Homologação visual/operacional | Configuração explícita da API | `loja_3d_test`, porta 5433, usuário `formo3d_test` | Contém seeds e dados de homologação; não descartável. |
| Testes de integração novos | `TEST_DATABASE_URL` | `127.0.0.1:5433/loja_3d_integration_test`, usuário `formo3d_integration_test` | Database/role dedicados; descartável somente sob procedimento operacional separado. |

`loja_3d_test` **não** é um banco descartável e não pode ser utilizado como destino de fixtures dos testes novos. O helper em `backend/internal/testsupport/postgres` lê somente `TEST_DATABASE_URL`, rejeita qualquer destino diferente do banco/usuário/host/porta de integração e não faz fallback para `DATABASE_URL`. Após conectar, confirma `current_database()`, `current_user` e a versão major 17 do servidor. Ele não aplica migrations.

O comando `backend/cmd/integration-db` também usa apenas `TEST_DATABASE_URL`. `status` consulta a identidade e a versão de migrations sem alterá-las. `migrate` só aceita uma base sem `schema_migrations` e sem relações de usuário em schemas não sistêmicos; em seguida aplica os arquivos oficiais em `backend/migrations` e exige estado limpo. Nenhuma operação desse comando foi executada ao preparar esta infraestrutura.

## Fixtures e limpeza

- Preferir `BeginTx(t, ctx, pool)` e deixar o rollback registrado no cleanup do teste.
- Para cenários concorrentes que precisam de commits independentes, gerar UUIDs aleatórios com `NewFixtureID()` e registrar callbacks por ID exato usando `RegisterIDCleanup`. O callback deve usar SQL parametrizado e excluir apenas o recurso criado pelo teste.
- `search_path` isolado é uma proteção complementar, não suficiente por si só: migrations podem conter referências explícitas a `public`, funções, extensões ou conexões adicionais. Cada teste precisa ser revisado para confirmar todas as relações acessadas e recursos limpos.
- Esta etapa não migra os testes legados. Alguns ainda usam tabelas públicas, IDs/cleanup antigos ou exigem portas e versões anteriores. Não apontar esses testes para o novo destino até revisão e adaptação individuais.

## Preparação manual do banco novo

O provisionamento do role e database é uma operação administrativa separada e não foi feito nesta etapa. A configuração aponta para o cluster local na porta 5433, mas exige database e role novos; não reutiliza `loja_3d_test`, `formo3d_test` nem segredos comerciais. A separação implementada é ao nível de database e role PostgreSQL; não comprova isolamento de container/volume. O operador deve confirmar que o cluster em 5433 é o pretendido antes de criar o destino. Exemplo interativo, **não executado**:

```powershell
psql -h 127.0.0.1 -p 5433 -U <administrador-do-cluster> -d postgres -X
```

No prompt `psql`, criar somente o role e database novos:

```sql
CREATE ROLE formo3d_integration_test LOGIN;
\password formo3d_integration_test
CREATE DATABASE loja_3d_integration_test OWNER formo3d_integration_test TEMPLATE template0;
```

`\password` solicita o segredo sem gravá-lo no histórico. Depois, disponibilize `TEST_DATABASE_URL` por um secret store local aprovado e limitado ao processo dos testes; não defina `DATABASE_URL` como fallback. Formato esperado (senha ilustrativa, não utilizável):

```text
postgres://formo3d_integration_test:<segredo-local>@127.0.0.1:5433/loja_3d_integration_test?sslmode=disable
```

No PowerShell, a partir de `backend`, consultar estado e aplicar migrations somente depois de confirmar que a saída de `status` reporta o banco e usuário esperados, PostgreSQL 17.x e `migration_version=none`:

```powershell
go run ./cmd/integration-db status
# Somente numa base nova e descartável, após revisar a identidade acima:
go run ./cmd/integration-db migrate
go run ./cmd/integration-db status
```

`migrate` recusa versão já registrada ou relações existentes; não executa `down`, não cria o database e não lê `DATABASE_URL`. O status final informa a versão aplicada e `dirty=false`. Não executar o fluxo em `loja_3d`, `loja_3d_test`, bancos remotos ou qualquer instância/volume compartilhado.

## Execução e estado desta etapa

Para os testes unitários do helper, que não abrem PostgreSQL:

```powershell
cd backend
go test ./internal/testsupport/postgres ./cmd/integration-db -count=1
go vet ./internal/testsupport/postgres ./cmd/integration-db
```

`go test -p 1 ./...` reduz concorrência entre pacotes, mas não isola dados por si só. Não executar a suíte completa com `TEST_DATABASE_URL` até a revisão/migração dos testes legados. Nenhum teste PostgreSQL, provisionamento, migration ou seed foi executado nesta etapa.

## Piloto de migração do teste Email Outbox

`TestOutboxIntegration` foi migrado para `internal/testsupport/postgres`: o URL é validado antes da conexão, o helper confirma a identidade do banco e PostgreSQL 17.x, e um schema exclusivo com UUID é criado por execução. O teste aplica nesse schema o arquivo oficial `000025_create_transactional_email_outbox.up.sql` usando `search_path`; não clona tabelas de `public`. Cleanup remove somente o schema UUID criado pelo próprio teste. A falha induzida de `Claim` renomeia/restaura uma tabela desse schema isolado, sem apagar tabela pública ou reconstruir DDL parcial.

A migration 25 define `email_outbox`, `email_delivery_events` e `email_suppressions`; não contém triggers nem funções de aplicação. O SQL usa `gen_random_uuid()`, disponível no PostgreSQL exigido pelo teste. O teste não depende de pedidos, catálogo, seeds ou registros comerciais. O handler de webhook é exercitado in-process com assinatura fictícia; o teste não inicia worker nem chama Resend.

Validação sem PostgreSQL: `go test ./internal/testsupport/postgres ./internal/infrastructure/mail ./cmd/integration-db -count=1 -v` passou para testes unitários e compilação; `TestOutboxIntegration` foi `SKIP` porque `TEST_DATABASE_URL` foi removida do processo durante a execução. `go vet ./...` e `git diff --check` passaram. A homologação PostgreSQL permanece pendente, pois `loja_3d_integration_test` ainda não foi provisionado.

Após provisionamento manual do novo banco e aplicação aprovada das migrations oficiais pelo fluxo documentado, executar a integração somente por:

```powershell
cd backend
go test ./internal/infrastructure/mail -run '^TestOutboxIntegration$' -count=1 -v
```

O teste não deve ser apontado a `loja_3d_test`; o guard aceita somente `127.0.0.1:5433/loja_3d_integration_test` com o usuário dedicado.
