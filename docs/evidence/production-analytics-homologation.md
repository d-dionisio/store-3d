# Evidências — homologação de analytics de produção (Etapa 10.9.1)

Data da tentativa: 2026-11-18 (America/Sao_Paulo)

## Retomada 10.9.1 — pré-verificação interrompida

Na retomada, `DATABASE_URL` foi encontrado no `.env` raiz, mas `TEST_DATABASE_URL` não foi encontrado no `.env` raiz, em `backend/.env`, em `backend/.env.test`, em `frontend/.env.local` nem no ambiente do processo consultado. Os valores das URLs não foram impressos.

A pré-condição de banco isolado não foi satisfeita. Por isso, a execução foi interrompida antes de qualquer chamada Docker, conexão PostgreSQL, SQL, migration ou teste de integração. Nenhum banco foi criado e nenhum container/volume foi consultado ou alterado nesta retomada. Versão do servidor PostgreSQL e versão da migration do banco de teste permanecem não verificadas.

## Resultado

**Pendente / bloqueado por ambiente.** O Docker CLI está instalado (28.3.2, contexto `desktop-linux`), porém o acesso ao engine Docker Desktop falhou com `Acesso negado` ao pipe `//./pipe/dockerDesktopLinuxEngine`. `docker compose ps` apresentou a mesma falha.

Por orientação de segurança, a homologação PostgreSQL foi interrompida nesse ponto. Não houve tentativa de iniciar ou recriar serviços, conectar por outra rota, executar migrations/SQL, acessar o banco comercial, inspecionar/alterar volumes ou gerar fixtures. Não foram criados snapshots de contagens comerciais: sem conexão autorizada, não seria correto declarar integridade pré/pós.

## Cenários PostgreSQL e EXPLAIN

Nenhum cenário foi executado contra PostgreSQL real nesta tentativa. Portanto estão pendentes: histórico e filtros; paginação/ordenação; detalhes e eventos; indicadores e agregações; durações/medianas; custos por snapshots; concorrência/consistência; comparação SQL independente; `EXPLAIN (ANALYZE, BUFFERS)` das consultas operacionais.

O teste `TestProductionAnalyticsPostgreSQL` existente exige `TEST_DATABASE_URL` diferente de `DATABASE_URL` e nome de banco terminado em `_test`; fixtures são executadas em transação revertida. A tentativa anterior de conexão do ambiente ao endpoint local falhou com conexão recusada. Não foi criado workaround nem usado outro banco.

## Frontend

Verificações executadas em 2026-11-18:

- `npm run lint`: aprovado, sem warnings.
- `npm test -- --run`: aprovado; 11 arquivos e 32 testes.
- `npx tsc --noEmit`: aprovado (o pipeline prosseguiu para o build).
- `npm run build`: aprovado; rotas de histórico e desempenho compiladas.

A revisão visual autenticada permanece pendente: não foi disponibilizada sessão administrativa legítima nesta execução e MFA não foi contornado.

Rotas a conferir visualmente: `/admin/producao/historico`, `/admin/producao/historico/[runID]` e `/admin/producao/desempenho`, em 1440×1200, 820×1180, 390×844 e 320×720.

## Integridade e conclusão

Não houve operação de escrita no banco nesta tentativa; assim, não há alteração comercial causada por esta homologação. A comparação formal antes/depois continua pendente, pois requer leitura autorizada do PostgreSQL isolado.

A Etapa 10.9 **não está homologada**. Para retomar: operador habilita o Docker Desktop e acesso ao engine; confirmar `DATABASE_URL` e `TEST_DATABASE_URL` são distintos, validar que apenas a URL de teste termina em `_test`, executar as integrações/consultas e anexar planos e contagens verificáveis. Não aplicar migration no banco `loja_3d`.

## Retomada com banco de teste disponível — 2026-11-18

Esta seção supersede o bloqueio operacional acima. As URLs foram lidas sem imprimir usuário ou senha: `DATABASE_URL` apontou para `loja_3d` e `TEST_DATABASE_URL` para `loja_3d_test`, ambas em `localhost:5432`, diferentes entre si. Todas as conexões e comandos PostgreSQL desta execução foram explicitamente direcionados ao banco de teste.

Pré-verificações observadas:

- Docker Engine 28.3.2, contexto `desktop-linux`.
- Container `store_3d-postgres-1`: `running`, `healthy`; publicação `127.0.0.1:5432`.
- `current_database()` em conexão explícita: `loja_3d_test`; PostgreSQL 17.6.
- Migrations: versão 27, `dirty=false`; nenhuma migration foi executada.
- O teste PostgreSQL foi o único teste de analytics executado com conexão: `TestProductionAnalyticsPostgreSQL` passou (0,26 s). O teste exige banco `_test`, recusa `loja_3d`, abre uma transação, usa fixtures identificadas por sufixo e agenda rollback. Consultas posteriores no banco de teste encontraram zero produto de fixture `Produto Analytics` e zero usuário `analytics-*@example.test`.
- Testes de aplicação `Analytics` (2) e handler `ProductionAnalytics` (1) passaram. A cobertura verificada inclui paginação básica, filtro de status, detalhamento com material/inspeção, contagem de conclusão/falha/reimpressão, custo de snapshot, mediana par, filtro inválido, `quote_viewer` proibido e handler anônimo 401.

