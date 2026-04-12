> Back to [README](../../../README.md)

# Weibo Channel

PicoClaw supports connecting to Weibo as a channel using the Weibo Official API over WebSocket.

## What This Channel Supports

- Direct message receiving and sending via Weibo
- WebSocket-based real-time communication
- Text message handling
- Sender allowlist filtering
- Reasoning output routing to a separate conversation

> No public webhook callback URL is required. PicoClaw establishes an outbound WebSocket connection to Weibo's server.

---

## Quick Start

### Get Credentials

1. Open your Weibo client (mobile app or web)
2. Send a direct message to **@微博龙虾助手**
3. Send the message: **连接龙虾** (Connect Lobster)
4. You will receive a reply with your credentials:

```
您的应用凭证信息如下：

AppId: your-app-id
AppSecret: your-app-secret
```

> To reset credentials, send the message "重置凭证" (Reset Credentials).

### Configuration

Add the following to your `config.json` under `channels`:

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

Then start the gateway:

```bash
picoclaw gateway
```

---

## Configuration

| Field | Type | Default | Description |
| ----- | ---- | ------- | ----------- |
| `enabled` | bool | `false` | Enable the Weibo channel. |
| `app_id` | string | — | Your Weibo application App ID. Required when enabled. |
| `app_secret` | string | — | Your Weibo application App Secret. Stored encrypted in `.security.yml`. Required when enabled. |
| `allow_from` | array | `[]` | Sender allowlist. Empty means allow all senders. |
| `reasoning_channel_id` | string | `""` | Optional chat ID to route reasoning/thinking output to a separate conversation. |

### Environment Variables

All fields can be overridden via environment variables with the prefix `PICOCLAW_CHANNELS_WEIBO_`:

| Environment Variable | Corresponding Field |
| -------------------- | ------------------- |
| `PICOCLAW_CHANNELS_WEIBO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_WEIBO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_WEIBO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_WEIBO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_WEIBO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## Runtime Behavior

- PicoClaw maintains an active WebSocket connection to Weibo's servers.
- Incoming text messages are processed by the agent and responses are sent back via the Weibo API.
- Incoming media is downloaded into the local media store before being passed to the agent.
- Duplicate messages are detected and suppressed.

---

## Troubleshooting

### Connection fails

- Verify `app_id` and `app_secret` are correct.
- Ensure your Weibo account has been authorized.
- Check that your server can reach Weibo's WebSocket endpoint.

### Messages not arriving

- Check whether `allow_from` is blocking the sender.
- Ensure `channels.weibo.enabled` is set to `true`.
- Verify that `app_id` and `app_secret` are non-empty.

### Need to reset credentials

- Send the message "重置凭证" to @微博龙虾助手 to get new credentials.
