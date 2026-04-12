> Voltar para [README](../../../README.pt-br.md)

# Canal Weibo

O PicoClaw suporta conexão ao Weibo como um canal usando a API oficial do Weibo over WebSocket.

## O que este canal suporta

- Recebimento e envio de mensagens diretas via Weibo
- Comunicação em tempo real baseada em WebSocket
- Processamento de mensagens de texto
- Filtragem por lista de permitidos de remetentes
- Roteamento de saída de raciocínio para uma conversa separada

> Nenhuma URL de callback webhook pública é necessária. O PicoClaw estabelece uma conexão WebSocket de saída para os servidores do Weibo.

---

## Início Rápido

### Obter Credenciais

1. Abra seu cliente Weibo (aplicativo móvel ou web)
2. Envie uma mensagem direta para **@微博龙虾助手**
3. Envie a mensagem: **连接龙虾**
4. Você receberá uma resposta com suas credenciais:

```
您的应用凭证信息如下：

AppId: your-app-id
AppSecret: your-app-secret
```

> Para redefinir as credenciais, envie a mensagem "重置凭证".

### Configuração

Adicione o seguinte ao seu `config.json` sob `channels`:

```json
{
  "channels": {
    "weibo": {
      "enabled": true,
      "app_id": "YOUR_APP_ID",
      "app_secret": "YOUR_APP_SECRET",
      "allow_from": [],
      "reasoning_channel_id": ""
    }
  }
}
```

Então inicie o gateway:

```bash
picoclaw gateway
```

---

## Configuração

| Campo | Tipo | Padrão | Descrição |
| ----- | ---- | ------ | ----------- |
| `enabled` | bool | `false` | Habilitar o canal Weibo. |
| `app_id` | string | — | O App ID do seu aplicativo Weibo. Necessário quando habilitado. |
| `app_secret` | string | — | O App Secret do seu aplicativo Weibo. Armazenado criptografado em `.security.yml`. Necessário quando habilitado. |
| `allow_from` | array | `[]` | Lista de permitidos de remetentes. Vazio significa permitir todos. |
| `reasoning_channel_id` | string | `""` | ID de conversa opcional para rotear saída de raciocínio/pensamento para uma conversa separada. |

### Variáveis de Ambiente

Todos os campos podem ser sobrescritos via variáveis de ambiente com o prefixo `PICOCLAW_CHANNELS_WEIBO_`:

| Variável de Ambiente | Campo Correspondente |
| -------------------- | ------------------- |
| `PICOCLAW_CHANNELS_WEIBO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_WEIBO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_WEIBO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_WEIBO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_WEIBO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## Comportamento em Runtime

- O PicoClaw mantém uma conexão WebSocket ativa com os servidores do Weibo.
- Mensagens de texto recebidas são processadas pelo agente e as respostas são enviadas via API do Weibo.
- Mídia recebida é baixada para o armazenamento local de mídia antes de ser passada ao agente.
- Mensagens duplicadas são detectadas e suprimidas.

---

## Solução de Problemas

### Conexão falha

- Verifique se `app_id` e `app_secret` estão corretos.
- Certifique-se de que sua conta Weibo foi autorizada.
- Verifique se seu servidor pode alcançar o endpoint WebSocket do Weibo.

### Mensagens não chegam

- Verifique se `allow_from` está bloqueando o remetente.
- Certifique-se de que `channels.weibo.enabled` está definido como `true`.
- Verifique se `app_id` e `app_secret` não estão vazios.

### Precisa redefinir credenciais

- Envie a mensagem "重置凭证" para @微博龙虾助手 para obter novas credenciais.
