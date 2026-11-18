# API administrativa de produção

## Planejamento operacional

Endpoints consultivos, protegidos pela sessão administrativa e disponíveis para `admin`, `production_manager` e `production_operator`:

- `GET /api/v1/admin/production/dashboard`: contagens globais e concluídos nos últimos 30 dias.
- `GET /api/v1/admin/production/planning`: fila classificada com previsão, confiança e motivos auditáveis. Aceita `status`, `risk`, `printer_id`, `due_from`, `due_to`, `page` e `limit`.
- `GET /api/v1/admin/production/capacity/printers`: estado cadastrado, execuções, carga conhecida e manutenção.
- `GET /api/v1/admin/production/planning/materials`: saldo físico utilizável, mínimo, demanda conhecida e demanda sem estimativa.

Estimativas históricas consideram os últimos 90 dias e usam mediana. Ausência de histórico é devolvida como desconhecida; a API não fabrica tempos. `quote_viewer` e usuários anônimos recebem acesso negado.

Todas as rotas usam a sessão administrativa `loja3d_admin_session`. Mutações exigem também `X-CSRF-Token`.

## Papéis

- `admin` e `production_manager`: acesso integral ao módulo.
- `production_operator`: consulta fila e recursos; inicia preparação e impressão; finaliza ou registra falha; registra consumo e inspeção técnica.
- `quote_viewer`: sem acesso ao módulo de produção.

## Trabalhos e execuções

- `GET /api/v1/admin/production/jobs`
- `POST /api/v1/admin/production/orders/{reference}/jobs`
- `GET /api/v1/admin/production/jobs/{id}`
- `POST /api/v1/admin/production/jobs/{id}/queue`
- `POST /api/v1/admin/production/jobs/{id}/prepare`
- `POST /api/v1/admin/production/jobs/{id}/complete`
- `POST /api/v1/admin/production/jobs/{id}/cancel`
- `POST /api/v1/admin/production/runs/{id}/start`
- `POST /api/v1/admin/production/runs/{id}/finish`
- `POST /api/v1/admin/production/runs/{id}/fail`
- `POST /api/v1/admin/production/runs/{id}/reprint`

Transições recebem `version` para concorrência otimista. Preparação também recebe `printer_id`; cancelamento e falha exigem descrição/motivo.

## Materiais e qualidade

- `POST /api/v1/admin/production/runs/{id}/materials`
- `POST /api/v1/admin/production/runs/{id}/inspections`
- `POST /api/v1/admin/production/runs/{id}/approve`
- `POST /api/v1/admin/production/runs/{id}/reject`
- `GET|POST /api/v1/admin/production/spools`
- `GET|PUT /api/v1/admin/production/spools/{id}`
- `POST /api/v1/admin/production/spools/{id}/adjust`
- `POST /api/v1/admin/production/spools/{id}/quarantine`

Consumo usa `spool_id`, `consumed_grams` e `wasted_grams`. O PostgreSQL bloqueia a bobina, calcula o custo pelo snapshot e impede saldo negativo.

## Impressoras

- `GET|POST /api/v1/admin/production/printers`
- `PUT /api/v1/admin/production/printers/{id}`
- `POST /api/v1/admin/production/printers/{id}/status`
- `POST /api/v1/admin/production/printers/{id}/maintenance`

## Confirmação financeira

Ao reconciliar um pagamento como `paid`, a mesma transação confirma o pedido e chama `create_production_jobs_for_paid_order`. A função bloqueia pedido e pagamento, valida liquidação integral e cria no máximo um job principal por item. Repetição de webhook, reconciliação e retry retorna zero novos jobs sem duplicação.

## Painel administrativo (Etapa 10.4)

- `/admin/producao`: indicadores derivados da listagem real, alertas, prazos e fila com busca, filtros e ordenação locais sobre a página carregada.
- `/admin/producao/jobs/{id}`: transições do job e das execuções, consumo de bobina, inspeção, aprovação, reprovação e histórico append-only.
- `/admin/producao/impressoras`: cadastro, status operacional e registro de manutenção.
- `/admin/producao/materiais`: cadastro, consulta, ajuste versionado e quarentena de bobinas.

