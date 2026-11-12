# E-mail transacional

## Arquitetura

Os serviços de identidade e cliente continuam gerando os tokens existentes. O adaptador `mail.Outbox` apenas registra a intenção de entrega; ele não cria nem renova tokens. O fluxo é:

`caso de uso → outbox PostgreSQL → worker → EmailSender → Resend`

Templates, armazenamento, despacho e provedor são componentes separados. `EmailSender` recebe uma mensagem já renderizada, portanto outro provedor pode substituí-lo sem alterar autenticação, pedidos ou propostas.

## Configuração

```env
EMAIL_PROVIDER=resend
EMAIL_DELIVERY_ENABLED=false
RESEND_API_KEY=
EMAIL_FROM=Lumé Studio <no-reply@example.com>
EMAIL_REPLY_TO=support@example.com
EMAIL_PUBLIC_BASE_URL=https://example.com
EMAIL_OUTBOX_ENCRYPTION_KEY=<32 bytes aleatórios em Base64>
EMAIL_OUTBOX_KEY_VERSION=1
EMAIL_OUTBOX_DECRYPTION_KEYS=
EMAIL_MESSAGE_RETENTION=720h
EMAIL_EVENT_RETENTION=2160h
RESEND_WEBHOOK_SECRET=
```

API keys, chave da outbox e segredo do webhook pertencem ao secret manager. Nenhuma variável é `NEXT_PUBLIC_*`. Produção exige URL pública HTTPS e remetente válido. `EMAIL_DELIVERY_ENABLED=false` impede iniciar o worker externo, mas permite validar a criação da outbox.

## Templates

Há HTML responsivo e texto simples para verificação, redefinição, magic link, alteração de senha/MFA, evento de segurança, pedido confirmado/atualizado e proposta/decisão. `html/template` escapa conteúdo dinâmico. Mensagens não contêm senha, segredo TOTP, código de recuperação ou dados de cartão, e não usam pixel de rastreamento.

## Confidencialidade e retenção

Destinatário, link e dados do template ficam em um payload AES-256-GCM, com nonce aleatório e AAD contendo a versão. A tabela expõe somente SHA-256 do destinatário, tipo e metadados operacionais. A chave da outbox é independente da chave MFA. Links vencidos mudam para `expired` antes do claim e nunca são enviados.

O keyring lê a chave ativa e versões antigas listadas em `EMAIL_OUTBOX_DECRYPTION_KEYS` no formato `1:<base64>,2:<base64>`. Rotação operacional: gerar uma nova chave fora do repositório, manter a versão anterior no keyring, configurar a nova versão ativa, parar o worker, executar `go run ./cmd/email-key status`, depois `go run ./cmd/email-key rotate`, conferir que nenhum payload depende da versão antiga, reiniciar os processos e somente então retirar a chave antiga. O comando recifra todas as mensagens em uma única transação e não deve ser executado sem autorização e backup. Chaves MFA e outbox nunca são reutilizadas.

## Worker, retries e idempotência

Execute somente depois da ativação deliberada:

```powershell
cd backend
go run ./cmd/email-worker
```

O worker usa `FOR UPDATE SKIP LOCKED`, recupera locks abandonados após cinco minutos, timeout por chamada, shutdown por contexto, oito tentativas, backoff exponencial com jitter e falha terminal. Uma trava consultiva serializa a reserva da quota diária entre workers. Mensagens suprimidas terminam sem envio. A cada hora, mensagens terminais e eventos antigos são eliminados conforme `EMAIL_MESSAGE_RETENTION` e `EMAIL_EVENT_RETENTION`; supressões não são removidas automaticamente. Essa limpeza alcança apenas tabelas de e-mail e nunca auditoria comercial ou financeira.

O header `Idempotency-Key` deriva da intenção local e permanece estável nos retries. O Resend retém a idempotência por 24 horas; portanto o sistema oferece entrega pelo menos uma vez com deduplicação local e do provedor, não exatamente uma vez.

## Webhook

`POST /api/v1/webhooks/resend` lê o corpo bruto e valida `svix-id`, `svix-timestamp` e `svix-signature`, com tolerância de cinco minutos. Eventos são deduplicados pelo ID. `delivered`, `bounced`, `complained` e `delivery_delayed` atualizam a outbox. Bounce e complaint criam supressão por hash.

A rota existe internamente, mas não foi publicada. O ingresso futuro deve terminar HTTPS em proxy dedicado, aceitar exclusivamente `POST /api/v1/webhooks/resend`, preservar `svix-id`, `svix-timestamp` e `svix-signature`, limitar corpo e taxa, bloquear todas as demais rotas e não registrar corpo ou segredos. O proxy Mercado Pago não deve ser reutilizado.

## Eventos integrados

- verificação inicial, reenvio, redefinição e magic link persistem token e intenção de envio na mesma transação PostgreSQL;
- alteração de senha/MFA gera alerta sem incluir credenciais;
- proposta enviada cria a outbox dentro da transação da proposta;
- pagamento reconciliado como `paid` cria `order_confirmed` dentro da transação financeira e é deduplicado pelo evento comercial;
- decisão da proposta utiliza o adaptador de outbox existente.

Se o enqueue falhar, a transação remove o novo token e restaura implicitamente o token anterior. Reenvios concorrentes são serializados por identidade ou e-mail; a intenção anterior ainda pendente é encerrada como `superseded` antes da nova intenção. Nenhuma mensagem é entregue antes do commit: somente o worker lê registros confirmados.

## Domínio e segurança

Antes de produção, verifique o domínio no Resend e publique SPF/DKIM fornecidos pelo provedor. Configure DMARC inicialmente em monitoramento e evolua após observar alinhamento. Não altere DNS automaticamente. Use remetente autenticado, trate bounce/complaint e respeite LGPD com minimização, retenção curta e processo de eliminação.

## Rollback e homologação

Com o worker parado, `go run ./cmd/migrate down 1` remove somente outbox, eventos e supressões. A API deve voltar ao adaptador indisponível antes do rollback. Restaure `backups/pre_transactional_email_20261112.dump` apenas em desastre, nunca como rollback rotineiro.

Não houve envio real, criação de conta Resend, DNS ou túnel. Homologação posterior deve usar domínio/verificador de teste autorizado, conferir assinatura real e separar evidência simulada de entrega externa.

