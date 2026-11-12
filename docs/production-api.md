# API administrativa de produção

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

