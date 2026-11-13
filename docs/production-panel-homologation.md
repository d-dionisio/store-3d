# Homologação operacional do painel de produção — Etapa 10.4.1

Data: 12/11/2026.

## Diagnóstico e reinicialização

O processo original em `:8080` era `api.exe` (PID 20488), artefato temporário de um `go run` iniciado em 11/11/2026. Ele respondia ao health check, mas devolvia `404` para `/api/v1/admin/production/jobs`. O código atual já criava `ProductionRepository`, `application/production.Service` e `ProductionHandler`, passando-o para `NewHandlerCompleteWithProduction`; portanto, o processo em memória correspondia a um build anterior.

Somente o par `go run`/`api.exe` foi reiniciado. PostgreSQL, volume e demais serviços não foram interrompidos. Depois do reinício:

- API vinculada a `127.0.0.1:8080`;
- `/api/v1/ready`: `200`, banco conectado;
- `/api/v1/admin/production/jobs` anônimo: `401` com erro JSON `unauthorized`, comprovando registro e proteção da rota.

## Banco isolado

- Desenvolvimento: `localhost:5432/loja_3d`.
- Homologação: `localhost:5432/loja_3d_production_test`.
- URLs diferentes; o banco de homologação termina em `_test` e não se chama `loja_3d`.
- Migration no banco de homologação: versão 25, `dirty=false`.
- O seed demonstrativo foi aplicado somente ao banco `_test`, pois a suíte histórica depende desses registros.
- Os dois jobs do banco de desenvolvimento não foram usados nem modificados.

## Cenários PostgreSQL

`TestProductionLifecycleIntegration` valida com aplicação e repositório reais:

- criação idempotente de jobs após liquidação integral;
- fila, preparação, atribuição de impressora, início, consumo, finalização, inspeção, aprovação e conclusão;
- falha e reimpressão;
- reprovação de qualidade e reimpressão;
- cancelamento permitido e transição inválida;
- conflito de versão;
- eventos append-only;
- cadastro/status/manutenção de impressora;
- cadastro e ajuste de bobina;
- saldo insuficiente, consumo concorrente, quarentena e snapshot de custo.

Fixtures de produção usam produto `draft`, mantendo as FKs reais sem contaminar consultas do catálogo comercial.

## HTTP, segurança e DTOs

`TestProductionHandlerAuthCSRFRoleAndDTOIntegration` valida:

- anônimo: `401`;
- `quote_viewer`: `403`;
- `admin`, `production_manager` e `production_operator`: leitura autorizada;
- ausência de CSRF em mutação: `403`;
- operador e visualizador impedidos de administrar recursos;
- administrador autorizado a cadastrar recurso;
- paginação e envelope JSON da fila;
- DTO de impressora criado.

Os testes de aplicação também confirmam que o operador não pode colocar job na fila, concluir job ou administrar impressoras. A API permanece a autoridade final de RBAC e transições.

## Resultados

- `go test ./...`: aprovado com PostgreSQL real `_test`.
- `go vet ./...`: aprovado.
- `npm run lint`: aprovado.
- `npm test`: 9 arquivos e 26 testes aprovados.
- `npx tsc --noEmit`: aprovado.
- `npm run build`: aprovado, incluindo as quatro rotas administrativas de produção.

## Homologação visual

Chrome e Edge estão instalados, mas o projeto não possui driver de automação de navegador. Como o painel exige cookie HTTP-only e MFA, não foi produzida captura do painel sem uma sessão administrativa real. Nenhuma imagem foi apresentada como evidência. A revisão autenticada em 1440, 820, 390 e 320 px permanece como procedimento manual.

## Limitações de contrato

A API ainda não expõe nomes de material/cor, variante, especificações, histórico de manutenção, inspeções detalhadas, consumos detalhados, tempos estimados/reais ou limite mínimo da bobina. O frontend não inventa esses valores. Indicadores globais também exigiriam endpoint agregado; o painel atual deriva métricas da página carregada.
