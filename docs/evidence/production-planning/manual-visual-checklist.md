# Checklist manual — planejamento de produção

Este procedimento existe porque a execução de 13 de novembro de 2026 não dispunha de navegador automatizável. Não há screenshots sintéticas.

## Preparação segura

1. Inicie PostgreSQL, API e frontend com as configurações locais de desenvolvimento.
2. Confirme que operações destrutivas e fixtures usam exclusivamente um banco terminado em `_test`.
3. Entre pelo fluxo administrativo real e conclua MFA; não desative MFA ou CSRF.
4. Use uma conta `admin` ou `production_manager` e dados de homologação claramente identificados.

## Resoluções

Validar `/admin/producao`, `/admin/producao/planejamento`, fila, detalhes, impressoras e materiais em:

- desktop: 1440 px;
- tablet: 820 px;
- mobile: 390 px;
- mobile estreito: 320 px.

## Critérios

- cards globais permanecem consistentes ao paginar e filtrar;
- total por risco não muda apenas por trocar de página;
- filtros de status, impressora, vencimento e risco combinam corretamente;
- jobs concluídos/cancelados não aparecem no planejamento ativo sem filtro explícito;
- previsão apresenta origem, quantidade de amostras e confiança sem inventar duração quando não há histórico;
- alertas de atraso, capacidade e material insuficiente continuam legíveis;
- tabelas não geram overflow horizontal destrutivo; controles são acessíveis por teclado;
- loading, vazio, erro, conflito de versão e sessão expirada são distinguíveis;
- nenhuma mutação funciona sem CSRF e nenhuma ação privilegiada aparece como concluída antes da resposta do backend.

As capturas reais, quando produzidas, devem ser salvas neste diretório com resolução e data no nome e sem dados pessoais ou credenciais.
