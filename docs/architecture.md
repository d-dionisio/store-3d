# Arquitetura da Loja 3D

Este documento registra as decisões técnicas vigentes da Loja 3D. Ele deve ser atualizado sempre que uma etapa alterar a arquitetura, os limites do sistema ou uma decisão descrita abaixo.

Última atualização: 9 de novembro de 2026 — Etapa 7.2.

## Visão geral

A Loja 3D é uma plataforma de e-commerce com dois fluxos comerciais:

1. Venda de produtos próprios impressos em 3D.
2. Solicitação de orçamento para impressões personalizadas a partir de arquivos STL ou 3MF.

A implementação permanece em um monorepo e, neste momento, não utiliza microsserviços.

```text
Next.js frontend → API Go → PostgreSQL
                         ↘ object storage futuro para imagens e modelos 3D
```

## Stack aprovada

- Frontend: Next.js com App Router, React, TypeScript e Tailwind CSS.
- Backend: Go com `net/http` e roteamento da biblioteca padrão.
- PostgreSQL: versão 17.
- Driver: `github.com/jackc/pgx/v5` e `pgxpool`.
- Migrations: `github.com/golang-migrate/migrate/v4` com SQL em arquivos.
- Desenvolvimento local: Docker Compose.
- Identificadores principais: UUID com `gen_random_uuid()`.
- Valores monetários: `BIGINT` em centavos.
- Datas operacionais: `TIMESTAMPTZ`.

## Organização do repositório

```text
frontend/                 aplicação Next.js
backend/cmd/api/          processo HTTP
backend/cmd/migrate/      comando independente de migrations
backend/internal/http/    roteamento, handlers e middleware
backend/internal/infrastructure/postgres/
                          configuração, pool e readiness
backend/migrations/       migrations estruturais up/down
backend/seeds/            dados iniciais separados do schema
backend/tests/            testes SQL de integridade
docs/                     documentação arquitetural
```

Os diretórios de domínio e aplicação serão criados somente quando surgirem responsabilidades comerciais reais. Não serão adicionadas interfaces ou abstrações sem consumidores concretos.

## Frontend

O frontend segue a identidade Hybrid Premium:

- Luxury Minimal para catálogo e páginas comerciais.
- Dark Tech para impressão personalizada.
- Server Components por padrão e Client Components apenas onde há interação.
- Catálogo, categorias, destaques e detalhes carregados da API Go em Server Components.
- Filtros e paginação representados na URL, com a API como fonte de verdade.
- DTOs `snake_case` convertidos explicitamente para modelos TypeScript `camelCase`.
- Valores monetários mantidos em centavos inteiros até a apresentação.
- Carrinho persiste apenas IDs e quantidade; produto, variante e preço são revalidados na API.
- `API_URL` atende o servidor e `NEXT_PUBLIC_API_URL` o navegador; ambas são configurações públicas.
- Checkout, orçamento, frete e acompanhamento ainda são protótipos sem operação comercial real.

As imagens demonstrativas são assets originais locais. O frontend não deve depender de fotografias de produtos de terceiros apresentadas como itens da loja.

## API e ciclo de vida

A API usa um único processo Go e `http.Server` com timeouts explícitos. O encerramento é gracioso em `SIGINT` ou `SIGTERM`.

O pool PostgreSQL:

- É criado no `main` e não é global.
- Usa `DATABASE_URL`.
- Configura `DB_MAX_CONNS`, `DB_MIN_CONNS` e `DB_CONNECT_TIMEOUT`.
- Define lifetime, idle time e health period das conexões.
- Executa `Ping` com contexto e timeout antes de iniciar o servidor.
- É fechado durante o encerramento da API.
- Não registra DSN ou credenciais em logs.

## Health e readiness

- `GET /api/v1/health` é um liveness check do processo HTTP e não depende do banco.
- `GET /api/v1/ready` executa `Ping` no PostgreSQL.
- Banco disponível: readiness retorna HTTP 200.
- Banco indisponível: readiness retorna HTTP 503 sem detalhes internos da conexão.

Essa separação permite que infraestrutura de orquestração diferencie processo vivo de instância pronta para receber tráfego.

## API pública de catálogo

O catálogo segue a separação:

```text
HTTP DTOs → casos de uso → interface de repositório → PostgreSQL
```

