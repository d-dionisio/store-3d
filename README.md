# Loja 3D

Storefront Lumé Studio para uma plataforma de objetos autorais produzidos por impressão 3D, integrado à API Go e ao catálogo PostgreSQL.

## Identidade Lumé Studio

O storefront público utiliza a linguagem **Editorial Luxury**: Cormorant Garamond e Inter via `next/font`, paleta marfim/grafite/dourado discreto, header transparente sobre a homepage e superfícies sólidas nas rotas internas. O painel administrativo mantém sua identidade Dark Tech independente.

Os assets editoriais são locais em `frontend/public/images`, evitando dependência de imagens remotas. O redesign não altera contratos da API, autenticação, carrinho, frete ou pagamentos.

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

Para uploads de orçamento, configure `R2_ACCOUNT_ID`, `R2_BUCKET`, `R2_ACCESS_KEY_ID` e `R2_SECRET_ACCESS_KEY` somente no ambiente da API. O bucket privado deve permitir CORS `PUT` de `http://localhost:3000` com os headers `Content-Type`, `x-amz-meta-sha256` e `x-amz-checksum-sha256`, expondo `ETag` e `x-amz-checksum-sha256`. Sem essas variáveis, a API inicia, mas o upload fica indisponível.

### Administrador local

Depois de aplicar a migration 10, crie ou rotacione uma conta sem colocar a senha em argumentos de linha de comando:

```powershell
$env:ADMIN_EMAIL = "admin@example.com"
$env:ADMIN_NAME = "Administrador"
$env:ADMIN_PASSWORD = Read-Host "Senha administrativa"
go run ./cmd/admin
```

Use ao menos 12 caracteres. Inicie a API com `APP_ENV=production` em produção para exigir cookie Secure. O painel fica em `http://localhost:3000/admin/login`.

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

## Propostas de orçamento

Após autenticar em `/admin/login`, abra `/admin/orcamentos/{reference}`. Um administrador pode iniciar a análise, salvar a composição em centavos e enviar a proposta. O envio exige todos os anexos em estado `clean`; não libera download dos arquivos.

Endpoints protegidos:

- `POST /api/v1/admin/quotes/{reference}/analysis`
- `PUT /api/v1/admin/quotes/{reference}/pricing`
- `POST /api/v1/admin/quotes/{reference}/proposal`
- `POST /api/v1/admin/quotes/expire`

As requisições mutáveis exigem o cookie de sessão e `X-CSRF-Token`. A migration `000011_create_quote_proposals` é aplicada com o comando independente já documentado. E-mail ainda não é enviado: a proposta fica registrada como `not_configured` até a configuração de um provedor.

## Área do cliente e magic link

Configure o adaptador local somente durante desenvolvimento:

```dotenv
APP_ENV=development
EMAIL_PROVIDER=development
FRONTEND_ORIGIN=http://localhost:3000
```

Em `/minha-conta`, informe o e-mail usado na solicitação. O adaptador local escreve no log da API um link de desenvolvimento claramente identificado; ele não entrega uma mensagem real. `EMAIL_PROVIDER=development` é ignorado quando `APP_ENV=production`.

Endpoints:

- `POST /api/v1/customer/magic-links`
- `POST /api/v1/customer/magic-links/verify`
- `DELETE /api/v1/customer/session`
- `GET /api/v1/customer/profile`
- `GET /api/v1/customer/quotes`
- `GET /api/v1/customer/quotes/{reference}`
- `POST /api/v1/customer/quotes/{reference}/decision`

As páginas protegidas ficam em `/minha-conta/orcamentos` e `/minha-conta/orcamentos/{reference}`. Aceite, recusa e logout exigem `X-CSRF-Token`. Para produção ainda é necessário implementar um provedor real de e-mail e substituir o adaptador indisponível.

## Conversão de proposta aprovada em pedido

Depois que o cliente aceita a versão mais recente de uma proposta, a tela do orçamento disponibiliza “Converter em pedido”. A operação usa:

```http
POST /api/v1/customer/quotes/{reference}/convert
Cookie: loja3d_customer_session=...
X-CSRF-Token: ...
```

Resposta inicial ou idempotente:

```json
{
  "data": {
    "reference": "PED-...",
    "status": "pending_payment"
  }
}
```

O pedido apenas registra a obrigação comercial aprovada. Checkout, pagamento, frete, estoque e produção não são iniciados nesta etapa. A estrutura necessária é aplicada pela migration `000013_add_quote_order_snapshots`.

