# Materiais e compatibilidade técnica

`material_technical_specs` complementa `materials` sem duplicar sua identidade. Fabricante, nome comercial, polímero, faixas de hotend/mesa, temperatura recomendada, velocidade, secagem, ambiente fechado, umidade e superfície podem ficar não configurados.

Dados `verified` exigem referência verificável. O sistema não fornece temperaturas iniciais para PLA, PETG, TPU, ABS, ASA ou Nylon sem uma fonte do fabricante.

## Classificação

- `compatible`: especificações de impressora e material verificadas, configuração atual conhecida e todos os requisitos avaliados.
- `compatible_with_caveats`: requisitos conhecidos são atendidos, mas há ressalvas específicas, como superfície diferente.
- `incompatible`: existe evidência objetiva de volume, material, bico, temperatura, ambiente ou disponibilidade incompatível.
- `insufficient_data`: faltam dimensões, especificações verificadas ou configuração atual. Ausência nunca significa compatibilidade.

Cada resposta contém `reasons` e `missing_data`. A avaliação é consultiva: não atribui impressora, não instala bobina e não inicia execução. A decisão de fabricação continua humana.

## Integração com otimização

O motor 10.6 usa capacidades verificadas, configuração atual e bobina instalada. Material e cor coincidentes podem melhorar a pontuação, mas não convertem dados ausentes em compatibilidade. As cinco estratégias de sequenciamento permanecem inalteradas.

