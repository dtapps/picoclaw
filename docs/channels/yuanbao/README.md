> Back to [README](../../../README.md)

# Tencent Yuanbao Channel

PicoClaw supports connecting to Tencent Yuanbao as a channel using the official Yuanbao Bot API via WebSocket.

## What This Channel Supports

- Direct message and group chat delivery
- WebSocket-based real-time communication with Yuanbao
- Text message handling
- Group trigger configuration (mention-only mode)
- Sender allowlist filtering
- Reasoning output routing to a separate conversation

> No public webhook callback URL is required. PicoClaw establishes an outbound WebSocket connection to Yuanbao's server.

---

## Quick Start

### Prerequisites

You need to obtain your Yuanbao Bot credentials:
- **App ID** (`app_id`)
- **App Secret** (`app_secret`)

### Configuration

Add the following to your `config.json` under `channel_list`:

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

Then start the gateway:

```bash
picoclaw gateway
```

---

## Configuration

| Field | Type | Default | Description |
| ----- | ---- | ------- | ----------- |
| `enabled` | bool | `false` | Enable the Yuanbao channel. |
| `app_id` | string | — | Your Yuanbao application App ID. Required when enabled. |
| `app_secret` | string | — | Your Yuanbao application App Secret. Stored encrypted in `.security.yml`. Required when enabled. |
| `allow_from` | array | `[]` | Sender allowlist. Empty means allow all senders. |
| `group_trigger` | object | `{}` | Group trigger settings. |
| `reasoning_channel_id` | string | `""` | Optional chat ID to route reasoning/thinking output to a separate conversation. |

### Group Trigger Configuration

```json
{
  "group_trigger": {
    "mention_only": true
  }
}
```

| Field | Type | Default | Description |
| ----- | ---- | ------- | ----------- |
| `mention_only` | bool | `true` | When true, the bot only responds when mentioned in group chats. |

### Environment Variables

All fields can be overridden via environment variables with the prefix `PICOCLAW_CHANNELS_YUANBAO_`:

| Environment Variable | Corresponding Field |
| -------------------- | ------------------- |
| `PICOCLAW_CHANNELS_YUANBAO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_YUANBAO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_YUANBAO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## Runtime Behavior

- PicoClaw maintains an active WebSocket connection to Yuanbao's servers.
- Incoming text messages are processed by the agent and responses are sent back via the Yuanbao API.
- Direct messages are sent directly to the user.
- Group messages are sent to the group chat.
- Duplicate messages are detected and suppressed.

---

## Troubleshooting

### Connection fails

- Verify `app_id` and `app_secret` are correct.
- Ensure your Yuanbao application has the required permissions enabled.
- Check that your server can reach Yuanbao's WebSocket endpoint.

### Messages not arriving

- Check whether `allow_from` is blocking the sender.
- Ensure `channels.yuanbao.enabled` is set to `true`.
- Verify that `app_id` and `app_secret` are non-empty.
- For group chats, ensure `group_trigger.mention_only` is configured correctly.
