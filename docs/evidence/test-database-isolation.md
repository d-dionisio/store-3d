# Isolamento PostgreSQL para homologação — Formô 3D

## Estado da inspeção

Inspeção somente leitura em 2026-11-19. Nenhum container/volume foi criado ou iniciado; não houve conexão PostgreSQL, SQL, migration ou alteração de dados.

- `compose.yaml` existente define apenas o serviço `postgres`, publica `127.0.0.1:${POSTGRES_PORT:-5432}:5432` e monta o volume persistente `postgres_data`.
- O `.env` local está presente. Foram lidos somente campos de destino não secretos: `POSTGRES_DB=loja_3d`, porta `5432` e `DATABASE_URL` com host local/porta 5432/banco `loja_3d`. A senha e os demais valores não foram exibidos.
- O `.env` também contém `TEST_DATABASE_URL`; sem revelar credenciais, seu destino observado é host `localhost`, porta `5432`, banco `loja_3d_test`. Isso não comprova que o banco exista nem que seja uma instância Docker independente.
- O `.env.example` também usa `loja_3d` e 5432 como valores de desenvolvimento.
- `docker context show` retornou `desktop-linux`; chamadas de leitura a `docker ps`, `docker volume ls` e `docker inspect` falharam porque o pipe `dockerDesktopLinuxEngine` não estava disponível. Portanto não foi possível confirmar/descartar uma instância ou volume de teste já existente, nem inventariar volumes.
- O router de configuração da API exige `DATABASE_URL`; o pool conecta e faz ping no startup. Não há guard no backend que valide nome do banco, host ou porta. O startup não executa migrations.
- O comando de migrations é independente. `go run ./cmd/migrate version` consulta a versão; `up`/`down` escrevem schema e não fazem parte deste procedimento.

## Projeto de isolamento preparado

Arquivos preparados:

- [`compose.test.yaml`](../../compose.test.yaml): serviço `postgres-test`, imagem `postgres:17.6-alpine`, porta loopback 5433, volume com nome obrigatório exclusivo e segredo montado como Docker secret local. O projeto Compose recebe nome independente na linha de comando e não usa o `.env` comercial.
- O exemplo `.env.test.example` é esperado pelo `.gitignore`, mas não estava presente no workspace na inspeção de 2026-11-19; verificar/repor esse arquivo em alteração documental própria antes de orientar sua cópia.
- `.gitignore`: `.env.test` continua ignorado pela regra `.env.*`; somente `.env.test.example` foi liberado para versionamento.

O Compose usa `POSTGRES_DB=postgres` intencionalmente para **não criar `loja_3d_test` automaticamente**. `loja_3d_test` só deve ser criado numa operação manual posterior, explicitamente aprovada. O healthcheck aponta para `loja_3d_test`; a verificação conclusiva do banco exige consulta read-only e não é substituída pelo healthcheck.

O volume e container são parametrizados para nomes únicos. Antes de qualquer `up`, o operador deve confirmar que o nome escolhido para volume e container não existe; em caso de colisão, escolher outro nome. A imagem fixa a versão PostgreSQL 17.6. A primeira inicialização precisa obter essa imagem se não estiver no cache local (acesso ao registry externo, somente após aprovação).

### Validação estrutural do Compose (2026-11-19)

A busca textual por `postgres_data` não é um teste válido de compartilhamento: ela também encontra o identificador lógico `test_postgres_data`. A validação correta considera o modelo renderizado pelo Compose e compara o **nome físico** do volume, o tipo do mount e seus destinos; também verifica ausência de `external: true`, bind mounts, e identidade independente de projeto/container.

Foi executado somente `docker compose --env-file .env.test -f compose.test.yaml -p formo3d-test config` (renderização; não inicia serviços). O resultado confirmou:

- Projeto Compose: `formo3d-test`.
- Serviço/container: `postgres-test` / `formo3d-postgres-test`.
- Imagem: `postgres:17.6-alpine`.
- Porta: host `127.0.0.1:5433` para container `5432/tcp`.
- Mount de dados: volume nomeado físico `formo3d_postgres_test_data` em `/var/lib/postgresql/data`; não é `postgres_data`, não é externo e não é bind mount.
- O serviço possui apenas esse mount de dados; não há mount do volume comercial.
- Secret configurado como Docker secret de arquivo. O caminho não foi exibido; o arquivo configurado não foi encontrado durante esta inspeção, portanto o Compose renderiza, mas o provisionamento ainda não é operacionalmente possível.

