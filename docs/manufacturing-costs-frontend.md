# Interface administrativa de custos de fabricação

Etapa 10.10.2.4.1 — implementação frontend.

## Rotas

- `/admin/producao/custos`: custo de material do analytics operacional, gráfico temporal relativo (com valores exatos e tabela), rentabilidade e comparação estimado versus realizado.
- `/admin/producao/custos/impressoras`: perfil atual e versões históricas por impressora.
- `/admin/producao/custos/execucoes`: seleção de execuções reais, consulta de snapshots e registro explícito de estimate/actual.
- `/admin/producao/custos/rentabilidade`: relatórios por produto e impressora, mais comparação estimado/realizado.

## Arquitetura de UI

- `frontend/src/services/production-api.ts`: contratos dos DTOs e chamadas HTTP dos endpoints já existentes.
- `frontend/src/components/production/cost-management.tsx`: navegação local de custos, formulário versionado, consulta de snapshots, confirmação de mutação, diálogo de reautenticação e tabelas financeiras.
- `frontend/src/components/production/costs-state.ts`: formatação de apresentação e conversão textual de moeda em centavos para o DTO, sem fórmulas de custo.
- `frontend/src/app/admin/producao/custos/**/page.tsx`: Server Components com carregamento autenticado por cookie de sessão e estados de erro de autorização.
- `frontend/src/app/admin/manufacturing-costs.css`: tokens e responsividade alinhados à identidade Dark Tech de produção.

## Contratos consumidos

Todos os endpoints permanecem sob `/api/v1/admin/production`:

```text
GET  /analytics/costs?from=&to=
GET  /analytics/cost-profitability?from=&to=
GET  /analytics/cost-profitability/printers?from=&to=
GET  /analytics/cost-estimate-comparison?from=&to=
GET  /printers
GET  /printers/{printerID}/cost-profile
GET  /printers/{printerID}/cost-profile/history
PUT  /printers/{printerID}/cost-profile
GET  /history?page=1&limit=100
GET  /runs/{runID}/costs
POST /runs/{runID}/costs/estimate
POST /runs/{runID}/costs/actual
```

`PUT` envia todos os campos do `CostProfileInput`, inclusive `expected_version`. Vida útil é mostrada em horas e convertida em segundos para o contrato. Campos financeiros editáveis aceitam valores com até duas casas e são enviados em centavos inteiros. Se potência não está configurada, exige-se observação da origem da medição; nenhum consumo de impressora é presumido.

## Segurança e permissões

Consultas financeiras, perfil e snapshots são autorizados pelo backend. A política atual de `canManage` permite `admin` e `production_manager`; analytics operacional compartilhado conserva a autorização existente. Erros 401 redirecionam ao login nas cargas server-side e 403 é mostrado como acesso não autorizado. A interface não decide permissão por role local.

Toda mutação de perfil ou criação de snapshot passa pelo diálogo MFA/reautenticação já existente (`reauthenticateAdmin`) e envia `X-CSRF-Token` e `X-Reauthentication-Token`. O proof não é persistido em localStorage/sessionStorage e é mantido transitoriamente apenas para a chamada. Não há login, sessão ou mecanismo MFA paralelo.

## Imutabilidade e semântica financeira

- A API continua calculando; o frontend não soma itens, recalcula margem, custo unitário ou diferença.
- Dinheiro vem em centavos inteiros e é formatado como BRL. Margens em basis points são apresentadas como percentuais.
- `/analytics/costs` é custo/material do analytics operacional, não custo total de fabricação; a tela deixa essa distinção visível.
- O endpoint atual não oferece estimativa de leitura: `POST .../estimate` grava um snapshot. A UI confirma explicitamente essa persistência e não o chama de simulação descartável.
- Snapshots estimate/actual são históricos append-only. Não há ação para editar ou excluir.
- HTTP 409 no salvamento de perfil comunica conflito de versão e recomenda atualização; o backend permanece autoridade.
- `material_breakdown` é apresentado somente quando o DTO contém a lista; campos ausentes aparecem como “Não disponível” ou não são inferidos.

## Limitações e homologação

- A lista de execução usa a primeira página de até 100 entradas do endpoint de histórico; a API atual não fornece picker específico nem filtro financeiro por execução na UI.
- Não existe endpoint de overview com agregados completos de fabricação nem endpoint para estimar sem persistir. A tela evita criar esses indicadores no cliente.
- Snapshot JSON não expõe `created_at` no DTO atual; portanto a UI não afirma uma data de criação que não recebeu.
- API e RBAC devem ser homologados com sessão legítima MFA. Testes unitários e build não comprovam login real, permissões reais nem comportamento em navegador autenticado.
- Revisão visual autenticada em desktop/tablet/mobile permanece pendente; não foram fabricadas capturas.