O frontend reutiliza a sessão administrativa e envia o token CSRF nas mutações. A API continua sendo a autoridade de RBAC e de transições; respostas `403` e `409` são apresentadas como falha operacional, sem aplicar estado otimista fictício.

Limitações conhecidas do contrato atual:

- `JobView` não expõe data de criação, variante, especificações, material/cor, estimativas nem detalhes das inspeções e consumos. Esses campos não são inventados no painel.
- `SpoolView` não expõe o limite mínimo, linha/lote ou nomes de material e cor. Por isso, estoque baixo não pode ser calculado e o cadastro ainda exige os UUIDs existentes.
- Não existe endpoint para consultar histórico de manutenção ou catálogo global de materiais/cores.
- O dashboard deriva métricas dos até 100 jobs solicitados; totais globais por status exigem um endpoint agregado futuro.

# Otimização consultiva

As rotas administrativas de otimização são `GET /api/v1/admin/production/optimization`, `GET /api/v1/admin/production/optimization/recommendations` e `POST /api/v1/admin/production/optimization/simulate`. O POST recebe `{"strategy":"combined"}` e exige CSRF, embora seja somente uma simulação. As estratégias e o contrato conceitual estão em `docs/production-optimization.md`.

## Gestão técnica (Etapa 10.7)

Leituras exigem `admin`, `production_manager` ou `production_operator`; mutações exigem `admin` ou `production_manager`, sessão MFA e CSRF.

- `GET /printer-technical-specs`
- `GET|PUT /printers/{id}/technical-specs`
- `GET /printers/{id}/technical-specs/history`
- `PUT /printers/{id}/operational-config`
- `GET /materials`
- `GET|PUT /materials/{id}/technical-specs`
- `GET|POST /spool-installations`
- `POST /spool-installations/{id}/remove`
- `POST /compatibility/simulate`

Todos os caminhos têm o prefixo `/api/v1/admin/production`. Atualizações recebem `expected_version`; conflitos retornam 409. Valores técnicos desconhecidos são `null`, e a simulação retorna classificação, motivos e dados ausentes.

## Monitoramento (Etapa 10.8)

- `GET /printers/{id}/monitoring` e `/telemetry`
- `GET|PUT /printers/{id}/connection`
- `POST /printers/{id}/connection/test`
- `GET /printers/{id}/connection/events`
- `GET /printers/{id}/observed-jobs`

Configuração/teste exigem `admin` ou `production_manager`, MFA e CSRF. Consultas permitem `production_operator`. A credencial nunca aparece na resposta.

## Inteligência de produtividade e histórico (Etapa 10.9)

Rotas de leitura protegidas pela sessão administrativa existente, disponíveis para `admin`, `production_manager` e `production_operator`; `quote_viewer` e usuários anônimos não têm acesso. Leituras são auditadas.

- `GET /api/v1/admin/production/history`: execuções paginadas. Parâmetros: `from`, `to` (datas inclusivas), `printer_id`, `product_id`, `material_id`, `product`, `material`, `status`, `quality`, `page`, `limit` (máximo 100).
- `GET /api/v1/admin/production/history/{runID}`: execução, linhas de consumo e custo histórico, inspeções, eventos append-only e candidatos temporais de telemetria.
- `GET /api/v1/admin/production/analytics/productivity`: resumo e série diária.
- `GET /api/v1/admin/production/analytics/printers`: agregados por impressora.
- `GET /api/v1/admin/production/analytics/materials`: gramas consumidos/desperdiçados e custo por material.
- `GET /api/v1/admin/production/analytics/costs`: resumo de custos e série por dia.
- `GET /api/v1/admin/production/analytics/time-accuracy`: medianas estimada/real e desvio por produto/variante.

