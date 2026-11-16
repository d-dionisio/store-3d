# Integração Bambu Lab LAN

A Bambu Lab documenta oficialmente LAN Only Mode com protocolo local, servidor MQTTs interno e FTPS. Não há contrato público oficial para tópicos e JSON; o componente de rede do Bambu Studio é fechado. O provedor `bambulab_lan` é, portanto, experimental e tolerante a campos ausentes.

Esta implementação é estritamente somente leitura: conecta por MQTT 3.1.1 sobre TLS e assina `device/{serial}/report`. Não publica em `request`, não solicita snapshots, não inicia, pausa ou cancela impressão e não usa FTPS ou nuvem.

## Configuração da A1

1. Ative o acesso LAN e anote IP privado, serial e código LAN.
2. Reserve o IP no DHCP; não exponha 8883 publicamente.
3. Confira o SHA-256 do certificado apresentado pela porta MQTTs.
4. Configure `BAMBU_LAN_ENCRYPTION_KEY` com 32 bytes em Base64 e `BAMBU_LAN_KEY_VERSION=1`.
5. Teste e salve em `/admin/producao/impressoras`.
6. Execute `go run ./cmd/printer-monitor`.

O código LAN é cifrado por AES-256-GCM. Apenas IP literal privado/loopback é aceito, bloqueando DNS e destinos públicos. A pinagem SHA-256 evita confiança cega no certificado privado. Nenhuma conexão real com uma A1 foi executada nesta implementação.

