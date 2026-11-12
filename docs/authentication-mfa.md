# Autenticação por senha e MFA

## Modelo adotado

- Administradores continuam usando as contas, papéis, bcrypt, sessões, CSRF e auditoria existentes. A senha passa a ser apenas o primeiro fator: nenhuma sessão administrativa válida é emitida antes da conclusão do TOTP ou do consumo de um código de recuperação.
- Clientes podem manter o magic link ou definir uma senha. Novas senhas usam Argon2id; um hash bcrypt legado é migrado para Argon2id depois de um login válido.
- MFA é obrigatório para administradores e opcional para clientes. O TOTP usa seis dígitos, período de 30 segundos, janela de um período para cada lado e contador persistido para impedir replay.
- Segredos TOTP são cifrados com AES-256-GCM. A chave e sua versão vêm de `AUTH_MFA_ENCRYPTION_KEY` e `AUTH_MFA_KEY_VERSION`; não pertencem ao banco nem ao repositório.
- Códigos de recuperação são exibidos uma vez e somente hashes SHA-256 são armazenados. Cada código é consumido transacionalmente uma única vez.

## Política de senha

O limite é de 12 caracteres Unicode no mínimo e 1024 bytes no máximo, sem regras artificiais de composição. O backend é a autoridade da política. Cadastro, verificação, recuperação e login usam respostas genéricas onde isso evita enumeração de contas. Falhas progressivas bloqueiam temporariamente a identidade.

Parâmetros Argon2id atuais: memória 19 MiB, duas iterações, paralelismo 1, salt aleatório de 16 bytes e saída de 32 bytes. A configuração deve ser revista por benchmark ao alterar a capacidade dos servidores.

## Fluxos administrativos

1. `POST /api/v1/admin/session` valida e-mail e senha.
2. No primeiro acesso, retorna um desafio de configuração TOTP e QR gerado localmente. Nos acessos posteriores, retorna um desafio MFA.
3. `POST /api/v1/admin/session/mfa` consome o desafio e o TOTP ou código de recuperação.
4. Apenas então são emitidos o cookie HTTP-only e o token CSRF. Sessões anteriores à migration 23 são revogadas.

O bootstrap permanece restrito à instalação inicial. Ele não configura um segundo caminho de autenticação nem permite elevação depois que a primeira conta existe.

## Fluxos de clientes

- Cadastro: `POST /api/v1/customer/register`, seguido da verificação de e-mail por token de uso único.
- Senha: `POST /api/v1/customer/session`; se MFA estiver ativo, a sessão só nasce após `/api/v1/customer/session/mfa`.
- Recuperação: solicitação e resposta são genéricas; o token expira, é de uso único e redefinir a senha revoga as sessões existentes.
- Magic link: continua disponível para clientes sem MFA. Quando MFA está habilitado, ele não pode criar uma sessão que contorne o segundo fator.
- Migração: um cliente autenticado por magic link pode definir uma senha sem criar um segundo usuário; pedidos e orçamentos continuam associados ao mesmo `users.id`.

## Operação de chaves e e-mail

Gere a chave fora do repositório, por exemplo com um gerenciador de segredos capaz de produzir 32 bytes aleatórios codificados em Base64. A API falha fechada para os novos fluxos quando a chave não é válida; em produção, o startup também deve falhar. Rotação exige manter a chave anterior disponível enquanto os segredos são recifrados para uma nova `key_version`.

O adaptador de desenvolvimento não deve entregar links reais nem registrar tokens. Cadastro, verificação e recuperação só ficam operacionais em produção depois de configurar um provedor transacional, domínio remetente e templates. Falha de envio não é tratada como sucesso de entrega.

## Pendências operacionais

- seleção e configuração do provedor de e-mail transacional;
- processo assistido de recuperação quando senha, TOTP e códigos forem perdidos;
- verificação de senhas comprometidas com um serviço que preserve privacidade;
- rate limiting compartilhado para múltiplas instâncias;
- runbook e automação de rotação da chave de cifragem;
- reautenticação recente dedicada antes de desativar MFA ou regenerar códigos.

O hardening posterior implementa evidência de reautenticação de uso único, keyring e recifragem. Consulte `docs/authentication-hardening.md`; esta referência prevalece para a operação atual.

