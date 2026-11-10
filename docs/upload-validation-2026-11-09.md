# Validação integrada e hardening de uploads — 9 de novembro de 2026

## Evidências executadas

- PostgreSQL `17-alpine`: container saudável.
- Migrations: versão `9`, `dirty=false` em banco vazio.
- Criação real no PostgreSQL: orçamento e `quote_files` associados na mesma transação.
- Reutilização: upload `consumed` rejeitado.
- Concorrência: duas transações disputaram o mesmo upload; uma confirmou e uma recebeu `forbidden`.
- Rollback: falha proposital na constraint de `quote_files` não deixou orçamento e preservou o upload `confirmed`.
- A suíte completa exigiu o seed demonstrativo por uma pré-condição já existente nos testes de catálogo; após carregá-lo, todos os pacotes passaram.
- `go vet ./...`, `npm run lint` e `npm run build` concluíram sem erros.

Cloudflare R2 não foi validado ao vivo: nenhuma variável `R2_*` ou bucket de teste foi fornecido. Os testes de storage permanecem locais/mocados e não constituem evidência de integração R2.

## Autorização de consulta

O token atual é uma credencial de capacidade para visitante, não autenticação de identidade:

1. A API gera 256 bits aleatórios ao criar o orçamento.
2. O valor bruto é entregue uma vez ao visitante e salvo pelo frontend no `localStorage`.
3. Somente SHA-256 do token é persistido em `quotes.public_access_token_hash`.
4. `GET /api/v1/quotes/{reference}` compara o bearer token por igualdade no PostgreSQL e retorna apenas referência, status e data.
5. Remover o hash do banco revoga o token, mas ainda não existe caso de uso administrativo de revogação.

O token não prova quem é o cliente, não recupera acesso perdido, não possui rotação e é vulnerável a scripts executados na mesma origem por estar no `localStorage`. Portanto, consulta de visitante não está liberada para produção. A etapa de autenticação deve definir sessão HTTP-only, verificação de e-mail ou magic link, recuperação, revogação e associação de `quotes.user_id`. Até lá, o endpoint deve ser tratado como experimental e nunca retornar PII ou URLs de objetos.

## Scanner assíncrono proposto

Fluxo: `pending → scanning → clean | rejected | failed`.

- Uma fila envia somente `quote_file_id`; workers obtêm objetos com URL de leitura curta e identidade de serviço.
- Cada análise roda em container/microVM sem rede de saída, filesystem descartável, usuário sem privilégios e limites rígidos de CPU, memória, tempo, processos e bytes lidos/escritos.
- ClamAV ou serviço equivalente verifica malware antes de parsers de modelo.
- STL: validar tamanho, contagem de triângulos, valores finitos, limites geométricos e ausência de conteúdo anexado inesperado.
- 3MF: validar ZIP central directory sem extrair inicialmente; limitar entradas, tamanho compactado, tamanho total descompactado, tamanho por entrada, profundidade de caminhos e razão de compressão; rejeitar path traversal, links, criptografia, ZIP64 fora da política e arquivos aninhados.
- Após extração limitada, validar OPC/3MF XML com parser sem entidades externas e allowlist de partes/relacionamentos.
- Registrar versão do scanner, regras, checksum observado, tempos, métricas, resultado e motivo sem armazenar conteúdo sensível no log.
- Retentativas exponenciais somente para falhas transitórias; após limite, estado `failed` e revisão operacional. Resultado malicioso ou estruturalmente inválido vira `rejected`, sem retentativa automática.
- Apenas o worker pode escrever `clean`, sempre preenchendo `security_checked_at` e auditoria imutável.

## Riscos restantes e liberação

Antes de produção são obrigatórios:

- teste real com bucket R2 privado, CORS restrito e credencial de escopo mínimo;
- confirmar suporte do R2 a `x-amz-checksum-sha256` no PUT pré-assinado e retorno no HEAD;
- lifecycle rule e job idempotente para objetos expirados/órfãos;
- rate limiter distribuído confiando apenas em IP fornecido por proxy conhecido;
- scanner efetivo com testes de decompression bomb e malware;
- autenticação/revogação de consulta de orçamento;
- observabilidade e alertas de falhas, rejeições e abuso.

Nenhum arquivo pode avançar para análise técnica enquanto `security_status` não for `clean` por um scanner real.
