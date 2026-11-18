# Checklist final — homologação do motor de custos (Etapa 10.10.1)

Data da consolidação: 2026-11-18. Esta consolidação é documental; não executou SQL, migrations, testes adicionais, autenticação, nem operações em bancos.

## Classificação por requisito

| Requisito | Resultado | Evidência | Pendência/limite |
|---|---|---|---|
| Integridade do banco | APROVADO PARCIALMENTE | Pré-verificação reportada: PostgreSQL 17.6, `loja_3d_test`, migration 28, `dirty=false`. Migration 28 passou up/down/up em teste; fixtures dos testes foram revertidas. | Não houve comparação de dados comerciais antes/depois nesta homologação. A concorrência foi exercitada em schema efêmero clonado sem foreign keys; FKs não foram re-homologadas nesse teste. Fase 4 não reconectou ao PostgreSQL. |
| Concorrência | APROVADO | Teste PostgreSQL real com dois pools e backends independentes, lock waits observados, disputa de versão de perfil e geração concorrente de snapshot. | O teste concorrente usa schema isolado sem foreign keys; ver integridade. |
| Cálculos financeiros | APROVADO | Teste PostgreSQL de custo real e testes Go: componentes, arredondamento half-up, custo total/unitário, margem, lucro, energia, consumo e desperdício; regressões direcionadas passaram. | Parâmetros elétricos e rateios continuam dependendo de medições/configuração operacional; não são validação contábil/fiscal. |
| Snapshots históricos | APROVADO | Snapshots preservaram preço/perfil usados após mudança de valores atuais; trigger append-only rejeitou alteração e teste comprovou ausência de linha parcial. | Não recalcular ou editar snapshots históricos. |
| Idempotência | APROVADO | Chamadas concorrentes para execução/tipo de snapshot retornaram o mesmo identificador; constraint única rejeitou duplicidade. | A operação de ciclo de vida que cria run e snapshot segue transações separadas; falha não deixa snapshot parcial, mas não reverte run pré-existente. |
| Rollback | APROVADO | Rollback transacional de fixtures, falhas de validação e rejeição do snapshot sem linha parcial foram exercitados no PostgreSQL de teste/schema isolado. | Fase 4 não executou testes PostgreSQL adicionais. |
| RBAC | APROVADO PARCIALMENTE | Teste da aplicação e testes HTTP automatizados: anônimo 401; `quote_viewer`/`production_operator` 403 nas leituras testadas; `production_manager`/`admin` autorizados nas leituras. | Sessão administrativa real não foi utilizada. A cobertura HTTP é automatizada com repositórios/sessão injetados e não substitui E2E. |
| CSRF | APROVADO PARCIALMENTE | Testes de handler rejeitaram token ausente/inválido com 403 e confirmaram ausência de chamada de mutação. | Não validado ponta a ponta em navegador com cookie/sessão MFA reais. Rejeições precoces não são registradas pelos handlers de custo na auditoria atual. |
| Reautenticação | APROVADO PARCIALMENTE | Sem serviço/evidência de reautenticação, as mutações falharam fechadas (503) antes da gravação. | Não foram comprovados com sessão real emissão, expiração, consumo único ou replay de evidência. |
| MFA | PENDENTE | Revisão estática observou que sessão administrativa autenticada exige `authentication_level='mfa'`. | Nenhuma sessão legítima com MFA foi usada nos testes de homologação; não houve teste real de login, TOTP ou cookie. |
| Auditoria | APROVADO PARCIALMENTE | Teste automatizado observou chamada de auditoria para leitura autorizada e ação sensível esperada. | Durabilidade não foi comprovada. O handler ignora erros de gravação de auditoria e retornos precoces por falha de CSRF/reautenticação não são auditados pelo handler. |
| Regressão backend | APROVADO | `go test ./...`, `go vet ./...` e teste direcionado de custos passaram na Fase 4. | URLs de banco ausentes intencionalmente: testes PostgreSQL opt-in/destrutivos foram ignorados nesta fase; a evidência PG vem das Fases 1/2 anteriores. |
| Regressão frontend | APROVADO | Lint, 11 arquivos/32 testes, `npx tsc --noEmit` e build Next.js passaram. | Vitest/build precisaram repetir fora do sandbox após `spawn EPERM`; não afeta o resultado final aprovado. |
| Homologação visual | PENDENTE | Build confirmou que rotas administrativas de produção/custos são compiladas. | Nenhuma inspeção visual autenticada em navegador nas resoluções-alvo foi feita nesta fase. |

## Evidência de regressão da Fase 4

- Backend: `go test ./...` — PASS; `go vet ./...` — PASS; regressões direcionadas dos pacotes de domínio/aplicação/infraestrutura de custos — PASS. `DATABASE_URL` e `TEST_DATABASE_URL` permaneceram ausentes; não houve conexão PostgreSQL nesta fase.
- Frontend: `npm run lint` — PASS; `npm test -- --run` — PASS (11 arquivos, 32 testes); `npx tsc --noEmit` — PASS; `npm run build` — PASS.
- Nenhuma migration foi executada. Nenhum pedido, pagamento, produto, catálogo ou snapshot comercial foi consultado ou modificado nesta fase. Não foi criada fixture persistente.
- A revisão estática de testes encontrou integrações antigas com `DELETE` amplo. Elas não receberam DSN e ficaram opt-out; não foram executadas contra banco compartilhado.

## Pendências de segurança obrigatórias

1. Obter sessão administrativa legítima com MFA e validar o fluxo real de login/sessão/cookie sem compartilhar credenciais com o relatório.
2. Homologar ponta a ponta a reautenticação, incluindo evidência expirada, uso único/replay e falhas, em ambiente isolado e com sessão legítima.
3. Corrigir em uma etapa autorizada a política de erro de gravação da auditoria (hoje descartado pelo handler) e registrar tentativas negadas por CSRF/reautenticação; então testar persistência e resposta fail-closed.
4. Fazer revisão visual autenticada das páginas administrativas nas resoluções suportadas.
5. Reexecutar testes PostgreSQL em banco `_test` após revisar isolamento dos testes destrutivos; não usar `loja_3d`.

## Conclusão formal

- O motor de custos está tecnicamente implementado? **Sim**, domínio, persistência, endpoints e regras foram implementados; a interface administrativa de custos descrita como planejada não foi construída nesta etapa.
- A integridade financeira foi validada? **Parcialmente**: cálculos, snapshots, constraints e rollback foram exercitados em PostgreSQL de teste; não há validação contábil/fiscal nem comprovação de FKs no schema concorrente clonado.
- A concorrência PostgreSQL foi homologada? **Sim**, com dois pools independentes e espera real por locks observada.
- A regressão automatizada passou? **Sim**, backend e frontend, conforme resultados acima.
- A segurança ponta a ponta foi homologada? **Não**: sessão/MFA real, reautenticação real e auditoria durável/cobertura de negativas não foram homologadas.
- A Etapa 10.10.1 pode ser considerada integralmente homologada? **Não.** A classificação global é parcial até fechar as pendências de segurança e revisão visual listadas acima.

Próxima etapa recomendada: correção e homologação ponta a ponta de auditoria, sessão administrativa MFA e reautenticação, seguida da inspeção visual autenticada. A recomendação não autoriza iniciar esse trabalho sem nova solicitação.
