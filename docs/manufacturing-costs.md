# Etapa 10.10 — Motor de custos de fabricação

## Diagnóstico

- `production_runs` já possui duração/peso estimados e realizados, além de snapshots parciais de material, energia e outros custos.
- `production_run_materials` é a fonte histórica de consumo, desperdício, preço por kg e custo total por bobina. O trigger captura o preço no momento do consumo e torna as linhas imutáveis.
- `material_spools.cost_per_kg_cents` é o preço operacional atual, não um histórico de alterações. Custos realizados usam os snapshots das linhas de consumo, sem recalcular por preço atual.
- `order_items` guarda preços e totais comerciais snapshot. São somente leitura no motor; checkout, pagamentos e dados comerciais não são atualizados.
- Analytics 10.9 já agrega material e tempo; o novo módulo mantém rotas próprias para não alterar o contrato legado.
- Não há medição elétrica automática nem potência validada da A1 no schema. Nenhum valor técnico vem pré-preenchido.

## Modelo e cálculo

Migration 28 cria `production_cost_profiles`, `production_cost_profile_current` e `production_run_cost_snapshots`. Perfis são históricos append-only por impressora; um ponteiro independente indica a versão vigente. Snapshots estimate/actual são append-only e guardam a versão usada, medições, origem da energia e breakdown JSON de materiais.

Unidades: centavos BIGINT, massa em gramas, energia em milliWh, potência em mW, tempo em segundos. O domínio Go usa aritmética inteira/racional com intermediários `math/big`, arredondamento half-up, sem `float` para cálculos monetários.

- Material consumido e desperdício são discriminados; ambos integram o custo material, usando os preços snapshot por bobina.
- Energia usa medição informada ou potência explicitamente cadastrada multiplicada pelo tempo. A origem é persistida como `measured` ou `configured_power`.
- Depreciação é aquisição × duração / vida útil. Manutenção é taxa horária × duração.
- Mão de obra é segmentada entre preparação e acabamento. Estimativas usam tempos padrão; custo real exige tempos observados na requisição.
- Custo total inclui material, energia, depreciação, manutenção e mão de obra.
- Margem de contribuição operacional subtrai custos variáveis (material, energia, manutenção e mão de obra) da receita. Depreciação fica no lucro/custo total. Percentuais são basis points inteiros.

Quando receita snapshot é zero (por exemplo, execução falha), os valores de margem em dinheiro continuam disponíveis, enquanto o percentual é nulo por não haver denominador de receita.

Execuções falhas/canceladas podem receber custo realizado quando há dados registrados. Reimpressões acumulam custos e a receita do item é atribuída a uma única execução concluída para não duplicar venda. Preço de bobina usado em estimativa fica identificado como a bobina compatível mais recente; preço realizado vem exclusivamente do snapshot de consumo.

## Segurança e interface

Configuração, custos e rentabilidade são gerenciais (`admin`/`production_manager`). Alterações usam autenticação administrativa existente, CSRF, reautenticação forte, controle otimista de versão e auditoria; não há autenticação paralela.

Interface planejada `/admin/producao/custos` (Dark Tech): configuração versionada por impressora com fonte/medição e confirmação de reautenticação; detalhe da execução comparando estimado/real e origem de cada dado; análise por produto/impressora com filtros temporais, custo unitário, receita, contribuição e lucro. Não foi construída nesta etapa.

Contratos REST estão documentados em `docs/production-api.md`.

### Auditoria obrigatória (Etapa 10.10.2.1)

As rotas de custo usam somente a auditoria administrativa existente. Atualizações de perfil e gravação/idempotência de snapshots inserem o evento de sucesso na mesma transação PostgreSQL da mutação; falha de auditoria aborta o commit. Leituras sensíveis e operações recusadas só retornam a resposta normal depois de persistir auditoria; falha do sink resulta em resposta genérica 503. Recusas registram eventos de vocabulário fechado para sessão, CSRF e reautenticação, sem copiar credenciais ou mensagens de erro. A homologação real de PostgreSQL desta alteração não foi executada nesta rodada porque `TEST_DATABASE_URL` estava ausente.

## Homologação PostgreSQL (Etapa 10.10.1, 2026-11-18)

Pré-verificação: `.env` foi lido sem imprimir credenciais; `DATABASE_URL` e `TEST_DATABASE_URL` eram diferentes e apontavam para `localhost:5432/loja_3d` e `localhost:5432/loja_3d_test`, respectivamente. A evidência do operador confirmou o container `store_3d-postgres-1` em execução. Uma consulta somente leitura pelo URL de teste confirmou `current_database()=loja_3d_test`, PostgreSQL `17.6`, migration `27`, `dirty=false`.

Migration 28 foi aplicada apenas ao banco de teste. Up: `28/false`, com 57 constraints, 3 triggers e 8 índices nas tabelas de custo. Down: `27/false`, e a inspeção encontrou zero constraints/triggers/índices dessas tabelas. A reaplicação terminou em `28/false`, restaurando 57/3/8. Nenhum `force` foi usado.