### EXPLAIN (ANALYZE, BUFFERS)

Foram executados planos para listagem paginada, detalhe por UUID, resumo de produtividade, agrupamento por impressora, consumo/custo por material e precisão temporal, dentro de `BEGIN READ ONLY` em `loja_3d_test`. O banco estava sem as fixtures após o rollback, portanto os planos representam schema vazio/baixo volume e não são evidência de capacidade sob carga.

- Histórico paginado: 0 linhas; execução 0,346 ms, planejamento 10,558 ms, 7 shared buffer hits. O plano usou scans sequenciais em tabelas sem linhas e sort em memória; índice de inspeção disponível aparece como index-only scan no ramo lateral.
- Detalhe por ID inexistente: 0 linhas; execução 0,107 ms, planejamento 0,894 ms, 1 shared buffer hit. Usou `production_runs_id_job_key`, PKs/índices relacionais e `quality_inspections_run_idx` no ramo lateral.
- Agregações gerais e por material/produto: consultas executadas e concluídas; com zero runs não há medição representativa de cardinalidade, buffers ou concorrência. Índices existentes foram observados; não há base mensurável para propor índices adicionais.
- A consulta de detalhe faz consultas separadas para materiais, inspeções, eventos e candidatos de telemetria (até quatro consultas após a consulta-base); não há consulta por item de lista, mas os planos isolados não substituem uma medição de latência por requisição.

Não foi executada comparação de totais analíticos com SQL independente sobre fixtures persistentes; as fixtures são transacionais e foram revertidas. Os valores esperados do teste são assertions do cenário, não uma segunda implementação SQL independente.

### Regressão e limitações

- `go test ./...`: aprovado sem `DATABASE_URL` nem `TEST_DATABASE_URL`; integrações que exigem banco foram puladas. A integração PostgreSQL de analytics foi executada separadamente acima.
- `go vet ./...`: aprovado.
- Frontend: lint aprovado; Vitest aprovado (11 arquivos, 32 testes); `npx tsc --noEmit` aprovado; build Next.js aprovado. O primeiro Vitest sob sandbox falhou ao spawn (`EPERM`) antes de carregar testes; a repetição autorizada concluiu com sucesso.
- A suíte completa não foi apontada ao banco de teste: revisão encontrou integrações antigas com `DELETE` direto e limpeza ampla de tabelas (por exemplo, outbox), sem rollback uniforme. Nenhuma delas foi executada nesta retomada.
- Não foi aberta conexão com `loja_3d`; logo, não há snapshot pré/pós das contagens comerciais. As operações executadas nesta homologação apontaram somente ao banco `_test`, e não houve comando de escrita direcionado ao banco comercial.
- Segurança HTTP comprovada parcialmente por testes unitários: anônimo 401, quote viewer 403 na camada de aplicação; operador autorizado em chamada com repositório stub. Matriz HTTP completa para operator/manager/admin, comparação SQL independente e EXPLAIN com volume representativo permanecem pendentes.

**Conclusão: Etapa 10.9 continua não homologada integralmente.** O teste PostgreSQL real de analytics passou e a infraestrutura/schema foram confirmados, mas a matriz de cenários, comparação SQL independente, teste de carga/planos representativos e matriz RBAC completa não estão cobertos pelas verificações existentes. Não houve alterações comerciais ou migrations.

## Etapa 10.9.2 — cobertura adicional executada

Pré-verificação repetida antes dos testes: as duas URLs eram distintas, loopback e porta 5432; a URL de teste apontou exatamente para `loja_3d_test`. Docker 28.3.2, container PostgreSQL saudável, PostgreSQL 17.6; migration 27, `dirty=false`. Nenhuma migration foi executada, e não houve conexão ao banco comercial.

O teste `TestProductionAnalyticsPostgreSQL` foi ampliado e passou no PostgreSQL real. Todas as fixtures (1 pedido/item, 1 produto/variante, 1 job, 7 runs, 1 impressora, 2 bobinas, 4 snapshots de consumo, 2 inspeções e 1 evento) foram inseridas em uma transação com rollback. A role de replicação é alterada somente com `SET LOCAL` nessa transação de teste para isolar triggers comerciais e não persiste após rollback.

### Cenários adicionais e referência SQL independente