O nome lógico `test_postgres_data` no YAML não implica reutilização do volume comercial: a resolução física renderizada é `formo3d_postgres_test_data`. O Compose comercial mantém o volume lógico `postgres_data`; a comparação entre nomes físicos confirma isolamento estrutural nesta configuração.

Pendência adicional: `POSTGRES_DB` permanece `postgres`, enquanto o healthcheck consulta `loja_3d_test`. Como o banco-alvo não é criado automaticamente, o healthcheck ficará sem sucesso até que a criação do banco seja autorizada e realizada em fase separada. Não houve conexão ao PostgreSQL para confirmar estado algum.

## Configuração local — somente após autorização explícita

Não copiar credenciais do `.env` comercial. Crie um usuário e uma senha novos para o ambiente descartável. Guarde o arquivo de senha fora do workspace/OneDrive, com ACL restrita ao usuário operacional. Exemplo de preparo em PowerShell; este bloco **não foi executado**:

```powershell
Copy-Item .env.test.example .env.test

$id = [guid]::NewGuid().ToString("N")
$secretDirectory = Join-Path $env:LOCALAPPDATA "Formo3D\secrets"
$secretPath = Join-Path $secretDirectory "postgres-test-$id.txt"
New-Item -ItemType Directory -Path $secretDirectory -Force | Out-Null
$bytes = New-Object byte[] 32
$rng = [Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
$password = [Convert]::ToBase64String($bytes)
$rng.Dispose()
[IO.File]::WriteAllText($secretPath, $password, [Text.Encoding]::ASCII)
Remove-Variable password, bytes, rng

$settings = @(
  "TEST_POSTGRES_USER=formo3d_test",
  "TEST_POSTGRES_CONTAINER=formo3d-postgres-test-$id",
  "TEST_POSTGRES_VOLUME=formo3d_pgtest_$id",
  "TEST_POSTGRES_PASSWORD_FILE=$secretPath",
  "TEST_COMPOSE_PROJECT=formo3d-pgtest-$id"
)
$settings | Set-Content -LiteralPath .env.test -Encoding ascii
```

Confirme manualmente as ACLs do arquivo de senha antes de uso. Não imprima o arquivo, senha, URL completa ou objeto `$settings`.

## Guardas e comandos operacionais futuros

Os comandos abaixo são instruções para outra execução, após autorização explícita. Não foram executados nesta etapa.

### 1. Verificar Docker e colisões (read-only)

```powershell
docker context show
docker info --format '{{.ServerVersion}}'
$testSettings = ConvertFrom-StringData (Get-Content -Raw .env.test)
if (docker ps -aq --filter "name=^/$($testSettings.TEST_POSTGRES_CONTAINER)$") {
  throw "Container de teste já existe; não reutilize. Escolha outro nome após inspeção."
}
docker volume inspect $testSettings.TEST_POSTGRES_VOLUME *> $null
if ($LASTEXITCODE -eq 0) { throw "Volume já existe; não reutilize." }
if ($LASTEXITCODE -ne 1) { throw "Não foi possível provar ausência do volume; interrompa." }
if (Get-NetTCPConnection -State Listen -LocalPort 5433 -ErrorAction SilentlyContinue) {
  throw "Porta 5433 já está ocupada; interrompa e investigue."
}
```

Se Docker não estiver acessível ou qualquer colisão ocorrer, interrompa. Não use `docker compose up` no Compose comercial.

### 2. Validar o Compose sem criar recursos

```powershell
$testSettings = ConvertFrom-StringData (Get-Content -Raw .env.test)
docker compose --env-file .env.test -f compose.test.yaml -p $testSettings.TEST_COMPOSE_PROJECT config --quiet
if ($LASTEXITCODE -ne 0) { throw "Configuração do Compose de teste inválida." }
```

O `--env-file .env.test` é obrigatório: não omita nem troque pelo `.env` comercial.

### 3. Provisionar manualmente, somente após autorização

```powershell
$testSettings = ConvertFrom-StringData (Get-Content -Raw .env.test)
docker compose --env-file .env.test -f compose.test.yaml -p $testSettings.TEST_COMPOSE_PROJECT up -d postgres-test
if ($LASTEXITCODE -ne 0) { throw "Falha ao iniciar PostgreSQL isolado." }
```

