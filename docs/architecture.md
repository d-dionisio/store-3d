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

## Checkout e fechamento de compras (Etapa 8.2)

- O armazenamento local do carrinho contém somente `productVariantId` e quantidade. Preço, nome, SKU, atributos e disponibilidade são reconsultados no PostgreSQL.
- O checkout de catálogo bloqueia as variantes durante a criação do pedido, valida produto, variante e atributos ativos, disponibilidade e limite de quantidade, e grava snapshots dos itens.
- `orders.checkout_key_hash` guarda somente SHA-256 da chave de idempotência. Um advisory lock por chave e o índice único impedem pedidos duplicados em requisições concorrentes.
- `order_addresses` é o snapshot de entrega. Alterações posteriores em `addresses` não modificam pedidos existentes.
- `payment_readiness` distingue `awaiting_address`, `awaiting_shipping_quote` e `ready`. O status comercial continua `pending_payment`.
- Produtos físicos exigem frete. Pedidos personalizados também exigem frete quando `shipping_included=false`. Sem `order_shipping` real, `PrepareOrderForPayment` retorna `shipping_required` e não inventa preço.
- O contrato futuro de Melhor Envio utiliza as tabelas existentes `shipping_quotes` e `order_shipping`; a Etapa 8.2 não cria cotações simuladas.

## Etapa 9.1 — cotação de frete

- A integração usa exclusivamente `POST /api/v2/me/shipment/calculate`. Token e `User-Agent` ficam no backend; o navegador nunca recebe credenciais do Melhor Envio.
- Produtos de catálogo usam peso e dimensões da variante. Pedidos personalizados usam `order_packages`, preenchido por administrador; dimensões de STL/3MF nunca são tratadas como embalagem.
- `shipping_quotes.order_id` impede selecionar uma cotação de outro pedido. `request_hash` permite reutilizar uma consulta idêntica enquanto ela estiver válida, reduzindo chamadas repetidas.
- `expires_at` é uma validade local (`SHIPPING_QUOTE_TTL`), pois o cálculo não fornece uma expiração universal. Uma cotação expirada não pode ser selecionada.
- `order_shipping` é o snapshot comercial escolhido. A seleção bloqueia concorrência pelo pedido, copia serviço, transportadora, preço e prazo e atualiza o total em uma única transação.
- Um pedido já `ready` ou fora de `pending_payment` não aceita troca de endereço, pacotes ou frete. A compra de etiquetas e rastreamento permanecem fora do escopo.

## Etapa 8.3 — pagamentos Mercado Pago

- PIX utiliza a Orders API (`POST /v1/orders`) com `X-Idempotency-Key`; consultas e webhooks do tópico `order` reconciliam por `GET /v1/orders/{id}`.
- O pedido precisa pertencer ao cliente, estar em `pending_payment`, possuir moeda BRL, total positivo e `payment_readiness=ready` antes da reserva da tentativa.
- `payments` representa a obrigação financeira; `payment_attempts` representa uma chamada idempotente ao provedor. Apenas uma obrigação aberta é permitida por pedido.
- O backend compara `external_reference`, moeda e valor em centavos antes de persistir ou confirmar. PIX expirado é encerrado antes de permitir nova obrigação.
- Webhooks validam HMAC-SHA256 em tempo constante usando `x-signature`, `x-request-id` e `data.id`. O payload apenas dispara consulta autenticada ao Mercado Pago e nunca confirma um pedido diretamente. Quando a notificação de Order não possui um ID de evento no topo, o SHA-256 do payload assinado funciona como chave estável de deduplicação.
- `provider_updated_at` impede regressão por eventos fora de ordem. Eventos são deduplicados, falhas podem ser reprocessadas e toda mudança gera histórico financeiro.
- Somente `paid` confirma o pedido. Estados pending/processing/authorized não iniciam produção. Reembolsos atualizam o agregado e criam registro em `payment_refunds`.
- Cartão usa MercadoPago.js v2 e o Card Payment Brick. Secure Fields mantêm PAN/CVV fora da aplicação; somente o token efêmero chega ao backend e nunca é persistido ou registrado em logs.
- A criação do pedido, itens, endereço, histórico e auditoria ocorre numa única transação. Não há captura de pagamento, produção nem reserva definitiva de estoque.

## Etapa 8.3.1 — validação do sandbox Mercado Pago

- Evidências unitárias, com HTTP mock, PostgreSQL e sandbox real são registradas separadamente em `docs/mercado-pago-sandbox-validation.md`.
- Access Token e assinatura de webhook exclusivos do ambiente de teste são obrigatórios; nunca devem ser enviados ao frontend ou registrados em logs.
- Em 11 de novembro de 2026, a credencial de teste criou e consultou uma order PIX oficial pela Orders API, incluindo QR Code e ticket. O adaptador foi migrado de `/v1/payments` para `/v1/orders` e o teste opt-in do próprio client passou contra o sandbox real.
- O request server-side da Orders API exige `payer.email`; dados adicionais de identificação não são inventados nem persistidos pelo adaptador.
- Nenhum sucesso obtido com mock ou banco local pode ser reportado como validação do sandbox.