`TestProductionCostsPostgreSQL` passou no PostgreSQL real. O teste mantém em uma transação com rollback fixtures identificadas de administrador, impressora, material, cor, orçamento/proposta, pedido pago, item, job, runs (incluindo reimpressão), bobina, consumo e snapshots. Verificou custos de material (2000 centavos consumidos + 200 de desperdício), energia (63), depreciação (10), manutenção (20), preparação (1500), acabamento (750), mão de obra total (2250), custo total/unitário (4543), margem de contribuição (5467) e lucro (5457). O caso inclui arredondamento half-up de energia em 62,5 centavos.

O teste também confirmou: débito de 110 g da bobina; custo/preço snapshot do consumo em 20.000 centavos/kg; reimpressão vinculada a uma tentativa falha; rejeição de número de tentativa duplicado; perfis 1–3 com conflito de versão obsoleta; uso do preço e perfil atuais somente na nova estimativa; snapshot realizado ainda apontando para preço 20.000 e perfil 2 após alterações para preço 90.000, tarifa 777, aquisição 200.000, manutenção 400 e mão de obra 6.000; update do snapshot imutável rejeitado; snapshot inconsistente e FK inválida rejeitados sem linha parcial; nova tentativa válida bem-sucedida; e zero resíduos após o rollback, conferido por IDs exatos em todas as tabelas de fixture.

Os testes passaram em camadas distintas: handler/router HTTP confirmou anônimo 401; teste de aplicação confirmou `quote_viewer` e `production_operator` negados; `production_manager` e `admin` autorizados. A Fase 3 ampliou os testes HTTP automatizados para seis endpoints GET e os fluxos de mutação com CSRF ausente/inválido e reautenticação ausente. Esses testes de handler usam sessão de teste injetada e não são evidência de sessão real/MFA. Evidências detalhadas e limitações estão em `docs/evidence/manufacturing-costs-homologation.md`.

Regressão: `go test ./...` e `go vet ./...` passaram com URLs de banco removidas do processo, de modo que os testes antigos que fazem cleanup por DELETE amplo foram ignorados. O teste PostgreSQL novo foi executado separadamente depois de revisar sua proteção de banco e rollback.

**Concorrência (Fase 2, 2026-11-18):** teste PostgreSQL real usou dois pools/backends independentes, esperas por lock observadas em `pg_stat_activity`, atualização otimista concorrente (um sucesso e um conflito), gravação concorrente idempotente de snapshot, unicidade, rollback e imutabilidade. Fixtures isoladas em schema efêmero foram removidas e verificadas. As tabelas clonadas não carregam FKs; veja a ressalva em `docs/evidence/manufacturing-costs-homologation.md`. A criação/execução do run e o snapshot de custo continuam sendo operações transacionais separadas; falha ao gravar snapshot não apaga um run já existente, mas não deixa snapshot parcial.

Não foi aberta conexão ao banco `loja_3d` e não foram alterados produtos, pedidos, pagamentos, estoque, preço de venda ou snapshots comerciais. Os registros sintéticos foram todos revertidos no banco `_test`/schema efêmero.

Limitações de negócio mantidas: estimativas usam a bobina compatível mais recente, não média contábil; potência configurada não substitui medição; manutenção/hora é rateio informado; tributos, embalagem, comissões e rateio contábil indireto não estão incluídos. Validar a definição da margem com a contabilidade antes de uso fiscal/contábil. A homologação PostgreSQL, incluindo corrida concorrente, está aprovada. A homologação real de sessão/MFA/CSRF/reautenticação permanece pendente; além disso, os handlers ignoram falhas de gravação de auditoria e não auditam falhas de CSRF/reautenticação antes do retorno, limitação registrada na evidência da Fase 3.

**Fase 4 (2026-11-18):** `go test ./...`, `go vet ./...` e regressões direcionadas de custos passaram. A suíte Go foi executada sem `DATABASE_URL` e `TEST_DATABASE_URL`; portanto, testes opt-in com PostgreSQL e limpeza destrutiva foram ignorados, sem conexão ao banco comercial. Frontend: lint, 11 arquivos/32 testes, TypeScript e build passaram. O Vitest e o build precisaram de repetição fora do sandbox por `spawn EPERM`. Nenhuma migration ou dado comercial foi alterado. Detalhes e comandos em `docs/evidence/manufacturing-costs-homologation.md`.

**Decisão de homologação:** implementação e concorrência técnica estão evidenciadas, mas a Etapa 10.10.1 não está integralmente homologada: MFA/sessão administrativa real, reautenticação ponta a ponta, auditoria durável e revisão visual autenticada permanecem pendentes. A matriz completa de classificação e os próximos passos estão em `docs/evidence/manufacturing-costs-final-checklist.md`.
