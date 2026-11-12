# Implantação do perímetro administrativo

Este documento é um procedimento de preparação. Ele não cria conta, DNS, Tunnel nem exposição pública automaticamente.

## Processos locais

Use endereços de loopback:

1. PostgreSQL: `docker compose up -d postgres` (porta vinculada a `127.0.0.1`).
2. API Go: `go run ./cmd/api` com `HTTP_HOST=127.0.0.1`.
3. Next.js: execute no frontend e mantenha a origem acessível apenas localmente.
4. Gateway: `go run ./cmd/access-gateway`.
5. Webhook restrito, somente quando necessário: `go run ./cmd/webhook-proxy`.

Em desenvolvimento, gere um valor aleatório com pelo menos 24 caracteres para `ZERO_TRUST_LOCAL_TOKEN` e envie-o apenas ao gateway admin local. Não o reutilize ou configure em produção. No deploy dividido, deixe `NEXT_PUBLIC_API_URL` vazio; o navegador usa o próprio hostname, enquanto `API_URL=http://127.0.0.1:8080` atende Server Components.

## Cloudflare Tunnel

1. Copie `infra/cloudflared/config.yml.example` para uma localização operacional fora do repositório.
2. Substitua UUID, caminho de credenciais e os três hostnames.
3. Mantenha a regra final `http_status:404`.
4. Preencha `teamName` e o AUD tag da aplicação Access no bloco `originRequest.access`; `required: true` obriga `cloudflared` a validar o JWT antes de alcançar o gateway.
5. Valide com `cloudflared tunnel ingress validate`.
6. Teste o roteamento de cada URL com `cloudflared tunnel ingress rule <URL>`.
7. Crie DNS somente após revisar as regras. Não publique 3000, 5432, 8080, 8081, 8082 ou 8090 no roteador/firewall.

O hostname de webhook é separado e sua regra aceita somente a rota exata. O proxy local ainda valida método, tamanho, rate limit e assinatura no backend. Nunca coloque o webhook atrás da sessão administrativa.

## Cloudflare Access para o admin

Crie uma aplicação **Self-hosted** cobrindo todo `admin.example.com/*`:

- política default-deny;
- regra Allow somente para os e-mails administrativos autorizados (não use domínio inteiro salvo decisão formal);
- MFA obrigatório no IdP;
- sessão curta, recomendação inicial de 4 horas, sujeita à política da organização;
- sem Bypass, Service Token ou `Everyone` para tráfego humano;
- cookies Secure e HTTPS obrigatório.

Após passar pelo Access, o operador ainda autentica no `/admin/login`; as roles internas continuam determinando permissões. O Access não substitui senha, sessão, CSRF, RBAC ou auditoria da aplicação.

## Revogação

Para desligar um colaborador:

1. desative a identidade no IdP e remova-a da allowlist Access;
2. revogue a sessão do usuário no Cloudflare Zero Trust;
3. desative a conta interna no painel e revogue suas sessões;
4. revise auditoria Access e aplicação;
5. rotacione credenciais somente se houver indício de comprometimento.

Para incidente amplo, use **Revoke existing tokens** na aplicação Access e revogue todas as sessões administrativas internas.

## Homologação antes de produção

- do domínio público, `/admin`, `/admin/login` e `/api/v1/admin/*` retornam 404 para GET/POST/PUT/PATCH/DELETE;
- variações codificadas e traversal também são bloqueadas;
- o domínio admin sem sessão Access não alcança a origem;
- após Access, ainda é necessário login interno; role insuficiente recebe 403;
- loja, cliente, orçamento e checkout continuam acessíveis no domínio público;
- apenas o hostname/rota de webhook chega ao proxy 8090;
- scanners externos não encontram portas de origem;
- cookies administrativos não aparecem no domínio público;
- desligar Access/Tunnel falha fechado;
- MFA interno deve ser implementado e validado antes da liberação final se a política exigir duas camadas com MFA.