## Checkout

O checkout de catálogo revalida o carrinho no backend e aceita somente IDs de variantes e quantidades. Endpoints principais:

- `POST /api/v1/customer/cart/validate`
- `GET|POST /api/v1/customer/addresses`
- `POST /api/v1/customer/orders/catalog`
- `PUT /api/v1/customer/orders/{reference}/delivery-address`
- `POST /api/v1/customer/orders/{reference}/prepare-payment`
- `GET /api/v1/customer/orders`
- `GET /api/v1/customer/orders/{reference}`

`POST /orders/catalog` exige `Idempotency-Key`, sessão do cliente e CSRF. Pedidos que precisam de frete permanecem em `awaiting_shipping_quote`; nenhum preço de entrega é simulado. As páginas ficam em `/checkout`, `/checkout/confirmacao`, `/minha-conta/pedidos` e `/minha-conta/pedidos/{reference}`.

## Cotação de frete — Melhor Envio

Configure no `.env` um token do sandbox, `MELHOR_ENVIO_USER_AGENT` com nome e e-mail técnico e o CEP de origem. A API usa `MELHOR_ENVIO_BASE_URL=https://sandbox.melhorenvio.com.br` por padrão de desenvolvimento.

- `POST /api/v1/customer/orders/{reference}/shipping-quotes`: consulta ou reaproveita modalidades ainda válidas.
- `POST /api/v1/customer/orders/{reference}/shipping-selection`: seleciona uma cotação pertencente ao pedido e preserva seu snapshot.
- `PUT /api/v1/admin/orders/{reference}/packages`: registra peso e dimensões finais de pedidos personalizados.

As três operações exigem sessão e CSRF; as duas primeiras exigem propriedade do pedido. Para aplicar a estrutura: `cd backend`, `go run ./cmd/migrate up` e `go run ./cmd/migrate version`. Sem token configurado, a API continua iniciando, mas a cotação retorna HTTP 503 e nenhum valor fictício é usado.

## Pagamento PIX com Mercado Pago

Configure `MERCADO_PAGO_ENVIRONMENT=sandbox`, o access token e a assinatura secreta do webhook de teste. Credenciais de produção possuem variáveis separadas e não devem ser copiadas para arquivos versionados.

- `POST /api/v1/customer/orders/{reference}/payments/pix`: cria ou recupera a tentativa PIX idempotente.
- `GET /api/v1/customer/orders/{reference}/payment`: consulta o estado financeiro persistido.
- `POST /api/v1/webhooks/mercado-pago?data.id={order_id}`: recebe notificações assinadas do tópico `order`.

Cadastre a URL pública HTTPS do último endpoint no painel do Mercado Pago para o tópico de Orders. O webhook consulta a order diretamente no provedor antes de confirmar o pedido. Sem credenciais configuradas, a API inicia normalmente e a criação de PIX retorna HTTP 503.

## Pagamento com cartão

Copie `NEXT_PUBLIC_MERCADO_PAGO_PUBLIC_KEY` para `frontend/.env.local` usando a Public Key do mesmo ambiente do Access Token. O frontend carrega o MercadoPago.js v2 oficial e o Card Payment Brick consulta as modalidades e parcelas. PAN e CVV permanecem nos Secure Fields; a API recebe somente token efêmero, método, parcelas e identificação do pagador.

- `POST /api/v1/customer/orders/{reference}/payments/card`: cria a tentativa com sessão, CSRF e `Idempotency-Key`.
- Quando o provedor exige 3DS, a resposta contém `RequiresAction` e uma URL HTTPS validada do domínio Mercado Pago.
- O webhook de Orders continua sendo a única confirmação assíncrona que muda o pedido para confirmado.

Aplique a migration 17 com `cd backend` e `go run ./cmd/migrate up`. Use Public Key e Access Token de teste da mesma aplicação e nunca coloque o Access Token em uma variável `NEXT_PUBLIC_*`.

## Proxy temporário do webhook

Para uma validação externa autorizada, mantenha a API em `127.0.0.1:8080` e execute `go run ./cmd/webhook-proxy` no diretório `backend`. O proxy escuta em `:8090` e só encaminha `POST /api/v1/webhooks/mercado-pago`; todas as demais rotas ficam bloqueadas. Aponte o túnel HTTPS exclusivamente para a porta 8090. O procedimento e os testes de bloqueio estão em `docs/mercado-pago-webhook-proxy.md`. O projeto nunca abre o túnel automaticamente.

