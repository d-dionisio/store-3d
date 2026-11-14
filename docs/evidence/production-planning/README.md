# Homologação da inteligência de produção — Etapa 10.5.1

Data: 13 de novembro de 2026.

## Ambiente protegido

- Banco: `loja_3d_production_test`.
- O teste rejeita `loja_3d`, bancos sem sufixo `_test` e `TEST_DATABASE_URL` igual a `DATABASE_URL`.
- 152 jobs identificados por `PV-planning-volume-*` foram criados dentro de uma única transação e removidos por rollback.
- Nenhum pedido, pagamento ou job comercial foi usado ou alterado.

## Comparação das métricas

O repository e uma consulta SQL independente retornaram exatamente o mesmo resultado global durante o teste:

| Métrica global | Resultado no ambiente de teste |
| --- | ---: |
| Aguardando produção | 24 |
| Fila e preparação | 44 |
| Execuções imprimindo | 19 |
| Controle de qualidade | 19 |
| Atrasados | 23 |
| Concluídos em 30 dias | 22 |
| Impressoras disponíveis | 7 |
| Impressoras ocupadas | 1 |
| Impressoras em manutenção | 1 |
| Materiais abaixo do mínimo | 1 |

Os valores incluem fixtures isoladas e registros previamente existentes no banco de teste. A asserção compara os objetos completos, e não números codificados no teste. A consulta de planejamento ativo retornou 128 registros; a segunda página com limite 100 não ficou vazia. Jobs concluídos e cancelados só entram quando solicitados explicitamente por status.

## Planos de execução

Resultados de `EXPLAIN (ANALYZE, BUFFERS)` com as 152 fixtures presentes:

| Consulta | Tempo de execução | Buffers | Memória de ordenação |
| --- | ---: | ---: | ---: |
| Dashboard | 0,063 ms | 5 hits | — |
| Planejamento paginado | 0,363 ms | 16 hits | 27 kB |
| Capacidade | 0,198 ms | 5 hits | 37 kB no aggregate |
| Materiais | 0,112 ms | 1 hit | 25 kB |

Foram observados sequential scans. Para apenas 165 jobs, 174 linhas intermediárias e poucas impressoras/bobinas, o otimizador escolheu corretamente varrer as tabelas pequenas: não há evidência para criar novos índices nesta etapa. Não foi detectado padrão N+1; cada visão é carregada por consulta agregada.

## Correções comprovadas

- O resumo de riscos agora é calculado sobre todo o conjunto filtrado, não apenas sobre a página atual.
- A paginação é aplicada após a classificação de risco, corrigindo totais e páginas filtradas.
- Estoque baixo é contado por combinação de material e cor, em vez de contar bobinas isoladas.
- A estrutura de testes passou a permitir repository transacional, mantendo as fixtures invisíveis fora da transação e garantindo rollback.

## Homologação visual

Não realizada nesta execução: não havia ferramenta de navegador/Playwright disponível e uma sessão administrativa real com MFA não foi fornecida ao processo. Nenhuma captura simulada foi criada. Permanecem pendentes as quatro resoluções solicitadas e a gravação de screenshots reais neste diretório.

O procedimento reproduzível está em `manual-visual-checklist.md`.

## Fechamento 10.5.2

- A hierarquia histórica foi validada contra PostgreSQL real: mesma variante/modelo, mesmo produto/modelo, mesmo produto independente do modelo e ausência de histórico.
- A consulta usa uma janela móvel de 90 dias, ignora execuções incompletas ou com duração não positiva e sempre seleciona o nível mais específico disponível.
- Para amostras pares, a mediana agora é a média dos dois valores centrais (`percentile_cont` arredondado), corrigindo o comportamento anterior de escolher o menor valor central.
- Os testes verificam contagem de amostras e confiança `low`, `medium`, `high` e `unknown`.
- Os testes legados de ciclo de vida e integridade passaram a remover somente os IDs exatos que criaram. Uma nova rodada não aumenta as fixtures persistentes.
- Foram encontrados dez grafos antigos no banco `_test`, anteriores ao novo cleanup. Eles foram preservados nesta etapa: a correção impede recorrência, sem fazer uma exclusão retroativa ampla ou ambígua.

### Verificações finais

- `go test ./...`: aprovado, incluindo PostgreSQL real.
- `go vet ./...`: aprovado.
- `npm run lint`: aprovado.
- `npm test -- --run`: 9 arquivos e 26 testes aprovados.
- `npx tsc --noEmit`: aprovado.
- `npm run build`: aprovado; 16 páginas estáticas geradas e rotas dinâmicas compiladas.

O primeiro teste/build frontend dentro do sandbox encontrou, respectivamente, bloqueio de subprocesso (`spawn EPERM`) e bloqueio de rede ao Google Fonts. Ambos passaram sem mudança de código quando repetidos com as permissões necessárias. A tentativa final de reconsultar as contagens pelo container não foi possível porque o Docker Desktop já não estava ativo; a última medição disponível, feita depois dos testes específicos de cleanup, permaneceu em dez grafos legados.
