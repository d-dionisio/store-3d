# Validação de cartão no Mercado Pago Sandbox

Data: 11 de novembro de 2026.

Este relatório não contém PAN, CVV, token de cartão, documento, identificadores do provedor, URLs de autenticação ou credenciais. Somente cartões oficiais e credenciais de teste foram utilizados.

## Evidências separadas

### Testes unitários e HTTP mock

- Payload da Orders API com cartão, parcelas, captura automática e configuração 3DS.
- Mapeamento de `action_required/pending_challenge`.
- Validação da URL HTTPS de autenticação.
- Timeout e indisponibilidade.
- Webhook inválido, duplicado e fora de ordem.
- Divergência de valor, moeda e referência.
- HTTP 402 com Order recusada no nó `data`.

### PostgreSQL

- Reserva e conclusão da tentativa de cartão.
- Token e documento não persistidos.
- Idempotência por tentativa.
- Bloqueio de obrigação PIX e cartão simultâneas.
- Confirmação do pedido somente após reconciliação.
- Deduplicação do evento financeiro.

### Sandbox real

| Cenário oficial | Resultado observado |
| --- | --- |
| `APRO` | `approved / accredited`, BRL, sem Challenge |
| `OTHE` | `rejected / failed`, resposta HTTP 402 corretamente mapeada |
| `CONT` | `rejected / failed`; não reproduziu o estado pendente descrito na tabela geral |
| `APRO-CHOK` | `approved / accredited`, sem Challenge retornado |
| `OTHE-CHNO` | `rejected / failed`, sem Challenge retornado |

### Diagnóstico 3DS

O request implementado coincide com o contrato atual da Orders API: `processing_mode=automatic`, `capture_mode=automatic`, `config.online.transaction_security.validation=on_fraud_risk` e `liability_shift=required`. Cartão, parcelas e token seguem dentro de `transactions.payments[].payment_method`.

O client também lê a URL de `transactions.payments[].payment_method.transaction_security.url`. Uma resposta `action_required/pending_challenge` é mapeada como pendente, devolvida ao Card Payment Brick e nunca confirma o pedido. O frontend abre somente URL HTTPS validada do Mercado Pago, aguarda o retorno e reconcilia o estado persistido; ele não converte o retorno do iframe diretamente em aprovação.

Apesar disso, o sandbox observado não seguiu a tabela oficial de cenários: `APRO-CHOK` foi aprovado sem `action_required` e `OTHE-CHNO` foi recusado sem URL de Challenge. Portanto, sucesso, abandono e recusa dentro do Challenge **não foram validados**. Não foi aplicada alteração especulativa ao payload.

Uma repetição em 11 de novembro de 2026, usando novamente o cartão e o cenário oficiais, falhou antes da criação da Order: o endpoint de tokenização retornou HTTP 400, `unexpected_processing`/`G001`. Nenhum token ou dado de cartão foi persistido. A tentativa com ano em dois dígitos confirmou que a API exige ano com quatro dígitos (`E205` para o formato alternativo), mas não resolveu `G001`. Esta execução não substitui as evidências reais anteriores e adiciona uma segunda divergência a ser encaminhada ao suporte.

Para abertura de chamado ao Mercado Pago, registrar: aplicação e país sem credenciais, data/hora com fuso, ambiente de teste, cenário/titular usado, HTTP status, `status/status_detail`, presença ou ausência de `transaction_security.url`, e IDs truncados da Order. Solicitar confirmação de pré-requisitos adicionais da conta de teste e da disponibilidade do simulador 3DS para Orders API no Brasil.

Public Key e Access Token foram aceitos no mesmo fluxo: o token de cartão gerado com a Public Key foi consumido com sucesso por Orders autenticadas com o Access Token. Valores e moeda retornaram iguais à requisição.

Um teste integrado adicional criou uma Order real aprovada, persistiu a tentativa no PostgreSQL, repetiu a operação sem criar nova tentativa, bloqueou PIX concorrente e reconciliou a Order por consulta autenticada. O pedido passou a `confirmed` somente durante a reconciliação.

### Webhook

A validação HMAC, a persistência, a consulta autenticada, a deduplicação e a atualização transacional foram executadas com gatilho assinado localmente. Isso não é evidência de entrega real pela internet.

Em 11 de novembro de 2026, o Mercado Pago tentou entregar pela internet uma notificação sandbox real `order.processed` para a URL HTTPS configurada. A borda respondeu HTTP 502. No diagnóstico posterior, a API respondia HTTP 200 em `127.0.0.1:8080`, mas não existia listener em `127.0.0.1:8090` nem interface local do ngrok em `127.0.0.1:4040`. Portanto, essa evidência comprova a tentativa externa até a URL pública, mas **não** comprova chegada ao proxy, validação HMAC, consulta autenticada, deduplicação ou atualização do pedido.

Foi criado um proxy local de allowlist que expõe somente o POST do webhook, documentado em `docs/mercado-pago-webhook-proxy.md`. Uma nova entrega precisa ser executada com API, proxy e túnel ativos simultaneamente.

### Navegador

O Card Payment Brick compila com a Public Key de teste configurada, mas preenchimento manual, Secure Fields e interação com iframe 3DS não foram executados nesta sessão. Como o provedor não retornou Challenge nos cenários `APRO-CHOK` e `OTHE-CHNO`, sucesso, abandono, falha e retorno do iframe permanecem sem evidência real.

## Correção encontrada

A Orders API retorna recusas comerciais com HTTP 402 e a Order completa dentro de `data`. O client descartava esse corpo e classificava a recusa como indisponibilidade. O parser agora preserva a Order recusada e registra o estado financeiro correto.

## Pendências para produção

- Obter Challenge real no sandbox e validar sucesso, abandono, falha e retorno do 3DS no navegador.
- Validar entrega real de webhook por endpoint público com exposição mínima.
- Confirmar no painel do Mercado Pago que o tópico configurado é `order` e que a URL corresponde ao ambiente.
- Executar os testes com usuário comprador oficial de teste e verificar se isso altera a emissão do Challenge.
- Definir monitoramento, alertas e política operacional de reprocessamento antes da ativação de produção.