- Entidades de domínio não possuem tags JSON nem dependem de HTTP ou PostgreSQL.
- DTOs HTTP são contratos públicos separados.
- A interface de repositório pertence à camada de aplicação.
- O repositório PostgreSQL usa `pgxpool`, contextos e queries parametrizadas.
- Produtos são paginados; variantes e imagens são carregadas em lote para evitar N+1.
- Ordenação SQL usa uma allowlist fechada.
- Apenas registros ativos e variantes comercializáveis são públicos.
- Variante com estoque controlado exige saldo disponível.
- Produtos sem variante comercializável não aparecem na API.

Endpoints atuais:

- `GET /api/v1/products`
- `GET /api/v1/products/{slug}`
- `GET /api/v1/categories`

## Administração

- O painel `/admin` usa identidade Dark Tech e é separado do layout público.
- Administradores pertencem a `admin_users`, sem reutilizar tokens de upload ou confiar em flags do frontend.
- Senhas usam bcrypt; criação/rotação ocorre somente pelo comando `cmd/admin` com variáveis de processo.
- Sessões usam token aleatório de 256 bits, armazenado apenas como SHA-256, expiram em 8 horas e podem ser revogadas.
- O cookie é HTTP-only, SameSite Strict e Secure quando `APP_ENV=production`.
- Operações mutáveis exigem token CSRF vinculado à sessão; login possui rate limiting e bloqueio temporário após falhas.
- `quote_viewer` e `admin` podem consultar orçamentos; não existem operações de aprovação ou preço nesta etapa.
- Login, logout, dashboard, listagem e detalhe geram eventos em `admin_audit_events`.
- Arquivos administrativos expõem apenas metadata; não há endpoint de download, chave de objeto ou URL assinada.

## Migrations

As migrations:

- São numeradas por domínio.
- Possuem pares reversíveis `.up.sql` e `.down.sql`.
- Permanecem como SQL explícito.
- Não são executadas automaticamente no startup da API.
- São aplicadas pelo comando `go run ./cmd/migrate`.
- Suportam `up`, `down [steps]` e `version`.
- Mantêm seeds fora das migrations estruturais.

Ordem atual:

1. Catálogo.
2. Clientes e endereços.
3. Orçamentos e arquivos.
4. Pedidos, itens e snapshots.
5. Frete e envios.
6. Pagamentos e reembolsos.
7. Produção.
8. Índices adicionais.

## Modelo comercial

### Catálogo e variantes

- Um produto possui uma categoria principal.
- Um produto possui uma ou mais variantes.
- A variante representa produto, material, cor, tamanho opcional e acabamento opcional.
- SKU, preço e estoque pertencem à variante.
- Combinações duplicadas são impedidas com `UNIQUE NULLS NOT DISTINCT`.
- Estoque disponível e reservado são controlados por variante.
- Imagens podem ser gerais do produto ou específicas da variante.
- FK composta garante que uma imagem não referencie variante de outro produto.

Produtos, categorias e variantes podem ser arquivados por soft delete porque participam de histórico comercial. Dados históricos não devem ser apagados em cascata.

### Orçamento para pedido

O fluxo aprovado é:

```text
solicitação → análise → proposta → aprovação → pedido → pagamento → produção
```

- Antes da aprovação, `quotes` é a fonte de verdade.
- Um orçamento aprovado origina no máximo um pedido.
- `orders.quote_id` é único quando preenchido.
- O subtotal do pedido originado de orçamento deve corresponder ao valor aprovado.
- Após a conversão, `orders` e `order_items` preservam o contrato comercial.
- Produção nunca começa diretamente no orçamento; ela referencia um item do pedido.

### Pedidos e snapshots

Pedidos armazenam snapshots de:

- Cliente e contato.
- Endereço.
- Nome e descrição do produto.
- SKU, material, cor, tamanho e acabamento.
- Preço unitário, quantidades e totais.
- Frete contratado.

Itens são classificados como `catalog` ou `custom`. Itens de catálogo exigem produto e variante coerentes. Itens personalizados exigem pedido originado de orçamento e não referenciam variante de catálogo.

### Frete

O provedor planejado é Melhor Envio. O modelo separa:

- `shipping_services`: serviço externo conhecido, sem preço fixo.
- `shipping_quotes`: cotação temporária por destino e pacote.
- `order_shipping`: snapshot do frete escolhido.
- `shipments`: pacote efetivamente despachado.
- `shipment_items`: itens e quantidades em cada envio.

Um pedido pode possuir vários envios. FKs compostas garantem que shipment e item pertençam ao mesmo pedido. Um trigger impede que a quantidade acumulada enviada ultrapasse a quantidade comprada.