Esse comando cria um container, rede e volume novos. **Não o execute sem aprovação explícita.** Ele não executa SQL nem migrations e deixa `loja_3d_test` ainda inexistente até autorização/provisionamento manual posterior. O healthcheck consulta apenas o nome `loja_3d_test` e não serve, por si só, como prova de existência do banco.

### 4. Verificar container, PostgreSQL e migrations sem segredo

```powershell
$testSettings = ConvertFrom-StringData (Get-Content -Raw .env.test)
docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}sem-healthcheck{{end}} {{json .NetworkSettings.Ports}} {{range .Mounts}}{{.Name}}@{{.Destination}};{{end}}' $testSettings.TEST_POSTGRES_CONTAINER
docker exec $testSettings.TEST_POSTGRES_CONTAINER pg_isready -h 127.0.0.1 -p 5432 -U $testSettings.TEST_POSTGRES_USER -d loja_3d_test
```

Depois que `loja_3d_test` tiver sido criado em operação autorizada, valide identidade, versão do servidor e migration com consulta read-only:

```powershell
docker exec $testSettings.TEST_POSTGRES_CONTAINER psql -U $testSettings.TEST_POSTGRES_USER -d loja_3d_test -X -v ON_ERROR_STOP=1 -At -c "SELECT current_database() || '|' || current_setting('server_version') || '|' || version::text || '|' || dirty::text FROM public.schema_migrations;"
```

Resultado esperado para homologação: `loja_3d_test|17.6|28|false`. Se a tabela não existir, estiver em outra versão ou `dirty=true`, interrompa. Não execute migrations nesta etapa.

### 5. API exclusivamente no banco de teste

O backend atual não tem um modo de homologação com validação interna do DSN. O guard operacional abaixo é obrigatório para o terminal que inicia a API. Obtenha a `TEST_DATABASE_URL` local por meio aprovado, apontando a `localhost:5433/loja_3d_test`; não a escreva na tela. A variável `DATABASE_URL` desse processo será substituída pela URL de teste antes do startup:

```powershell
if (-not $env:DATABASE_URL -or -not $env:TEST_DATABASE_URL) { throw "URLs necessárias ausentes." }
$dev = [uri]$env:DATABASE_URL
$test = [uri]$env:TEST_DATABASE_URL
if ($dev.AbsolutePath.TrimStart("/") -cne "loja_3d" -or
    $test.AbsolutePath.TrimStart("/") -cne "loja_3d_test" -or
    $env:DATABASE_URL -ceq $env:TEST_DATABASE_URL -or
    $test.Host -notin @("localhost", "127.0.0.1", "::1") -or $test.Port -ne 5433) {
  throw "Proteção do banco de teste falhou; API não iniciada."
}
$env:DATABASE_URL = $env:TEST_DATABASE_URL
$env:HTTP_HOST = "127.0.0.1"
$env:HTTP_PORT = "8080"
$env:FRONTEND_ORIGIN = "http://localhost:3000"
Set-Location .\backend
go run ./cmd/api
```

O terminal precisa receber também a configuração MFA existente, obtida de armazenamento operacional autorizado; não cole nem exiba o segredo. Não rode `cmd/migrate` neste procedimento. Sem o guard, `DATABASE_URL` pode voltar a apontar para `loja_3d`.

Endpoints read-only para confirmar depois do startup: `GET http://localhost:8080/api/v1/health` e `GET http://localhost:8080/api/v1/ready`. `ready` comprova que o pool do processo conectou, mas a garantia do destino decorre do guard aplicado no mesmo terminal.

### 6. Frontend e Playwright

Após API pronta, em outro terminal:

```powershell
$env:API_URL = "http://localhost:8080"
$env:NEXT_PUBLIC_API_URL = "http://localhost:8080"
Set-Location .\frontend
npm run dev -- --hostname 127.0.0.1 --port 3000
```

Verifique `http://localhost:3000/admin/login`. Para teste autenticado, a conta administrativa com MFA deve existir no banco de teste; não reutilize uma sessão de outro banco nem faça bootstrap/cópia de usuários sem aprovação. Se necessário, o operador autentica manualmente pelo fluxo MFA existente.

Os testes visuais atuais salvam screenshots em diretório fixo, limpam o `outputDir` do Playwright e o reporter padrão substitui o JSON e a matriz documental. Antes da execução, arquive localmente `frontend/test-results/visual-homologation/etapa-10.10.2.4.4/`, `frontend/test-results/playwright-run-artifacts/`, o report HTML e `docs/evidence/production-visual-matrix.md`. Os artefatos podem conter dados administrativos; não os sincronize nem compartilhe. Para reduzir efeitos sobre a documentação, o comando futuro pode usar `--reporter=line`, mas os screenshots do caminho fixo ainda serão substituídos:

