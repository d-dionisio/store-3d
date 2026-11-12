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
RESEND_WEBHOOK_SECRET=
```

API keys, chave da outbox e segredo do webhook pertencem ao secret manager. Nenhuma variável é `NEXT_PUBLIC_*`. Produção exige URL pública HTTPS e remetente válido. `EMAIL_DELIVERY_ENABLED=false` impede iniciar o worker externo, mas permite validar a criação da outbox.

## Templates

Há HTML responsivo e texto simples para verificação, redefinição, magic link, alteração de senha/MFA, evento de segurança, pedido confirmado/atualizado e proposta/decisão. `html/template` escapa conteúdo dinâmico. Mensagens não contêm senha, segredo TOTP, código de recuperação ou dados de cartão, e não usam pixel de rastreamento.

## Confidencialidade e retenção

Destinatário, link e dados do template ficam em um payload AES-256-GCM. A tabela expõe somente SHA-256 do destinatário, tipo e metadados operacionais. A chave da outbox é separada da chave MFA. Links vencidos mudam para `expired` e nunca são enviados. A migration inicial suporta uma versão de chave; rotação deve manter a versão antiga disponível até recifrar os itens pendentes.

## Worker, retries e idempotência

Execute somente depois da ativação deliberada:

```powershell
cd backend
go run ./cmd/email-worker
```

O worker usa `FOR UPDATE SKIP LOCKED`, recupera locks abandonados após cinco minutos, timeout por chamada, shutdown por contexto, oito tentativas, backoff exponencial com jitter e falha terminal. Um envio por segundo e limite PostgreSQL de 100 entregas diárias protegem a franquia gratuita.

O header `Idempotency-Key` deriva da intenção local e permanece estável nos retries. O Resend retém a idempotência por 24 horas; portanto o sistema oferece entrega pelo menos uma vez com deduplicação local e do provedor, não exatamente uma vez.

## Webhook

`POST /api/v1/webhooks/resend` lê o corpo bruto e valida `svix-id`, `svix-timestamp` e `svix-signature`, com tolerância de cinco minutos. Eventos são deduplicados pelo ID. `delivered`, `bounced`, `complained` e `delivery_delayed` atualizam a outbox. Bounce e complaint criam supressão por hash.

A rota existe internamente, mas não foi publicada. O proxy Mercado Pago não deve ser reutilizado; uma allowlist separada será necessária na homologação externa.

## Eventos integrados

- verificação, reenvio, redefinição e magic link usam os tokens atuais;
- alteração de senha/MFA gera alerta sem incluir credenciais;
- proposta enviada cria a outbox dentro da transação da proposta;
- pagamento reconciliado como `paid` cria `order_confirmed` dentro da transação financeira e é deduplicado pelo evento comercial;
- decisão da proposta utiliza o adaptador de outbox existente.

Na implementação atual, a criação inicial de tokens de autenticação e o enqueue são duas transações consecutivas. Isso é fail-safe — não libera conta nem expõe token e permite reenvio — mas ainda existe uma janela de não entrega entre commits. Unificar essas operações na mesma unidade transacional permanece pendência antes da ativação de produção.

## Domínio e segurança

Antes de produção, verifique o domínio no Resend e publique SPF/DKIM fornecidos pelo provedor. Configure DMARC inicialmente em monitoramento e evolua após observar alinhamento. Não altere DNS automaticamente. Use remetente autenticado, trate bounce/complaint e respeite LGPD com minimização, retenção curta e processo de eliminação.

## Rollback e homologação

Com o worker parado, `go run ./cmd/migrate down 1` remove somente outbox, eventos e supressões. A API deve voltar ao adaptador indisponível antes do rollback. Restaure `backups/pre_transactional_email_20261112.dump` apenas em desastre, nunca como rollback rotineiro.

Não houve envio real, criação de conta Resend, DNS ou túnel. Homologação posterior deve usar domínio/verificador de teste autorizado, conferir assinatura real e separar evidência simulada de entrega externa.