## Etapa 8.4 — cartão e 3DS

- Cartão reutiliza `payments`, as tentativas idempotentes e a reconciliação de Orders já usadas pelo PIX; não existe um segundo agregado financeiro.
- A order usa captura automática e `config.online.transaction_security`. `action_required/pending_challenge` continua pendente e nunca confirma pedido.
- A migration 17 preserva parcelas, método e URL temporária do desafio. Token do cartão e documento do pagador são transitórios e não entram no PostgreSQL.
- A URL 3DS só é aceita em HTTPS no domínio `mercadopago.com` ou subdomínio. Apenas a consulta autenticada disparada pelo webhook confirma o pagamento.
- Uma obrigação aberta impede criar outra modalidade no mesmo pedido. Trocar PIX por cartão após gerar cobrança exige cancelamento ou expiração explícita, evitando cobranças simultâneas.
- O Card Payment Brick obtém do provedor as condições de parcelamento para o total atual; a loja não inventa parcelas.
- Recusas comerciais da Orders API podem usar HTTP 402 com a Order no nó `data`; esse retorno é um resultado financeiro válido, não indisponibilidade do provedor.
- Evidências da validação de cartão são mantidas em `docs/mercado-pago-card-sandbox-validation.md`, sempre separando mock, PostgreSQL, sandbox e navegador.

## Etapa 8.4.2 — exposição restrita do webhook

- O sandbox não reproduziu o Challenge documentado para `APRO-CHOK` e `OTHE-CHNO`, embora o payload possua `on_fraud_risk`, `liability_shift=required` e captura automática. Sem `action_required/pending_challenge` e URL real, 3DS permanece não validado; não se altera o contrato por tentativa e erro.
- `cmd/webhook-proxy` é uma borda temporária e independente da API. Só encaminha `POST /api/v1/webhooks/mercado-pago`, preserva os headers HMAC e `data.id`, remove cookies/autorização, limita corpo e taxa e recusa upstream que não seja loopback.
- Um túnel HTTPS autorizado deve apontar para a porta 8090 do proxy, nunca para a 8080 da API. Assim, endpoints administrativos, autenticação de clientes e health checks não ficam publicamente roteáveis.
- O proxy não substitui a validação HMAC, a consulta autenticada da Order, a deduplicação nem a transação financeira já existentes. Ele também não inicia exposição pública automaticamente.

## Etapa 9 — storefront Editorial Luxury

- A marca pública passa a ser Lumé Studio. A mudança é de apresentação; módulos, banco, APIs e referências de pedido permanecem estáveis.
- Cormorant Garamond atende títulos editoriais e Inter atende navegação, formulários e dados transacionais. Ambas usam `next/font`.
- O header é um Client Component isolado por depender de rolagem, rota e menu; as páginas continuam Server Components sempre que possível.
- Na homepage o header começa transparente sobre o hero e ganha superfície translúcida após rolagem. Nas páginas internas nasce sólido para preservar contraste.
- O menu mobile bloqueia rolagem, fecha com Escape, restaura foco e mantém foco contido enquanto aberto.
- Coleções são construídas exclusivamente com categorias retornadas pela API. O mosaico local é tratamento visual e não cria entidades fictícias.
- Carrinho, checkout, frete, PIX, cartão e área do cliente recebem apenas estilos; seus contratos e regras permanecem inalterados.
- O painel administrativo conserva o tema Dark Tech, mantendo separação contextual entre operação e storefront.

## Etapa 9.3.1 — catálogo digital interativo

- `/catalogo` deriva suas edições diretamente das categorias públicas; `/catalogo/[slug]` pagina toda a consulta de produtos ativos e comercializáveis daquela categoria. Não há entidade, migration ou cópia comercial específica de catálogo.
- O Server Component carrega conteúdo e metadados Open Graph. O Client Component fica restrito à navegação, teclado e compartilhamento; preço permanece em centavos até a formatação.
- A virada usa CSS 3D e APIs nativas, sem dependência de flipbook. Em telas menores que 900 px o documento vira uma leitura vertical integral; `prefers-reduced-motion` elimina a animação sem ocultar conteúdo.
- URLs de compartilhamento usam a localização atual no navegador. `SITE_URL` define a base canônica dos metadados no servidor e deve apontar para o domínio público em produção.

## Etapa 9.3.2 — gestão editorial de catálogos

