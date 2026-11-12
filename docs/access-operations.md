# Operacionalização de acessos

## Primeiro administrador

Com as migrations aplicadas e a partir de `backend`:

```powershell
$env:DATABASE_URL = "postgres://..."
$env:ADMIN_BOOTSTRAP_ENABLED = "true"
go run ./cmd/admin bootstrap
$env:ADMIN_BOOTSTRAP_ENABLED = "false"
```

E-mail, nome e senha são solicitados interativamente. A senha não aparece no terminal nem é aceita por argumento ou variável. O banco usa um advisory lock e exige `admin_users` vazio, impedindo concorrência e reutilização para elevar privilégios.

## Gestão interna

As rotas abaixo exigem cookie `loja3d_admin_session`; mutações também exigem `X-CSRF-Token`. Apenas a role `admin` é aceita.

- `GET /api/v1/admin/users`
- `POST /api/v1/admin/users`
- `PATCH /api/v1/admin/users/{id}`
- `PUT /api/v1/admin/users/{id}/password`
- `POST /api/v1/admin/users/{id}/sessions/revoke`

Roles válidas: `admin`, `quote_viewer`, `production_operator` e `production_manager`. Não é permitido ao administrador remover a própria role ou bloquear a própria conta. Bloqueio e reset de senha revogam sessões; todas as operações geram `admin_audit_events`.

## Clientes

O cliente solicita um magic link em `/minha-conta`. A resposta pública é sempre genérica. Tokens têm 256 bits, somente o SHA-256 é armazenado, expiram em 15 minutos e são consumidos atomicamente uma única vez. Há limites por IP e por e-mail. A verificação cria sessão revogável em cookie HTTP-only, SameSite Lax e Secure em produção.

Orçamentos anteriores são associados somente após comprovação do e-mail pelo magic link. Queries de orçamentos, pedidos e endereços sempre filtram pelo `user_id` da sessão. Logout e decisões mutáveis exigem CSRF.

## E-mail transacional

`Mailer` desacopla aplicação e provedor. O adaptador `development` não envia mensagens e, por segurança, não registra destinatário, token ou link. Ele é bloqueado em produção.

Para liberar acesso real deve ser implementado um adaptador de provedor com:

- domínio remetente verificado, SPF, DKIM e DMARC;
- `EMAIL_PROVIDER`, `EMAIL_FROM`, `EMAIL_REPLY_TO` e segredo específico do provedor;
- ambientes/sandboxes separados e credenciais fora do repositório;
- templates versionados para magic link, proposta e decisão;
- timeout, idempotência, classificação de falhas transitórias e retentativas assíncronas;
- métricas sem destinatário ou token e tratamento de bounce/complaint.

Até esse adaptador existir, magic link não deve ser considerado operacional em produção.
# Recuperação assistida de identidade

Não existe rota pública, senha mestra ou token universal de recuperação.

## Cliente sem todos os fatores

1. O atendimento abre incidente sem solicitar senha, TOTP ou código completo.
2. Confirmar identidade com evidências independentes já existentes, evitando usar somente o e-mail. Dados de pedidos não devem ser revelados para ajudar a resposta.
3. Um administrador autorizado revisa a evidência e registra a decisão na auditoria.
4. Aplicar período de segurança e notificar os canais previamente verificados.
5. Em implementação futura, emitir uma autorização curta e específica para reset, revogar todas as sessões e exigir nova senha/MFA. A Etapa 7.5.2 não executa recuperações reais.

## Administrador sem todos os fatores

Exigir dois responsáveis, evidência independente e acesso administrativo ainda autenticado. Revogar sessões e códigos anteriores, registrar incidente e notificar os demais administradores. E-mail isolado nunca é suficiente.

Para o último administrador, a recuperação é offline e excepcional: acesso privilegiado ao host e PostgreSQL, backup confirmado, janela de manutenção, aprovação de dois responsáveis e registro imutável do incidente. Um comando operacional futuro pode remover exclusivamente o fator do ID aprovado e revogar suas sessões; não se deve editar hashes ou criar um bypass permanente. Depois, o login normal força nova configuração TOTP.