```powershell
npx playwright test playwright/tests/production-pages.spec.ts --grep "production-history|production-job-detail" --reporter=line
```

Ele cobre as quatro resoluções para `/admin/producao/historico` e navega a detalhe de job somente por link/ID real. Se não houver job real ou a sessão MFA não for válida para `loja_3d_test`, interrompa sem inventar fixture.

### 7. Encerramento futuro

Após encerrar os testes e a API/frontend, pare somente o serviço isolado usando o mesmo `.env.test` e o mesmo projeto:

```powershell
$testSettings = ConvertFrom-StringData (Get-Content -Raw .env.test)
docker compose --env-file .env.test -f compose.test.yaml -p $testSettings.TEST_COMPOSE_PROJECT stop postgres-test
```

`stop` preserva container e volume. Não use `docker compose down -v` nem remova o volume. Remoção/deprovisionamento exige autorização separada e verificação explícita do nome do volume.

## Lacunas e riscos

1. **Inventário runtime pendente:** nesta revalidação, `docker compose config` funcionou, mas o acesso ao Docker Engine via `docker version` falhou com acesso negado ao pipe `dockerDesktopLinuxEngine`. Containers, volumes existentes e disponibilidade real da porta 5433 continuam não verificados.
2. **Backend sem guard interno:** a API apenas exige `DATABASE_URL`; o bloqueio em PowerShell protege somente o caminho de inicialização documentado. Proposta para etapa futura, mediante aprovação: adicionar um modo explícito de homologação que valide antes de abrir o pool o banco `loja_3d_test`, host loopback e porta 5433, e falhe fechado; cobrir com testes unitários. Não foi implementado.
3. **Banco e migration não provisionados:** Compose configura PostgreSQL 17.6, mas não cria `loja_3d_test` nem migra schema. A criação do banco e aplicação/validação de migration 28 precisam de autorização/fase próprias.
4. **Autenticação:** login visual autenticado depende de usuário/MFA existente no banco isolado e da chave de criptografia correta. Nenhuma conta ou segredo foi copiado.
5. **Porta/volume:** verificar porta 5433 livre e nomes de container/volume inéditos antes de provisionar; conflito implica parada, nunca reutilização.
6. **Imagem externa:** se `postgres:17.6-alpine` não estiver no cache, será necessário baixar do registry, com aprovação operacional.

## Aplicação controlada das migrations 1–28

Foi preparado [`backend/scripts/apply-test-migrations.ps1`](../../backend/scripts/apply-test-migrations.ps1) para ser executado manualmente no PowerShell que já possui `DATABASE_URL` para o banco isolado. O script não lê `.env` nem usa fallback, não inicia containers, não executa `down` e não altera `DATABASE_URL`.

Antes de `up`, o script valida host loopback, porta 5433, banco `loja_3d_test`, usuário `formo3d_test`; estado `running/healthy`, imagem PostgreSQL 17.6, projeto Compose `formo3d-test`, mount de volume nomeado exclusivamente `formo3d_postgres_test_data` e publicação somente em `127.0.0.1:5433`. Consulta identidade e estado inicial por `SELECT` read-only, exige `version none`, confirma os 28 pares completos, cria uma cópia exclusiva em `$env:TEMP`, verifica os 56 SHA-256 e aponta `MIGRATIONS_PATH` à cópia antes de chamar `go run ./cmd/migrate up`. Não chama o migrador `version` antes do `up`, pois inicializar o driver PostgreSQL do golang-migrate pode criar a tabela `schema_migrations` se ela estiver ausente.

Após o `up`, exige `version: 28, dirty: false` e revalida banco, usuário e versão. A cópia somente é removida após todas as verificações finais; em erro, é preservada e o caminho é informado. O valor anterior de `MIGRATIONS_PATH` e o diretório atual do shell são restaurados em `finally`.

Comando manual (não executado durante a preparação):

```powershell
Set-Location <raiz-do-repositorio>\backend
.\scripts\apply-test-migrations.ps1
```

Estado da preparação: script analisado estaticamente; a execução real depende do PowerShell operacional com a URL de teste e do Docker Engine acessível. Nenhuma migration foi aplicada por esta alteração documental/de script.

## API Go vinculada ao banco isolado

