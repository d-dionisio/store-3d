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

## Testes e limitações

Testes de domínio cobrem centavos/arredondamento, consumo e desperdício, energia, depreciação, manutenção, mão de obra, margem e overflow. O teste PostgreSQL exige `TEST_DATABASE_URL` exatamente para `loja_3d_test`, loopback:5432, migration 28 limpa e usa uma transação com rollback. Ele não cria banco nem aplica migration.

Nesta execução, `DATABASE_URL` e `TEST_DATABASE_URL` estão ausentes no processo. Nenhuma conexão, migration ou escrita PostgreSQL foi feita; migration 28 e teste de integração ainda precisam de homologação real no banco isolado.

Limitações: estimativas usam a bobina compatível mais recente, sem média contábil; potência configurada não substitui medição; manutenção/hora é rateio operacional informado; tributos, embalagem, comissões e rateio contábil indireto não estão incluídos. Validar a definição de margem com a contabilidade antes de uso fiscal/contábil.
