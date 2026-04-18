> Voltar para [README](../../project/README.pt-br.md)

# Canal Tencent Yuanbao

O PicoClaw suporta conexão ao Tencent Yuanbao como um canal usando a API oficial do Yuanbao Bot via WebSocket.

## O que este canal suporta

- Entrega de mensagens diretas e de grupo
- Comunicação em tempo real baseada em WebSocket com Yuanbao
- Processamento de mensagens de texto
- Configuração de acionamento de grupo (modo apenas menção)
- Filtragem por lista de permitidos de remetentes
- Roteamento de saída de raciocínio para uma conversa separada

> Nenhuma URL de callback webhook pública é necessária. O PicoClaw estabelece uma conexão WebSocket de saída para o servidor do Yuanbao.

---

## Início Rápido

### Pré-requisitos

Você precisa obter as credenciais do seu bot Yuanbao:
- **App ID** (`app_id`)
- **App Secret** (`app_secret`)

### Configuração

Adicione o seguinte ao seu `config.json` sob `channel_list`:

```json
{
  "channel_list": {
    "yuanbao": {
      "enabled": true,
      "type": "yuanbao",
      "app_id": "YOUR_APP_ID",
      "app_secret": "YOUR_APP_SECRET",
      "allow_from": [],
      "group_trigger": {},
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
| `enabled` | bool | `false` | Habilitar o canal Yuanbao. |
| `app_id` | string | — | O App ID do seu aplicativo Yuanbao. Necessário quando habilitado. |
| `app_secret` | string | — | O App Secret do seu aplicativo Yuanbao. Armazenado criptografado em `.security.yml`. Necessário quando habilitado. |
| `allow_from` | array | `[]` | Lista de permitidos de remetentes. Vazio significa permitir todos. |
| `group_trigger` | object | `{}` | Configurações de acionamento de grupo. |
| `reasoning_channel_id` | string | `""` | ID de conversa opcional para rotear saída de raciocínio/pensamento para uma conversa separada. |

### Configuração de Acionamento de Grupo

```json
{
  "group_trigger": {
    "mention_only": true
  }
}
```

| Campo | Tipo | Padrão | Descrição |
| ----- | ---- | ------- | ----------- |
| `mention_only` | bool | `true` | Quando true, o bot responde apenas quando mencionado em chats de grupo. |

### Variáveis de Ambiente

Todos os campos podem ser sobrescritos via variáveis de ambiente com o prefixo `PICOCLAW_CHANNELS_YUANBAO_`:

| Variável de Ambiente | Campo Correspondente |
| -------------------- | ------------------- |
| `PICOCLAW_CHANNELS_YUANBAO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_YUANBAO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_YUANBAO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## Comportamento em Runtime

- O PicoClaw mantém uma conexão WebSocket ativa com os servidores do Yuanbao.
- Mensagens de texto recebidas são processadas pelo agente e as respostas são enviadas via API do Yuanbao.
- Mensagens diretas são enviadas diretamente ao usuário.
- Mensagens de grupo são enviadas ao chat de grupo.
- Mensagens duplicadas são detectadas e suprimidas.

---

## Solução de Problemas

### Conexão falha

- Verifique se `app_id` e `app_secret` estão corretos.
- Certifique-se de que seu aplicativo Yuanbao tem as permissões necessárias habilitadas.
- Verifique se seu servidor pode alcançar o endpoint WebSocket do Yuanbao.

### Mensagens não chegam

- Verifique se `allow_from` está bloqueando o remetente.
- Certifique-se de que `channels.yuanbao.enabled` está definido como `true`.
- Verifique se `app_id` e `app_secret` não estão vazios.
- Para chats de grupo, certifique-se de que `group_trigger.mention_only` está configurado corretamente.