### Pagamentos

O modelo não armazena PAN, CVV ou credenciais de cartão.

- `payments`: obrigação financeira consolidada do pedido.
- `payment_attempts`: tentativas individuais e idempotentes no provedor.
- `payment_status_events`: histórico de estados.
- `payment_webhook_events`: deduplicação e auditoria de notificações.
- `payment_refunds`: reembolsos idempotentes.

Identificadores externos são únicos por provedor. Webhooks podem referenciar a tentativa correspondente e notificações duplicadas são rejeitadas pela chave do evento.

### Produção

- `production_jobs` sempre referencia `order_items`.
- Produção exige pagamento confirmado com saldo pago positivo.
- Reimpressões são representadas por novos jobs para o mesmo item.
- Falhas, pós-processamento e controle de qualidade possuem estados próprios.

## Arquivos e object storage

Fotografias e modelos STL/3MF não são armazenados como binários no PostgreSQL.

O banco mantém:

- Provedor, bucket e chave do objeto.
- Nome original e tipo.
- Tamanho.
- Checksum SHA-256.
- Estado da verificação de segurança.
- Política de retenção e confirmação de exclusão.

Arquivos personalizados:

- Devem ficar em bucket privado.
- Possuem limite inicial de 200 MiB.
- Aceitam somente STL e 3MF.
- Precisam ser aprovados pela verificação de segurança antes da análise técnica.
- Devem ser acessados por URLs assinadas e temporárias.

### Upload público de orçamento

- O navegador envia STL/3MF diretamente ao bucket privado Cloudflare R2 por URL SigV4 válida por 15 minutos.
- `quote_uploads` representa autorizações transitórias; tokens de posse e consulta são persistidos somente como SHA-256.
- A autorização fixa chave aleatória, tamanho, tipo e checksum antes do PUT; o header padrão `x-amz-checksum-sha256` obriga o storage a validar o payload e a confirmação compara o resultado retornado pelo R2.
- 3MF exige assinatura ZIP, mas permanece `pending`: proteção completa contra decompression bomb depende do scanner assíncrono ainda não implementado.
- A criação de `quotes`, `quote_files` e o consumo do upload ocorre na mesma transação.
- Arquivos definitivos entram como `security_status = pending`, nunca como `clean`.
- Uploads abandonados dependem de job de limpeza futuro e lifecycle rule no bucket.

## Integridade e status

- Foreign Keys usam `RESTRICT`, `CASCADE` ou `SET NULL` conforme a responsabilidade histórica.
- Status são `TEXT` com `CHECK`, evitando enums PostgreSQL difíceis de evoluir.
- Constraints simples permanecem no banco.
- Regras que cruzam linhas ou tabelas usam triggers e também deverão ser respeitadas pelos serviços Go.
- Índices cobrem catálogo, histórico do cliente, filas de status, webhooks, retenção de arquivos e produção.

## Estratégia de testes

- Testes unitários não dependem de banco externo.
- Readiness usa uma dependência mínima injetada, permitindo simular sucesso e falha.
- Testes de integração usam `TEST_DATABASE_URL` e PostgreSQL isolado.
- As migrations são validadas em banco vazio no ciclo `up → down → up`.
- A suíte SQL de constraints roda em transação com rollback.

## Integração frontend/API

- O cliente HTTP aplica timeout de 8 segundos e erros tipados.
- Catálogo e detalhes usam `no-store`; categorias revalidam em 5 minutos.
- Busca, filtros, ordenação e paginação permanecem responsabilidade da API.
- Imagens locais usam `next/image`; hosts externos exigirão allowlist após a escolha do object storage.
- Falha da API, carregamento, resultado vazio e 404 possuem estados próprios.

## Limites atuais

Ainda não foram implementados:

- Autenticação e autorização.
- Repositories e serviços comerciais.
- APIs de pedidos e orçamentos.
- Upload e object storage real.
- Integração com Melhor Envio.
- Integração com provedor de pagamentos.
- Emissão fiscal.
- Painel administrativo.

## Decisões pendentes

- Provedor de autenticação e política de anonimização.
- Provedor de object storage e prazo definitivo de retenção.
- Provedor de pagamento e mapeamento dos seus estados.
- Regras de reserva e liberação concorrente de estoque.
- Política de pagamentos e reembolsos parciais.
- Estratégia de divisão automática em múltiplos pacotes.
- Emissão fiscal e retenção legal de dados.
- Suporte futuro a múltiplas categorias ou coleções por produto.

