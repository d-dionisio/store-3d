# Hardening e homologação da autenticação

## Modelo de ameaças

Os controles consideram credential stuffing, enumeração de contas, brute force concorrente, replay de TOTP/código de recuperação, sequestro de sessão, CSRF, downgrade de MFA, vazamento de chave do banco, abuso de recuperação e confusão entre as superfícies pública e administrativa. Cloudflare Access é uma camada adicional; sessão interna, MFA, RBAC e CSRF continuam obrigatórios.

## Reautenticação

`POST /api/v1/{admin|customer}/session/reauthenticate` exige a senha atual e, quando o fator existe, TOTP ou código de recuperação. A evidência retornada:

- é armazenada somente como SHA-256;
- pertence à sessão e ao tipo de identidade;
- expira em `AUTH_REAUTH_TTL` (padrão 5 minutos, máximo 15);
- é consumida uma única vez pelo header `X-Reauthentication-Token`;
- não usa horário fornecido pelo navegador.

Desativar MFA, regenerar recuperação, trocar senha e revogar sessões exigem essa evidência além de sessão e CSRF. Troca de senha e desativação de MFA revogam sessões. Operações administrativas de credenciais devem adotar o mesmo middleware antes da liberação operacional.

## Rotação da chave TOTP

1. Gere 32 bytes aleatórios fora do repositório e armazene a nova chave no secret manager.
2. Mantenha a chave anterior em `AUTH_MFA_DECRYPTION_KEYS=1:<antiga>` e configure a nova em `AUTH_MFA_ENCRYPTION_KEY`, incrementando `AUTH_MFA_KEY_VERSION`.
3. Reinicie uma instância e valide login TOTP antigo e criação de fator novo.
4. Execute `go run ./cmd/mfa-key status`.
5. Execute `go run ./cmd/mfa-key rotate`. O comando bloqueia os fatores, decifra pela versão registrada e recifra transacionalmente com a chave ativa.
6. Execute novamente `status`; a versão antiga deve ter zero dependências.
7. Valide MFA e somente então retire a chave antiga. Durante rollback, restaure a versão ativa e mantenha ambas as chaves autorizadas.

O comando não imprime plaintext nem chaves. Ciphertext adulterado ou versão ausente aborta toda a transação.

## Senhas comprometidas

`PASSWORD_BREACH_CHECK_URL` habilita consulta k-anônima: apenas os cinco primeiros caracteres do SHA-1 são enviados, com padding solicitado. Cadastro, redefinição e alteração consultam o serviço. Testes usam servidor HTTP local, nunca a internet. `PASSWORD_BREACH_FAILURE_MODE=allow` mantém disponibilidade se o provedor falhar; `deny` é fail-closed. O modo `allow` deve ser acompanhado por alerta operacional e cache futuro de prefixos, nunca de senhas.

## Rate limiting compartilhado

`auth_rate_limits` mantém contadores atômicos no PostgreSQL para login, MFA, cadastro, recuperação e verificação. A chave é SHA-256 do sujeito; IP/e-mail não são armazenados em claro. A API usa `RemoteAddr` e não confia em `X-Forwarded-For`. Falha no PostgreSQL fecha os novos fluxos de autenticação. O limitador local permanece como defesa rápida por processo.

## Sessões e rollback

Cookies são host-only, HTTP-only, `Secure` em produção e separados por nome. Admin usa SameSite Strict; cliente, Lax. Desafios não são sessões. MFA eleva diretamente para nova sessão e não atualiza uma sessão de senha administrativa.

Rollback da migration 24 exige interromper temporariamente as ações sensíveis e executar `go run ./cmd/migrate down 1`. Isso remove somente evidências e contadores; fatores, senhas e sessões da migration 23 permanecem. Não faça rollback enquanto uma versão de API que exige essas tabelas estiver atendendo tráfego.

## Pendências para produção

- homologação visual real nas quatro resoluções;
- provedor de e-mail e alertas de segurança;
- reautenticação nas telas administrativas de gestão de usuários;
- cache compartilhado da consulta de senhas comprometidas;
- job periódico de limpeza de contadores/evidências expirados;
- testes de carga e alertas de bloqueios anormais.

