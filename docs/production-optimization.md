# Motor consultivo de otimização da produção

O motor da Etapa 10.6 é determinístico, reproduzível e estritamente somente leitura. Ele não altera jobs, impressoras, bobinas, pedidos ou pagamentos e não chama funções de transição.

## Compatibilidade

O PostgreSQL fornece dimensões da variante e volume útil cadastrado da impressora. O resultado é:

- `incompatible`: impressora indisponível ou volume útil comprovadamente excedido;
- `insufficient_data`: dimensões da variante ausentes;
- `compatible_with_caveats`: requisitos conhecidos são atendidos, mas há uma ressalva técnica explícita;
- `compatible`: especificações verificadas, configuração instalada e requisitos técnicos comprovadamente compatíveis.

Ausência de dados nunca é convertida em incompatibilidade.

## Pontuação

| Critério | Pontos |
| --- | ---: |
| Compatível | 50 |
| Compatível com ressalvas | 35 |
| Dados insuficientes | 20 |
| Disponível | 20 |
| Ocupada | 5 |
| Carga conhecida | até 20, reduzindo 1 ponto por hora |
| Mesmo material e cor da carga ativa | 5 |
| Histórico alto / médio / baixo | 10 / 7 / 4 |

Empates são resolvidos pelo UUID da impressora, garantindo resultado estável. Impressoras incompatíveis permanecem visíveis como alternativas explicadas, mas não são recomendadas.

## Sequenciamento e simulação

Estratégias: `fifo`, `shortest_processing_time`, `earliest_due_date`, `material_color_grouping` e `combined`. A combinada prioriza prazo, prioridade operacional, material/cor e FIFO como desempate. O simulador distribui jobs pela disponibilidade estimada das impressoras e calcula atrasos potenciais, utilização relativa, trocas observáveis, jobs não alocados e estimativas ausentes. Nenhum custo temporal fictício é atribuído a uma troca.

O histórico considera apenas execuções concluídas, positivas e nos últimos 90 dias. A duração usa mediana contínua arredondada e informa quantidade de amostras/confiança.

## API

- `GET /api/v1/admin/production/optimization`: visão completa, recomendações e cinco cenários.
- `GET /api/v1/admin/production/optimization/recommendations`: recomendações por job.
- `POST /api/v1/admin/production/optimization/simulate`: recalcula uma estratégia; exige sessão MFA e CSRF, mas não persiste dados.

Exemplo:

```json
{"strategy":"combined"}
```

Todas as rotas exigem `admin`, `production_manager` ou `production_operator`. `quote_viewer` é recusado.

## Limitações

- capacidades só são confiáveis após cadastro com fonte e estado `verified`;
- bobinas sem instalação registrada permanecem explicitamente desconhecidas;
- dimensões ausentes e histórico insuficiente são explicitados;
- utilização é uma projeção relativa sobre a carga conhecida, não telemetria;
- a recomendação necessita aprovação humana e não possui endpoint de aplicação automática.

Desde a Etapa 10.7, o snapshot considera material suportado, limites de temperatura, ambiente fechado, bico e superfície instalados e bobinas fisicamente carregadas. Os novos critérios refinam a compatibilidade e o bônus de material/cor; não alteram as cinco estratégias nem executam decisões automaticamente.

## Evidência PostgreSQL

O teste de integração somente leitura executa quatro snapshots concorrentes, compara contagens antes/depois e rejeita qualquer banco que não termine em `_test`. Em 14 de novembro de 2026, no banco `loja_3d_production_test`, o `EXPLAIN (ANALYZE, BUFFERS)` da seleção operacional apresentou `Planning Time: 0.652 ms` e `Execution Time: 0.207 ms`. Com 27 jobs no ambiente, o PostgreSQL escolheu sequential scans nas tabelas pequenas; não houve evidência para migration ou índice adicional.
