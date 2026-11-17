# Integração Bambu Lab LAN

A Bambu Lab documenta oficialmente LAN Only Mode com protocolo local, servidor MQTTs interno e FTPS. Não há contrato público oficial para tópicos e JSON; o componente de rede do Bambu Studio é fechado. O provedor `bambulab_lan` é, portanto, experimental e tolerante a campos ausentes.

Esta implementação é estritamente somente leitura: conecta por MQTT 3.1.1 sobre TLS e assina `device/{serial}/report`. Não publica em `request`, não solicita snapshots, não inicia, pausa ou cancela impressão e não usa FTPS ou nuvem.

## Configuração da A1

1. Confirme o firmware instalado e ative o modo LAN conforme a documentação oficial. Não habilite Developer Mode para esta integração somente leitura.
2. Reserve o IP no DHCP; não exponha 8883 publicamente.
3. Confira o SHA-256 do certificado apresentado pela porta MQTTs.
4. Configure `BAMBU_LAN_ENCRYPTION_KEY` com 32 bytes em Base64 e `BAMBU_LAN_KEY_VERSION=1`.
5. Teste e salve em `/admin/producao/impressoras`.
6. Execute `go run ./cmd/printer-monitor`.

O código LAN é cifrado por AES-256-GCM. Apenas IP literal privado/loopback é aceito, bloqueando DNS e destinos públicos. A pinagem SHA-256 evita confiança cega no certificado privado. Nenhuma conexão real com uma A1 foi executada nesta implementação.

## Rotação da chave

1. Gere uma nova chave de 32 bytes, aumente `BAMBU_LAN_KEY_VERSION` e mantenha a versão anterior somente em `BAMBU_LAN_DECRYPTION_KEYS=1:<chave-antiga>`.
2. Execute `go run ./cmd/bambu-key` para uma verificação transacional com rollback.
3. Execute `go run ./cmd/bambu-key --execute` em janela operacional autorizada.
4. Reinicie API e worker com o keyring ainda contendo a chave anterior; confirme que todas as integrações usam a versão ativa.
5. Remova a chave anterior somente depois da verificação e de um backup restaurável.

O comando bloqueia as linhas, decifra todas as versões antigas, recriptografa e verifica o resultado na mesma transação. É retomável e uma segunda execução não altera linhas já rotacionadas. Chaves e credenciais nunca são registradas.

Developer Mode remove controles de autorização do MQTTs para aplicações de terceiros e transfere ao operador a responsabilidade de proteger a rede local. Ele não é necessário para o monitoramento passivo aqui implementado e só pode ser habilitado após explicação dos riscos e autorização explícita.

