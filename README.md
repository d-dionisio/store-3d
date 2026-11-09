# Loja 3D

Protótipo frontend Hybrid Premium para uma plataforma de impressão 3D, acompanhado pela base local da API Go e PostgreSQL. Os produtos, preços, prazos, orçamento, carrinho, checkout e acompanhamento de pedido ainda são demonstrativos e não executam operações comerciais reais.

## Pré-requisitos

- Node.js 22 ou superior e npm
- Go 1.24 ou superior
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

O Next.js lê somente `NEXT_PUBLIC_API_URL` desse arquivo; as demais variáveis não são expostas ao navegador porque não usam o prefixo `NEXT_PUBLIC_`.

## PostgreSQL

Suba o banco e confira o healthcheck:

```powershell
docker compose up -d postgres
docker compose ps
```

Os dados são preservados no volume nomeado `postgres_data`. Nenhuma tabela de negócio é criada nesta etapa.

Para interromper o serviço, execute `docker compose down`. Adicionar `-v` também remove o volume e todos os dados locais.

## Backend

O módulo usa provisoriamente `example.com/loja-3d/backend`. Troque esse caminho quando existir um repositório remoto definitivo.

Em outro terminal:

```powershell
Set-Location backend
$env:HTTP_PORT = "8080"
$env:FRONTEND_ORIGIN = "http://localhost:3000"
$env:DATABASE_URL = "postgres://loja_3d:change_me_for_local_development@localhost:5432/loja_3d?sslmode=disable"
go run ./cmd/api
```

`DATABASE_URL` já está preparada para a futura integração, mas a API ainda não abre conexão com o banco nesta etapa.

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

## Frontend

Em outro terminal:

```powershell
Set-Location frontend
npm install
npm run dev
```

Acesse http://localhost:3000. O frontend possui as rotas:

- `/`: home Hybrid Premium;
- `/produtos` e `/produtos/[slug]`: catálogo mockado e detalhe interativo;
- `/personalizados`: simulação visual de orçamento, sem upload;
- `/carrinho`: carrinho demonstrativo persistido em `localStorage`;
- `/checkout`: protótipo sem coleta de dados sensíveis ou pagamento;
- `/acompanhar-pedido`: timeline com pedido fictício;
- `/sobre`: apresentação provisória da marca.

A variável `NEXT_PUBLIC_API_URL` continua preparada para futuras integrações com a API Go. Nenhuma API comercial é consumida nesta etapa.

## Testes e verificações

Backend:

```powershell
Set-Location backend
go test ./...
go vet ./...
```

Frontend:

```powershell
Set-Location frontend
npm run lint
npm run build
```

Compose:

```powershell
docker compose config
```

## Organização

- `frontend/src/app`: App Router, metadata e oito experiências públicas.
- `frontend/src/components`: layout, catálogo, detalhe, orçamento, carrinho e componentes de UI.
- `frontend/src/data`: catálogo comercial fictício e tipado.
- `frontend/src/types`: contratos locais de produtos e carrinho.
- `frontend/src/lib`: configuração e utilitários internos.
- `frontend/src/services`: clientes de serviços externos, começando pelo health check da API.
- `backend/cmd/api`: ponto de entrada e ciclo de vida do servidor HTTP.
- `backend/internal/http`: roteamento, handlers e middleware CORS.

Os diretórios `domain`, `application`, `infrastructure/postgres` e `migrations` serão adicionados quando tiverem código real. Mantê-los ausentes agora evita arquivos marcadores e abstrações sem responsabilidade.

## Limites do protótipo

Não há autenticação, pagamento, frete, envio de formulário, upload, reserva de estoque ou consulta real de pedidos. Imagens, catálogo, valores e prazos são demonstrativos. Os assets visuais foram gerados originalmente para este protótipo e armazenados localmente em `frontend/public/images`.

