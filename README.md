# Loja 3D

Frontend Hybrid Premium para uma plataforma de impressão 3D, integrado à API Go e ao catálogo PostgreSQL. Orçamento, checkout e acompanhamento de pedido ainda são demonstrativos e não executam operações comerciais reais.

## Pré-requisitos

- Node.js 22 ou superior e npm
- Go 1.25 ou superior
- Docker Desktop com Docker Compose v2
- Git

## Configuração

Na raiz do projeto, copie o arquivo de exemplo:

```powershell
Copy-Item .env.example .env
```

Use credenciais locais próprias em `.env`. O arquivo é ignorado pelo Git. Depois, crie a configuração pública do frontend:

```powershell
Copy-Item .env.example frontend/.env.local
```

O Next.js usa `API_URL` nos Server Components e `NEXT_PUBLIC_API_URL` nas consultas do carrinho no navegador. As URLs não contêm credenciais.

## PostgreSQL

Suba o banco e confira o healthcheck:

```powershell
docker compose up -d postgres
docker compose ps
```

Os dados são preservados no volume nomeado `postgres_data`. O startup da API não altera o schema; migrations são executadas explicitamente pelo comando independente documentado abaixo.

Para interromper o serviço, execute `docker compose down`. Adicionar `-v` também remove o volume e todos os dados locais.

## Backend

O módulo usa provisoriamente `example.com/loja-3d/backend`. Troque esse caminho quando existir um repositório remoto definitivo.

Em outro terminal:

```powershell
Set-Location backend
$env:HTTP_PORT = "8080"
$env:FRONTEND_ORIGIN = "http://localhost:3000"
$env:DATABASE_URL = "postgres://loja_3d:change_me_for_local_development@localhost:5432/loja_3d?sslmode=disable"
$env:DB_MAX_CONNS = "10"
$env:DB_MIN_CONNS = "2"
$env:DB_CONNECT_TIMEOUT = "5s"
go run ./cmd/api
```

A API valida a configuração, inicializa um pool `pgxpool`, confirma a conexão com `Ping` e fecha o pool durante o encerramento do processo. Credenciais e a URL de conexão não são registradas em logs.

Para uploads de orçamento, configure `R2_ACCOUNT_ID`, `R2_BUCKET`, `R2_ACCESS_KEY_ID` e `R2_SECRET_ACCESS_KEY` somente no ambiente da API. O bucket privado deve permitir CORS `PUT` de `http://localhost:3000` com os headers `Content-Type` e `x-amz-meta-sha256`. Sem essas variáveis, a API inicia, mas o upload fica indisponível.

### Migrations

Com o PostgreSQL ativo, execute a partir de `backend`:

```powershell
$env:DATABASE_URL = "postgres://loja_3d:change_me_for_local_development@localhost:5432/loja_3d?sslmode=disable"
go run ./cmd/migrate up
go run ./cmd/migrate version
```

`down` desfaz uma migration por padrão. Também aceita um número explícito de etapas:

```powershell
go run ./cmd/migrate down
go run ./cmd/migrate down 2
```

O comando procura os arquivos em `backend/migrations`. Para execução a partir de outro diretório, configure `MIGRATIONS_PATH` com uma URL `file://` adequada. Migrations nunca são aplicadas automaticamente durante o startup da API.

### Catálogo demonstrativo

Após aplicar as migrations, carregue o seed separado:

```powershell
Get-Content -Raw seeds/development.sql | psql $env:DATABASE_URL
```

O seed contém produtos fictícios e não deve ser usado como catálogo de produção.

Teste o endpoint:

```powershell
Invoke-RestMethod http://localhost:8080/api/v1/health
```

Resposta esperada:

```json
{
  "status": "ok",
  "service": "loja-3d-api"
}
```

Readiness do PostgreSQL:

```powershell
Invoke-RestMethod http://localhost:8080/api/v1/ready
```

Resposta com o banco disponível:

```json
{
  "status": "ready",
  "database": "connected"
}
```

`/api/v1/health` verifica somente o processo HTTP e continua respondendo 200 quando o banco está indisponível. `/api/v1/ready` executa um `Ping` com timeout e retorna 503 sem expor detalhes internos quando o PostgreSQL não pode ser alcançado.

API pública do catálogo:

```text
GET /api/v1/products
GET /api/v1/products/{slug}
GET /api/v1/categories
```

`/products` aceita `category`, `search`, `color`, `min_price`, `max_price`, `sort`, `page` e `limit`. Preços são representados em centavos. O contrato completo e as diferenças para os mocks do Next.js estão em `docs/api-products.md`.

## Frontend

Em outro terminal:

```powershell
Set-Location frontend
npm install
npm run dev
```

Acesse http://localhost:3000. O frontend possui as rotas:

- `/`: home Hybrid Premium;
- `/produtos` e `/produtos/[slug]`: catálogo PostgreSQL, filtros, paginação e variantes;
- `/personalizados`: simulação visual de orçamento, sem upload;
- `/carrinho`: IDs persistidos em `localStorage`, com produto e preço revalidados na API;
- `/checkout`: protótipo sem coleta de dados sensíveis ou pagamento;
- `/acompanhar-pedido`: timeline com pedido fictício;
- `/sobre`: apresentação provisória da marca.

O frontend consome `/api/v1/products`, `/api/v1/products/{slug}` e `/api/v1/categories`. Inicie PostgreSQL, API e frontend para navegar pelo catálogo.

## Testes e verificações

Backend:

```powershell
Set-Location backend
go test ./...
go vet ./...
```

O teste de integração do pool é habilitado somente quando `TEST_DATABASE_URL` estiver definido:

```powershell
$env:TEST_DATABASE_URL = "postgres://loja_3d:change_me_for_local_development@localhost:5432/loja_3d?sslmode=disable"
go test ./internal/infrastructure/postgres -run TestNewPoolIntegration -v
```

Frontend:

```powershell
Set-Location frontend
npm run lint
npm test
npm run build
```

Compose:

```powershell
docker compose config
```

## Organização

- `frontend/src/app`: App Router, metadata e oito experiências públicas.
- `frontend/src/components`: layout, catálogo, detalhe, orçamento, carrinho e componentes de UI.
- `frontend/src/data`: mocks legados preservados como referência, fora do fluxo comercial atual.
- `frontend/src/types`: modelos da API, variantes, paginação e carrinho.
- `frontend/src/lib`: configuração e utilitários internos.
- `frontend/src/services`: cliente HTTP tipado, mapeadores do catálogo e health check.
- `backend/cmd/api`: ponto de entrada e ciclo de vida do servidor HTTP.
- `backend/cmd/migrate`: comando independente para `up`, `down` e `version`.
- `backend/internal/http`: roteamento, handlers e middleware CORS.
- `backend/internal/domain/catalog`: entidades e regras do catálogo.
- `backend/internal/application/catalog`: casos de uso e porta do repositório.
- `backend/internal/infrastructure/postgres`: configuração, pool e readiness do PostgreSQL.
- `backend/migrations`: migrations SQL reversíveis.

## Limites do protótipo

Não há autenticação, pagamento, frete, envio de formulário, upload, reserva de estoque ou consulta real de pedidos. O catálogo é real no banco local; checkout e orçamento continuam protótipos. URLs externas de imagens exigirão allowlist quando o object storage for definido.