Filtros textuais `product` e `material` são buscas parciais, parametrizadas e limitadas a 100 caracteres; os filtros `*_id` são identificadores exatos. `actual_minutes` é mostrado como duração quando registrado, mas médias, medianas e precisão incluem apenas durações positivas. Custos usam os snapshots de linhas de consumo; runs legados sem linhas usam `production_runs.material_cost_cents`. Não somar valores de origem dupla evita dupla contagem.

Utilização de impressora é desconhecida (`null`) sem registro confiável do tempo disponível. Telemetria que apenas coincide com o intervalo de uma execução aparece como `time_window_candidate`; não é tratada como prova nem associada automaticamente. Métricas não modificam pedidos, produção, estoque ou snapshots comerciais.

Homologação Etapa 10.9.2 (2026-11-18): `TestProductionAnalyticsPostgreSQL` aprovado em PostgreSQL 17.6, banco isolado `loja_3d_test`, migration 27 (`dirty=false`). A cobertura inclui filtros combinados, paginação/ordenação, estados e durações incompletas, medianas pares/ímpares, custos e desperdício históricos, reimpressão, agregações sem duplicidade, alteração do preço atual de bobina e comparação contra SQL independente.

`EXPLAIN (ANALYZE, BUFFERS)` foi coletado para sete consultas com fixture transacional de 7 runs. Os scans sequenciais observados foram em tabelas pequenas; não foi criado índice. Esses tempos não representam benchmark de carga. Teste HTTP RBAC valida 401/403/200 por papel com sessão de teste injetada; o fluxo real de sessão/MFA não foi simulado nem alterado. Relatório completo, métricas e ressalvas: `docs/evidence/production-analytics-homologation.md`.

## Etapa 10.10 — Custos completos de fabricação

Todas as rotas abaixo são administrativas. Perfis, snapshots financeiros e rentabilidade exigem `admin` ou `production_manager`; alterações exigem cookie/sessão administrativa, CSRF e token de reautenticação (`X-Reauthentication-Token`). As operações são auditadas pelo mecanismo administrativo existente.

- `GET /api/v1/admin/production/printers/{printerID}/cost-profile`: configuração vigente da impressora.
- `PUT /api/v1/admin/production/printers/{printerID}/cost-profile`: cria nova versão; exige `expected_version` igual à versão vigente (0 no primeiro cadastro). Não altera versões anteriores.
- `GET /api/v1/admin/production/printers/{printerID}/cost-profile/history`: histórico de configurações.
- `POST /api/v1/admin/production/runs/{runID}/costs/estimate`: cria snapshot estimado com tempo/peso planejado, perfil vigente e preço da bobina compatível mais recente. A origem do preço fica discriminada.
- `POST /api/v1/admin/production/runs/{runID}/costs/actual`: exige execução terminal, dados registrados e `preparation_seconds`/`finishing_seconds` observados. Opcionalmente recebe `energy_milliwh`; sem medição, energia usa potência cadastrada e origem `configured_power`.
- `GET /api/v1/admin/production/runs/{runID}/costs`: snapshots estimado/real e breakdown.
- `GET /api/v1/admin/production/analytics/cost-profitability?from=YYYY-MM-DD&to=YYYY-MM-DD`: custos, receita, contribuição e lucro por produto.
- `GET /api/v1/admin/production/analytics/cost-profitability/printers`: indicadores por impressora.
- `GET /api/v1/admin/production/analytics/cost-estimate-comparison`: diferença estimado versus realizado por execução com ambos snapshots.

Moeda trafega em centavos inteiros; energia em milliWh, potência em mW e duração em segundos. A margem de contribuição operacional desconta custos variáveis (material, energia, manutenção e mão de obra); depreciação integra custo total/lucro. Reimpressões acumulam custo e receita comercial é contada uma vez por job. Estimativas não inventam dados: sem profile, peso/tempo ou bobina correspondente retornam conflito de dados insuficientes. A UI Dark Tech está planejada em `/admin/producao/custos`, mas não foi construída nesta etapa.

