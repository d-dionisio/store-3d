# Homologação Bambu Lab A1 — Etapa 10.8.1

Data da execução simulada: 17 de novembro de 2026.

## Evidência simulada

Broker controlado: Mochi MQTT v2 em loopback, MQTT 3.1.1 sobre TLS 1.2+, certificado e credenciais exclusivos de teste.

- conexão autenticada e assinatura QoS 1 confirmadas;
- certificado pinado correto aceito e fingerprint diferente rejeitado;
- credencial inválida rejeitada;
- peer TLS sem resposta encerrado por timeout;
- interrupção do broker seguida de reconexão e nova telemetria;
- payload parcial e campo desconhecido aceitos sem inventar dados;
- mensagens duplicadas toleradas;
- duas impressoras/tópicos monitorados isoladamente;
- cancelamento encerra os monitores de forma graciosa;
- hook ACL confirmou zero publicações do cliente monitor em tópicos operacionais.

Os testes usam certificados, códigos e números seriais efêmeros. Nenhuma credencial física foi usada.

## Homologação física

Status: `physical_homologation_pending`.

Checklist reproduzível, a executar presencialmente com uma A1 sem AMS Lite:

1. Registrar localmente modelo e versão de firmware sem copiar serial completo para relatórios.
2. Confirmar rede Wi-Fi local isolada, IP privado reservado e porta 8883 inacessível fora da LAN.
3. Ativar LAN Only Mode segundo a documentação oficial. Não ativar Developer Mode para esta leitura passiva.
4. Obter IP, código LAN e serial diretamente no equipamento e inseri-los somente no formulário protegido; nunca em chat, ticket ou log.
5. Capturar o SHA-256 do certificado apresentado na LAN por canal administrativo confiável e comparar antes de salvar.
6. Validar idle, impressão iniciada manualmente, progresso/temperaturas quando enviados, conclusão e posterior liberação física pelo operador.
7. Desconectar e restaurar a rede, confirmando estado desatualizado durante a queda e retomada posterior.
8. Conferir que nenhum job, run ou comando da impressora foi criado automaticamente.

Developer Mode desativa parte dos controles de autorização do MQTTs para terceiros. Caso um firmware futuro impeça até a assinatura somente leitura, a decisão de habilitá-lo exige avaliação de segmentação da LAN, explicação prévia e autorização explícita do operador.

## Homologação visual

Status: pendente de sessão administrativa MFA e navegador real autorizado.

Em 1440, 820, 390 e 320 px, verificar cartões sem overflow, formulário preenchido com host/porta/serial/fingerprint atuais, segredo sempre vazio, conflito 409 compreensível, teclado/foco e estados online, desatualizado, ocupado e indisponível. Nenhuma captura foi declarada como evidência real nesta execução.

## Limitações

- O formato dos tópicos e payloads não possui contrato público oficial estável.
- A aplicação apenas observa mensagens espontâneas; não publica `pushall`, comandos ou controle remoto.
- Campos que o firmware não enviar permanecem nulos.
- Compatibilidade física depende da versão efetivamente instalada na A1 e permanece não comprovada até a execução do checklist.
