# Especificações técnicas de impressoras

A migration 26 mantém o cadastro operacional em `printers` e adiciona capacidades documentadas, versionadas e independentes em `printer_technical_specs`. Fabricante, modelo, código, status e volume operacional existente não são duplicados.

## Separação dos dados

- `printer_technical_specs`: capacidade máxima documentada, fonte, estado `draft|verified|superseded` e versão imutável.
- `printer_supported_nozzles`, `printer_supported_materials`, `printer_build_surfaces` e `printer_features`: capacidades ligadas à versão da especificação.
- `printer_operational_configs`: bico, superfície e quantidade de slots atualmente configurados, com concorrência otimista.
- `spool_installations`: histórico físico de instalação e remoção. Uma bobina só pode estar ativa em uma impressora e um slot só recebe uma bobina ativa.

Uma especificação `verified` exige fonte. Valores desconhecidos permanecem `NULL`; nome ou modelo da impressora nunca gera capacidades presumidas. A troca de versão marca a anterior como `superseded`, preservando histórico.

## Operação

Antes de instalar uma bobina, configure e verifique a especificação da impressora, declare o material suportado e configure seus slots. Bobinas em quarentena, descartadas ou esgotadas são recusadas. A remoção encerra o registro; registros encerrados são imutáveis.

Configurações técnicas e operacionais usam versões esperadas. Resposta HTTP 409 indica edição concorrente e exige recarregar o estado atual.