- Filtros combinados: intervalo, impressora, produto, material, texto de produto/material, status e qualidade retornaram somente a execução esperada. Também foram conferidos filtros de data, status de falha e qualidade reprovada.
- Paginação e ordem descendente: 7 registros, páginas com 3 linhas, sem sobreposição e na ordem temporal configurada.
- Estados/duração: 5 concluídas, 1 falha e 1 impressão ainda ativa; uma duração zero e uma estimativa ausente não entraram na precisão temporal. A amostra temporal válida ficou em 3, exercitando mediana ímpar. Um filtro temporal com as duas execuções válidas mais recentes exercitou mediana par (estimada 135, realizada 150, diferença 15 min).
- Reimpressão: 1; agregações não multiplicaram execuções apesar de uma run ter dois snapshots de bobina.
- Consumo: 650 g consumidos e 17 g desperdiçados. O detalhe da run com duas bobinas apresentou 150 g, 7 g de desperdício e custo snapshot de 1.800 centavos.
- Custos: 5.800 centavos somados dos quatro snapshots de consumo mais 700 centavos da run legada sem linhas de consumo = 6.500 centavos. O preço atual das bobinas foi alterado dentro da transação depois dos snapshots; a API permaneceu em 6.500 centavos, comprovando que não recalcula histórico pelo preço atual.
- Resumo esperado e retornado: 7 runs, 5 concluídas, 1 falha, 1 reimpressão, 725 minutos registrados, 650 g consumidos, 17 g desperdiçados, custo 6.500 centavos, sucesso 83,33%, falha 16,67%.
- SQL independente, escrito no próprio teste e sem chamar funções de cálculo da aplicação, comparou contagem/status/reimpressões, duração, consumo, desperdício, custo, taxas, percentis temporais e agregados por impressora, produto e material. Todas as assertions API versus SQL passaram.
- Run legada sem snapshot de material preservou o custo de 700 centavos; a mediana excluiu falha, zero, sem estimativa e run incompleta.

### EXPLAIN (ANALYZE, BUFFERS)

Os sete planos foram executados dentro da mesma transação com 7 runs sintéticas; são tempos de uma fixture pequena, úteis para validar planos e não para estimar throughput. Valores observados na última execução:

| Consulta | Linhas no fixture | Shared hits | Planning | Execution | Scans/índices relevantes |
|---|---:|---:|---:|---:|---|
| Histórico paginado | 7 (3 retornadas) | 56 | 2,580 ms | 0,763 ms | seq scan em `order_items`; índices de job/run, pedido, material, cor e inspeção |
| Detalhe | 1 | 13 | 1,525 ms | 0,356 ms | seq scan em `production_runs` para a tabela pequena; PKs e índice de qualidade |
| Produtividade geral | 7 | 66 | 1,407 ms | 0,836 ms | seq scan em `order_items`; índices relacionais e de consumo |
| Produtividade por impressora | 7 | 66 | 1,830 ms | 1,330 ms | seq scan em `order_items`; índice de run por job |
| Consumo/custo por material | 7 | 66 | 1,708 ms | 1,190 ms | seq scan em `order_items`; índice único de consumo por run/bobina |
| Custo histórico | 7 | 66 | 2,058 ms | 1,293 ms | seq scan em `order_items`; índices de execução e consumo |
| Precisão temporal | 3 válidas | 35 | 3,003 ms | 1,095 ms | seq scan em `production_runs` pequeno; índice de run por job |

Os sequential scans observados ocorreram em tabelas minúsculas da fixture e não justificam novos índices. O planejador alternou para índices existentes nos joins/lookup. Não foi executado teste de carga com volume alto; nenhuma migration ou índice foi criado. A listagem usa duas consultas fixas (count e página), produtividade faz consultas agregadas fixas e detalhe carrega conjuntos relacionados em quantidade fixa, sem consulta por item de lista (sem N+1 na paginação).

### Autorização HTTP e regressão

- Teste HTTP de handler após resolução de sessão: anônimo 401; `quote_viewer` 403; `production_operator`, `production_manager` e `admin` 200 para leitura de analytics; filtro inválido 400. Esse teste usa um stub do repositório de sessão para isolar o gate RBAC; não valida emissão/expiração de sessão nem o desafio MFA real. O teste de aplicação também confirmou `quote_viewer` proibido e operador permitido. Código de MFA, CSRF, sessão e gateway Zero Trust não foi alterado.
- `go test ./...` passou sem `DATABASE_URL` e sem `TEST_DATABASE_URL`; assim, integrações antigas foram puladas. O teste analytics PostgreSQL real foi executado separadamente, conforme acima. `go vet ./...` passou.
- Frontend: `npm run lint`, `npm test -- --run` (11 arquivos/32 testes), `npx tsc --noEmit` e `npm run build` passaram. Build compilou as rotas de histórico e desempenho.
- Depois do rollback, SELECTs em `loja_3d_test` confirmaram zero produtos/users/admins/printers marcados pelas fixtures. Nenhuma escrita foi direcionada a `loja_3d`; não foram coletadas contagens comerciais antes/depois porque esse banco não foi conectado.

**Conclusão atual:** a matriz de analytics PostgreSQL, comparação independente e planos com fixture representativa de casos passaram. A homologação operacional de analytics está concluída para correção funcional em baixo volume; permanecem ressalvas para uma alegação de performance sob carga e para teste ponta a ponta do fluxo real de MFA/sessões. Os resultados não afirmam capacidade de produção sob volume alto.
