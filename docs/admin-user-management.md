# Gestão de usuários administrativos

## Acesso

A interface fica em `/admin/usuarios` e somente é carregada depois da sessão administrativa existente, com MFA concluído. O hostname administrativo continua protegido pelo Cloudflare Access; essa camada não substitui a sessão, o RBAC ou o CSRF da aplicação.

Somente a role `admin` pode listar ou alterar contas internas. As roles permitidas continuam sendo `admin`, `quote_viewer`, `production_operator` e `production_manager`.

## Operações

- consultar, pesquisar, filtrar e paginar contas;
- criar uma conta ativa com senha temporária e role inicial;
- alterar role;
- bloquear ou desbloquear, revogando sessões ao bloquear;
- redefinir senha, revogando as sessões existentes;
- revogar todas as sessões de uma conta;
- consultar estado de MFA, último acesso e quantidade de sessões ativas.

Segredos TOTP, códigos de recuperação, hashes e tokens nunca fazem parte do contrato HTTP. Eventos detalhados de segurança não possuem endpoint de consulta nesta etapa e, por isso, não são simulados no frontend.

## Reautenticação

Toda mutação usa `POST /api/v1/admin/session/reauthenticate`. A evidência retornada permanece apenas no estado em memória do diálogo e é enviada imediatamente no header `X-Reauthentication-Token`, junto do `X-CSRF-Token` já existente. O backend vincula a evidência à sessão, aplica expiração e permite um único consumo. Falha ou nova operação exige outra reautenticação.

## Proteções

- o backend é a autoridade das roles;
- autodemissão e autobloqueio são recusados;
- uma trava transacional impede bloquear ou rebaixar o último administrador ativo;
- bloqueio e reset revogam sessões;
- todas as operações continuam em `admin_audit_events`;
- não existe cadastro público de administrador nem mecanismo paralelo de autenticação.

## Limitações

A listagem e paginação são feitas no navegador porque o endpoint existente retorna o conjunto de usuários internos. Para equipes muito maiores, o contrato poderá evoluir para paginação no PostgreSQL. Não há recuperação administrativa automática de MFA; incidentes seguem o runbook offline de `docs/access-operations.md`.
