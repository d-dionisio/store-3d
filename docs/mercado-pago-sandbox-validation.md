# Validação do Mercado Pago Sandbox

Estado em 11 de novembro de 2026: **client, PostgreSQL e reconciliação assinada validados; entrega externa do webhook pendente**.

Este documento separa evidências locais de chamadas reais ao Mercado Pago. Nenhuma
credencial, assinatura, QR Code ou código PIX deve ser copiado para este arquivo.

## Configuração esperada

Somente credenciais do ambiente de teste são aceitas nesta validação:

```text
MERCADO_PAGO_ENVIRONMENT=sandbox
MERCADO_PAGO_SANDBOX_ACCESS_TOKEN=<access token de teste>
MERCADO_PAGO_SANDBOX_WEBHOOK_SECRET=<assinatura do webhook de teste>
MERCADO_PAGO_TIMEOUT=8s
MERCADO_PAGO_PIX_TTL=30m
```

As variáveis devem existir apenas no ambiente local não versionado. O Access Token
de produção não deve ser configurado durante esta etapa.

## Evidências executadas

### Testes unitários e HTTP mock

Executados com `go test ./... -count=1`:

- criação PIX em `POST /v1/orders`;
- header Bearer e `X-Idempotency-Key`;
- parsing de QR Code, código Copia e Cola, identificador e expiração;
- consulta autenticada em `GET /v1/orders/{id}`;
- timeout do provedor;
- HMAC válido e inválido em tempo constante;
- tentativa existente reutilizada sem nova chamada;
- divergência de referência/valor rejeitada;
- webhook duplicado não reconciliado novamente.

Resultado registrado: suíte Go aprovada e `go vet ./...` aprovado.

### PostgreSQL local

Com `TEST_DATABASE_URL` apontando para o PostgreSQL local, passaram testes de:

- prontidão e propriedade do pedido;
- reserva concorrente com uma única obrigação e tentativa;
- persistência dos artefatos PIX;
- deduplicação do webhook;
- confirmação transacional de pagamento e pedido;
- evento fora de ordem sem regressão;
- reembolso idempotente;
- divergência de valor com rollback.

O acesso ao engine foi liberado fora do sandbox local, o container
PostgreSQL 17 ficou `healthy` e a versão foi confirmada como `16, dirty: false`.
O teste opt-in ponta a ponta criou um pedido demonstrativo de R$ 10,01, persistiu a
order e os artefatos PIX, validou HMAC, consultou a order real, reconciliou o webhook
e deduplicou uma segunda entrega. A fixture foi removida ao final.

Uma tentativa de alterar o fixture para R$ 50,00 foi rejeitada pela constraint que
exige subtotal igual ao valor da proposta aprovada. O teste final preservou o snapshot
comercial, sem contornar a integridade.

### Sandbox real

Executado contra a API oficial:

- `GET /users/me`: HTTP 200, conta brasileira (`MLB`), confirmando que o token é
  reconhecido pelo provedor;
- `POST /v1/orders`: HTTP 201 no cenário oficial PIX `APRO` de R$ 50,00;
- order e pagamento retornaram `action_required / waiting_transfer`;
- QR Code e ticket PIX estiveram presentes, sem que seus conteúdos fossem registrados;
- `GET /v1/orders/{id}`: HTTP 200, com referência externa correspondente;
- o client da aplicação foi migrado para `POST /v1/orders` e seu teste opt-in real passou, normalizando o resultado para `pending/BRL`, confirmando os artefatos PIX e verificando que a repetição da mesma chave idempotente retorna a mesma order;
- a entrega originada pelo painel do Mercado Pago através do túnel ngrok ainda não
  foi observada; a mesma validação HMAC e reconciliação foi executada pelo teste
  integrado usando consulta real à Orders API.

A credencial estava correta. O diagnóstico anterior de credencial live foi descartado
após a evidência do painel e o ensaio oficial. A incompatibilidade era o contrato da
Payments API legada; o adaptador agora usa a Orders API.

## Auditoria do contrato

- O client utiliza `https://api.mercadopago.com/v1/orders` para criar e consultar
  orders de pagamento.
- A criação envia `X-Idempotency-Key` e credencial Bearer somente em header.
- O webhook usa `x-signature`, `x-request-id` e `data.id`, calcula HMAC-SHA256 e
  compara em tempo constante.
- O payload do webhook não confirma pedidos diretamente: ele dispara uma consulta
  autenticada ao provedor.
- Moeda, valor e referência externa são comparados com o snapshot do pedido.
- `provider_updated_at` bloqueia regressões por notificações fora de ordem.
- O frontend apenas consulta o estado persistido; ele não confirma pagamento.

## Roteiro do ensaio real

1. Iniciar o PostgreSQL e confirmar `version: 16, dirty: false`.
2. Configurar exclusivamente o Access Token e o segredo de webhook de teste.
3. Expor temporariamente o endpoint de webhook por HTTPS e cadastrar a URL no
   ambiente **Teste** da aplicação.
4. Criar um cliente e pedido demonstrativos, endereço e frete, deixando
   `payment_readiness=ready`.
5. Criar o PIX pelo endpoint autenticado da Loja 3D.
6. Registrar somente metadados não sensíveis: status HTTP, ID mascarado, status,
   moeda, presença dos artefatos e horários.
7. Consultar o pagamento pelo provedor e comparar referência, valor e moeda.
8. Simular/receber webhook pelo painel de teste e confirmar assinatura,
   deduplicação, consulta autenticada e atualização transacional.
9. Repetir com assinatura inválida e confirmar HTTP 401.
10. Remover o túnel, revogar sessões demonstrativas e excluir dados de teste quando
    a política do provedor permitir.

## Critério de conclusão

A validação técnica do backend está concluída. Para fechar a validação operacional,
uma notificação deve ser entregue pelo Mercado Pago através da URL pública HTTPS e
aparecer no monitoramento do painel com HTTP 200.