O bootstrap da API reconhece `APP_ENV=homologation` e valida o `DATABASE_URL` **antes** de criar o `pgxpool` ou tentar conectar. O guard aceita apenas uma URL PostgreSQL cujo host resolvido pelo pgx seja loopback, porta seja `5433`, banco seja `loja_3d_test` e usuário seja `formo3d_test`; rejeita hosts/fallbacks múltiplos e não inclui a URL em mensagens de erro. Não há fallback para `.env` ou para `DATABASE_URL` comercial. Com `APP_ENV=production` ou `development`, o comportamento existente permanece inalterado.

Execução manual, somente após autorização operacional e com `DATABASE_URL` já injetada no processo por armazenamento aprovado (não imprima a URL):

```powershell
$env:APP_ENV = 'homologation'
$env:HTTP_HOST = '127.0.0.1'
$env:HTTP_PORT = '8080'
$env:FRONTEND_ORIGIN = 'http://localhost:3000'
# DATABASE_URL deve estar previamente definida e protegida nesta sessão.
Set-Location <raiz-do-repositorio>\backend
go run ./cmd/api
```

O processo também deve receber a chave MFA legítima do ambiente isolado (`AUTH_MFA_ENCRYPTION_KEY`, versão em `AUTH_MFA_KEY_VERSION` e, se aplicável, chaves antigas em `AUTH_MFA_DECRYPTION_KEYS`) sem colocá-las no repositório, histórico do shell ou logs; `homologation` falha no startup se o keyring MFA não for válido. Este procedimento não cria usuários nem contorna MFA. O guard valida o destino antes de qualquer conexão; o startup normal então inicializa e faz `Ping` do pool. A API não aplica migrations.

### Bootstrap administrativo em homologação

O CLI oficial `go run ./cmd/admin bootstrap` também valida `DATABASE_URL` quando `APP_ENV=homologation`, antes de solicitar dados interativos ou criar o pool. Reutiliza `ValidateHomologationDatabaseURL`; portanto aceita somente loopback na porta `5433`, banco `loja_3d_test` e usuário `formo3d_test`, rejeitando hosts alternativos/remotos e overrides incompatíveis sem registrar a URL. Fora de homologação, o comportamento anterior do CLI permanece.

Procedimento manual, somente após autorização para criar a conta e com `DATABASE_URL` do banco isolado já fornecida por mecanismo operacional aprovado (não imprimir nem colocar a URL no histórico):

```powershell
$env:APP_ENV = 'homologation'
$env:ADMIN_BOOTSTRAP_ENABLED = 'true'
Set-Location <raiz-do-repositorio>\backend
try {
  go run ./cmd/admin bootstrap
} finally {
  $env:ADMIN_BOOTSTRAP_ENABLED = 'false'
}
```

O CLI falha fechado se o destino não passar pelo guard. Ele solicita senha sem eco, grava bcrypt e usa o bootstrap transacional de uso único, com evento `admin.bootstrap` na mesma transação. O primeiro login configura TOTP pelo fluxo administrativo normal; a API de homologação precisa receber o keyring MFA legítimo do ambiente. Esta etapa não executou o comando nem criou administrador.

Após a inicialização manual, os checks HTTP são:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/api/v1/health
Invoke-RestMethod http://127.0.0.1:8080/api/v1/ready
```

`health` confirma o processo HTTP. `ready` confirma conectividade do pool PostgreSQL e responde 503 sem detalhes internos quando indisponível. O CORS mantém allowlist exata derivada de `FRONTEND_ORIGIN`; para esta sessão local é `http://localhost:3000`.

Testes unitários do guard cobrem URL válida, porta 5432, banco comercial, usuário incorreto, host externo/múltiplo, override de host e URL ausente. A configuração e os testes não abrem conexão PostgreSQL; não houve inicialização de API/frontend.

## Resultado desta etapa

- Compose comercial e `.env` preservados.
- Nenhum dado comercial acessado ou alterado.
- A validação estática estrutural confirmou volume físico e projeto de testes distintos; isso não substitui a inspeção runtime de colisões.
- O arquivo de senha configurado não foi localizado; nenhum segredo foi lido ou exibido.
- Nenhuma chamada Docker pôde listar containers/volumes por indisponibilidade do pipe; nenhum recurso foi criado.
- Configuração e procedimento isolado preparados para revisão. Provisionamento, criação manual do banco, migrations e homologação permanecem pendentes de autorização explícita.