- `editorial_catalogs` armazena somente título, slug, descrição, chamadas editoriais, status, capa referenciada e controle de versão. `editorial_catalog_products` guarda apenas `product_id` e posição; preço, nome, estoque, imagens e disponibilidade continuam no catálogo comercial.
- A capa seleciona um registro existente de `product_images`. Upload editorial novo permanece fora do escopo até existir um fluxo de imagens de produto administrável.
- Publicação exige catálogo não vazio e revalida todos os produtos pelas mesmas regras públicas: produto, categoria, variante, material e cor ativos, além de estoque disponível quando controlado.
- `version` implementa concorrência otimista. Metadados, reordenação e publicação com versão obsoleta retornam conflito, impedindo sobrescrita silenciosa.
- Rascunhos existem apenas sob `/api/v1/admin/catalogs`; a API pública filtra `status=published` e remove produtos que deixaram de ser comercializáveis.
- Todas as mutações exigem sessão administrativa com role `admin` e token CSRF. Criação, edição, produtos, publicação e despublicação geram eventos em `admin_audit_events`.
- A interface administrativa usa substituição ordenada transacional para salvar a composição completa; endpoints explícitos de adicionar e remover também são oferecidos para outros clientes.

## Etapa 9.3.1.1 — Luxury Digital Showroom

- O catálogo público deixa de usar o leitor em formato flipbook e adota uma narrativa vertical contínua. O leitor anterior permanece disponível apenas para a prévia administrativa, evitando regressão no painel.
- `/catalogo` compõe hero, coleções, seleção da edição mais recente, destaques e categorias exclusivamente a partir dos catálogos publicados e de seus produtos comerciais. Não há mocks nem cópia de preço, estoque ou conteúdo de produto.
- `/catalogo/[slug]` alterna proporções e posicionamentos com base na ordem editorial persistida. A variação é de apresentação; URLs, slugs e dados comerciais continuam sendo os mesmos da API.
- Interações de revelação ficam em um Client Component isolado com `IntersectionObserver`. Conteúdo, metadados Open Graph e composição permanecem em Server Components; `prefers-reduced-motion` remove animações e rolagem suave.
- O showroom usa uma folha de estilos restrita ao segmento `/catalogo`, com tokens próprios de ivory, sand, taupe, graphite e white. O Design System global, checkout e painel administrativo não são alterados.
- Produtos repetidos entre edições são deduplicados apenas para as seleções visuais da home. Isso não cria coleção, preço ou disponibilidade paralelos.

## Etapa 10.2 — schema e domínio de produção 3D

- `production_jobs` representa a obrigação produtiva ligada ao item do pedido; `production_runs` representa cada tentativa física. Reimpressões são novas execuções ligadas por `reprint_of_run_id`, preservando métricas e falhas anteriores.
- As colunas legadas de impressora, tempo e material permanecem temporariamente em `production_jobs`. A migration 20 copia seus valores para a primeira execução e cria um evento de migração, sem apagar histórico.
- Um job só entra em `queued` pela função transacional `transition_production_job`. O trigger bloqueia atualizações diretas, valida concorrência otimista por `version` e, sob os mesmos locks, exige pedido confirmado/em produção, pagamento `paid`, valor integral, moeda idêntica e reembolso zero. `partially_refunded` nunca libera produção.
- O status `in_production` é aceito apenas para permitir reimpressões de um pedido cujo primeiro ciclo já começou; estados cancelado, enviado ou entregue permanecem bloqueados.
- `production_events` é append-only. Toda transição executada pelas funções oficiais preserva estado anterior, novo estado, horário, administrador e descrição. Eventos de migração podem não possuir administrador por serem ações do sistema.
- `material_spools` mantém saldo e custo por quilograma. Inserir um consumo bloqueia a bobina, rejeita saldo insuficiente, debita consumo e desperdício atomicamente e grava o custo vigente como snapshot imutável. Uma execução pode usar várias bobinas.
- Uma inspeção só pode ser registrada durante `quality_check`. Concluir uma execução exige inspeção aprovada; concluir o job exige pelo menos uma execução concluída e aprovada. Reimpressões exigem execução anterior falha ou cuja inspeção mais recente tenha sido rejeitada.
- Impressoras começam com cadastro manual e tecnologia FDM. A estrutura contempla dimensões úteis e manutenção, sem integração automática com Bambu Lab nesta etapa.
- Os papéis `production_operator` e `production_manager` são adicionados sem remover `admin` ou `quote_viewer`. O down converte esses papéis para `admin` antes de restaurar a constraint antiga, permitindo reversão determinística.
- O domínio `internal/domain/production` não depende de HTTP, PostgreSQL ou bibliotecas externas. Ele replica as regras centrais para falhar cedo, enquanto o PostgreSQL continua sendo a última barreira transacional.