## Propostas administrativas (Etapa 7.3)

- `quotes` mantém o estado agregado do fluxo (`submitted → under_review → quoted → approved/rejected/expired`); `quote_proposals` preserva a composição comercial imutável por versão.
- Existe no máximo um rascunho por orçamento. O envio fecha o rascunho, copia preço, prazo e validade para o snapshot de `quotes` e impede um segundo envio concorrente pela combinação de lock pessimista e estado.
- Custos e preço final são `BIGINT` em centavos; margem é armazenada em basis points. O domínio rejeita custos negativos e preço inferior a custos mais margem.
- Alterações de preço, início da análise, envio e expiração registram auditoria dentro da mesma transação PostgreSQL.
- Uma proposta somente pode ser enviada quando há arquivo associado e todos os arquivos estão `clean`. Estados `pending`, `scanning`, `rejected` e `failed` permanecem bloqueados.
- O frete é explicitamente separado por `shipping_included`; o padrão é não incluído.
- A integração de e-mail ainda não está configurada. O envio comercial é persistido com `notification_status=not_configured`, sem simular entrega. O link seguro ao cliente depende da autenticação/capability prevista para a Etapa 7.4.
- Mutações administrativas exigem sessão `admin`, cookie HTTP-only e token CSRF. Leitores (`quote_viewer`) não podem iniciar análise nem precificar.

## Identidade e área do cliente (Etapa 7.4)

- A autenticação do cliente é independente da administrativa. `customer_magic_links` e `customer_sessions` nunca compartilham cookies, tokens ou tabelas com `admin_sessions`.
- Magic links usam 32 bytes aleatórios, persistem somente SHA-256, expiram em 15 minutos e são consumidos atomicamente uma única vez.
- O endpoint de solicitação sempre devolve uma resposta genérica. Isso evita revelar se um endereço possui conta ou orçamento.
- Após provar a titularidade do e-mail, o backend cria ou verifica `users` e associa somente os orçamentos legados ainda sem usuário cujo e-mail normalizado seja idêntico.
- Sessões duram 30 dias, são revogáveis e usam cookie HTTP-only, `Secure` em produção e `SameSite=Lax`. Decisões e logout exigem token CSRF separado.
- Toda consulta filtra simultaneamente por `user_id` e referência. Referências fornecidas pelo navegador não são consideradas autorização.
- `quote_proposal_decisions` identifica orçamento, versão exata, cliente, decisão e horário. Locks no orçamento e constraints únicas impedem aceite/recusa concorrente ou duplicado.
- Apenas a versão mais recente enviada, ainda válida e com todos os arquivos `clean`, pode ser aceita ou recusada. O conteúdo da proposta enviada permanece imutável.
- O adaptador `development` apenas registra a mensagem no log e só pode ser habilitado quando `APP_ENV=development`. Em produção, o adaptador permanece indisponível até a integração de um provedor real.
- A aceitação atualiza o orçamento para `approved`, mas não cria pedido, reserva estoque, cobra pagamento ou inicia produção.

## Conversão de orçamento aprovado em pedido (Etapa 8.1)

- A conversão é uma operação explícita do cliente após o aceite. Aprovar uma proposta não cria automaticamente um pedido.
- `CustomerRepository.ConvertQuote` bloqueia o orçamento com `SELECT ... FOR UPDATE`, valida proprietário, status `approved`, decisão `accepted`, versão mais recente e validade no instante da decisão.
- `orders.quote_id` permanece a proteção idempotente principal. O índice único existente impede mais de um pedido por orçamento; chamadas repetidas retornam o pedido já criado.
- O pedido inicia em `pending_payment`. Esse estado não representa pagamento, reserva de estoque, produção ou contratação de frete.
- A migration 13 adiciona ao pedido snapshots de `quote_proposal_id`, versão, prazo de fabricação, indicação de frete e observações comerciais.
- Itens personalizados mantêm quantidade e `approved_total_cents`. O total aprovado não é reconstruído por divisão unitária, evitando perda de centavos quando preço ÷ quantidade não é exato.
- Material, cor, dimensões, descrição, cliente, telefone, preço e quantidade são copiados no momento da conversão e não dependem de consultas futuras ao catálogo.
- Pedido, item, evento inicial e auditoria do cliente são gravados na mesma transação. Qualquer falha provoca rollback integral.

