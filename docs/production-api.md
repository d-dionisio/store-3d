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

