# Monitoramento de impressoras

`cmd/printer-monitor` é independente da API. Ele carrega configurações habilitadas, limita conexões, mantém uma rotina controlada por impressora, reconecta com backoff e encerra por contexto.

O estado atual é sobrescrito em `printer_current_telemetry`; temperaturas não formam uma série infinita. Eventos de conexão são deduplicados e retidos por 30 dias. Trabalhos observados são agregados sem criar ou alterar jobs/runs.

Telemetria vence em 45 segundos. Depois disso, a interface mostra estado desatualizado e o planejamento não a trata como disponibilidade real. Estado observado, operacional e estimado permanecem separados.

Quando presentes, são lidos estado, progresso, temperaturas, velocidade, nome do trabalho e erro. Campos desconhecidos são ignorados; ausentes permanecem nulos. O protocolo de payload não é uma API pública estável da fabricante.

