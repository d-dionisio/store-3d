# Isolamento de senha e MFA administrativo — Etapa 10.10.2.4.4.1

## Diagnóstico

O formulário anterior era um único `<form>` com campos não controlados, alternados por ramos condicionais sem `key`. No primeiro acesso que exige configuração de MFA, o ramo MFA posiciona a imagem QR e a chave manual antes do campo TOTP. O input da senha e o input TOTP ocupavam a mesma posição reconciliada e tinham o mesmo tipo de elemento HTML (`input`); ao trocar `type=password` por `type=text`, o nó DOM podia ser reutilizado e manter seu valor. Isso explica a senha aparecer literalmente no campo de código.

O teste E2E também identificou uma condição de corrida pré-hidratação: o formulário HTML sem método explícito poderia usar submissão nativa GET antes de os handlers React estarem ativos. O formulário agora usa `method="post"` e mantém o botão desabilitado até a hidratação, evitando que credenciais sejam colocadas na query string.

## Correção

- Senha, TOTP e código de recuperação têm estados React separados.
- Cada etapa usa formulário com `key` distinta, forçando o descarte do DOM da etapa anterior.
- A senha é apagada após a resposta do login e em qualquer falha; TOTP/código de recuperação também são limpos após tentativa.
- O TOTP é `type="text"`, `name="totp"`, `inputMode="numeric"`, `autoComplete="one-time-code"` e máximo de seis caracteres.
- A senha usa `type="password"`, `name="password"` e `autoComplete="current-password"`.
- Senha, TOTP e desafio não são persistidos em localStorage ou sessionStorage. O token CSRF continua sendo armazenado após a autenticação MFA conforme o fluxo existente.
- MFA, verificação no backend, códigos de recuperação, sessão HttpOnly, CSRF e RBAC não foram alterados.

## Testes

O teste `admin-login-mfa-isolation.spec.ts` executa Chromium contra `/admin/login` e intercepta apenas as chamadas da tela com respostas fictícias. O cenário usa `mfa_setup_required`, cobrindo o ramo que causava a colisão de posição. Verifica campos vazios/atributos semânticos, transição para TOTP vazio, independência dos inputs, erro genérico de TOTP inválido, limpeza após falha, novo login após reload e ausência do valor fictício na URL. Não cria sessão, não invoca autenticação real e não acessa banco.

Resultado Playwright: **1 teste aprovado** (`npm run test:e2e:mfa-isolation`). O cenário inclui resposta fictícia de senha incorreta, limpeza do campo e nova tentativa; os primeiros ensaios também revelaram a corrida de hidratação e um seletor ambíguo do alert. A cobertura foi ajustada, a submissão pré-hidratação foi protegida e a execução final passou. Os resultados intermediários continham somente valores fictícios e foram tratados como artefatos locais descartáveis.

Regressão frontend:

- `npm run lint`: aprovado.
- `npm test -- --run`: 12 arquivos, 37 testes aprovados.
- `npx tsc --noEmit`: aprovado.
- `npm run build`: **falhou por bloqueio de rede** ao baixar Cormorant Garamond e Inter de `fonts.googleapis.com`. Foi executado em diretório de saída isolado; não alterou `.next` compartilhado nem a configuração final. É necessária conectividade de build aos Google Fonts ou auto-hospedagem de fontes, fora do escopo desta correção.
- Smoke test autenticado existente do dashboard: aprovado, 1 teste. Usou o estado Playwright local já existente, validado pelo global setup; nenhuma senha/TOTP foi digitada ou lida. A captura resultante é local e ignorada pelo Git.

## Artefatos Playwright anteriores

Foram inventariados sem imprimir conteúdo sensível:

- 57 capturas PNG anteriores correspondem à homologação de páginas do painel de produção; aquela suíte não preencheu o formulário de login. Não há evidência de senha/MFA nessas capturas. Podem conter informação operacional administrativa, portanto devem permanecer privadas.
- O relatório HTML não contém marcação de campos de senha/MFA.
- Dois traces autenticados não contêm marcação de senha/MFA, mas incluem referências a headers/cookies de autenticação. São tratados como potencialmente sensíveis por poderem revelar contexto de sessão. Não foram removidos nem anexados; não compartilhar. Recomenda-se remoção segura dos traces e revogação da sessão se houver suspeita de acesso de terceiros.
- O arquivo de estado Playwright existe no perfil local fora do repositório; seu conteúdo não foi lido. Deve continuar privado e nunca ser compartilhado.
- O teste novo não grava screenshots, vídeo ou trace. Seus resultados intermediários continham exclusivamente credenciais fictícias.

## Homologação manual e classificação

O teste automatizado comprova o isolamento no ramo de configuração MFA, mas não substitui autenticação real. A confirmação manual após a correção permanece **PENDENTE**: o operador deve abrir `/admin/login`, preencher a senha e avançar pelo fluxo legítimo, observar TOTP vazio e concluir MFA manualmente. Não registrar nem compartilhar senha, código, chave, cookie ou captura que os contenha.

**Classificação técnica: APROVADO PARCIALMENTE.** Causa identificada e correção automatizada aprovada. O build foi bloqueado pela indisponibilidade dos Google Fonts, e a confirmação manual real permanece pendente.
