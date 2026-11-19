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

As rotas de custo usam somente a auditoria administrativa existente. Atualizações de perfil e gravação/idempotência de snapshots inserem o evento de sucesso na mesma transação PostgreSQL da mutação; falha de auditoria aborta o commit. Leituras sensíveis e operações recusadas só retornam a resposta normal depois de persistir auditoria; falha do sink resulta em resposta genérica 503. Recusas registram eventos de vocabulário fechado para sessão, CSRF e reautenticação, sem copiar credenciais ou mensagens de erro. A homologação PostgreSQL da atomicidade e concorrência passou posteriormente no PowerShell do operador em `loja_3d_test`, migration 28 (`dirty=false`); evidência detalhada em `docs/evidence/manufacturing-costs-homologation.md`.

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

## Etapa 10.10.2.2 — reautenticação administrativa

O fluxo existente permanece baseado em sessão administrativa MFA, senha + TOTP/código de recuperação, CSRF e evidência opaca de uso único enviada apenas no header da operação sensível. A evidência é persistida como hash e seu TTL continua configurável por `AUTH_REAUTH_TTL` (padrão 5 minutos, máximo aceito 15 minutos). Não foi criado login, cookie ou mecanismo paralelo.

A revisão encontrou uma lacuna concreta no repositório: o consumo verificava vínculo do hash à sessão e expiração da evidência, mas não revalidava a sessão e a conta no mesmo commit. O repositório agora bloqueia identidade e sessão durante validação/consumo, em ordem alinhada à desativação administrativa, e exige sessão ativa, não expirada, usuário ativo e `authentication_level='mfa'` para administradores. A emissão também valida a propriedade da sessão e exige representação de prova TOTP/código de recuperação antes de inserir a evidência. Falhas de persistência são propagadas pelo repositório; a camada de serviço continua respondendo de forma fechada e sem revelar detalhes internos.

Teste unitário direcionado de aplicação passou: MFA administrativo continua obrigatório e a expiração capturada respeita o TTL configurado. `TestAdminReauthenticationPostgreSQL` passou em `loja_3d_test` (PostgreSQL 17.6, migration 28 limpa): expiração, replay, sessão/usuário incorretos, sessão revogada/expirada, corrida com conexões independentes (um único consumo) e falha induzida no UPDATE com rollback. A primeira execução havia encontrado uma constraint temporal na fixture; ela foi corrigida e a repetição passou. Esses resultados homologam persistência/repositório, não uma sessão humana/MFA ponta a ponta.

Não havia sessão administrativa legítima com MFA disponível para este agente. Portanto, login/MFA/cookie/CSRF e uso real da evidência em navegador permanecem pendentes de homologação ponta a ponta; testes com repositório/sessão de fixture não substituem essa evidência. O comando guardado para a execução PostgreSQL e a classificação atual estão em `docs/evidence/manufacturing-costs-homologation.md`.

## Etapa 10.10.2.3 — MFA e sessão administrativa

A inspeção confirmou o fluxo existente: senha → desafio MFA → TOTP/código de recuperação → sessão PostgreSQL com `authentication_level='mfa'`. O cookie `loja3d_admin_session` é HTTP-only, Secure em produção e SameSite Strict. Logout exige CSRF, revoga a sessão e grava auditoria administrativa. A evidência de reautenticação continua vinculada à sessão e é consumida antes das operações sensíveis.

Foi corrigida uma falha concreta: erros inesperados de `GetFactor` eram tratados como ausência de fator. No admin isso podia iniciar sobrescrita/reinscrição do MFA; no cliente podia degradar a sessão para autenticação somente por senha. Agora apenas `ErrUnauthorized` (fator inexistente) segue o ramo de setup/login opcional; falhas de infraestrutura retornam indisponibilidade. Desafios e eventos de emissão são persistidos juntos; sucesso MFA e sessão são registrados na mesma transação. Eventos de tentativa não armazenam senhas, códigos, tokens ou segredo MFA.

Testes direcionados da aplicação/handlers, compilação do pacote PostgreSQL e `go vet ./...` passaram localmente. O operador executou `TestAdminMFASessionPostgreSQL` no PowerShell: **PASS**, teste 1,14 s, pacote 1,634 s, com guardas aprovadas e schema efêmero exclusivamente em `loja_3d_test`. A validação com conta real e navegador continua pendente; veja `docs/evidence/manufacturing-costs-homologation.md`.

### Etapa 10.10.2.4 — homologação visual

Inspeção visual autenticada **pendente**: não havia navegador de automação acessível e as portas locais do frontend/API não estavam em escuta. Nenhuma sessão foi injetada, serviço iniciado, captura produzida ou correção visual presumida. As rotas de produção existem no código, mas não foram visualmente avaliadas. Também não foram encontradas páginas frontend para custos por impressora, estimativa/realizado ou rentabilidade. Detalhes e procedimento manual estão em `docs/evidence/production-visual-homologation.md`.

### Interface administrativa — Etapa 10.10.2.4.1

A interface frontend foi implementada nas rotas `/admin/producao/custos`, `/custos/impressoras`, `/custos/execucoes` e `/custos/rentabilidade`. Ela consome perfis versionados, histórico, snapshots por execução e relatórios de rentabilidade/comparação já existentes; a API continua autoridade para cálculos e autorização. Valores continuam em centavos nos DTOs e são apenas formatados para BRL na apresentação.

Salvar perfil envia `expected_version`; conflito HTTP 409 orienta recarregar. Toda mutação de perfil ou criação de snapshot pede senha e MFA/recovery no diálogo de reautenticação já utilizado pelo painel e envia CSRF e `X-Reauthentication-Token`. A evidência é transitória em memória. Estimativa e realizado criam snapshots imutáveis: a interface confirma essa consequência antes do POST e não disponibiliza edição/exclusão.

Limites contratuais preservados: o endpoint geral `/analytics/costs` oferece custos de material e métricas operacionais, não custo total de fabricação; por isso não é rotulado como custo total. Rentabilidade e comparação usam endpoints financeiros próprios. Não existe endpoint de estimativa sem persistência: gerar estimativa cria snapshot. Não é presumida potência elétrica; ausência de dados suficientes é comunicada pela API. A nova interface ainda requer homologação visual com sessão administrativa legítima e backend com dados adequados.
