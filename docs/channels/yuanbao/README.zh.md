> 返回 [README](../../../README.zh.md)

# 腾讯元宝频道

PicoClaw 支持通过腾讯官方元宝 Bot API over WebSocket 连接元宝作为频道。

## 支持的功能

- 私聊和群聊消息收发
- 基于 WebSocket 的实时通信
- 文本消息处理
- 群聊触发配置（仅@提及模式）
- 发送者白名单过滤
- 推理输出路由到独立会话

> 无需公网 Webhook 回调地址。PicoClaw 主动向元宝服务器建立出站 WebSocket 连接。

---

## 快速开始

### 前置要求

你需要获取元宝机器人的凭证：
- **App ID** (`app_id`)
- **App Secret** (`app_secret`)

### 配置

在 `config.json` 的 `channels` 部分添加以下内容：

```json
{
  "channels": {
    "yuanbao": {
      "enabled": true,
      "app_id": "YOUR_APP_ID",
      "app_secret": "YOUR_APP_SECRET",
      "allow_from": [],
      "group_trigger": {},
      "reasoning_channel_id": ""
    }
  }
}
```

然后启动网关：

```bash
picoclaw gateway
```

---

## 配置项说明

| 字段 | 类型 | 默认值 | 说明 |
| ---- | ---- | ------ | ---- |
| `enabled` | bool | `false` | 启用元宝频道。 |
| `app_id` | string | — | 元宝应用 App ID。启用时必填。 |
| `app_secret` | string | — | 元宝应用 App Secret。加密存储于 `.security.yml`。启用时必填。 |
| `allow_from` | array | `[]` | 发送者白名单。为空时允许所有人。 |
| `group_trigger` | object | `{}` | 群聊触发设置。 |
| `reasoning_channel_id` | string | `""` | 可选，将推理/思考内容路由到指定会话 ID。 |

### 群触发配置

```json
{
  "group_trigger": {
    "mention_only": true
  }
}
```

| 字段 | 类型 | 默认值 | 说明 |
| ----- | ---- | ------- | ----------- |
| `mention_only` | bool | `true` | 为 true 时，机器人在群聊中仅在被 @ 时响应。 |

### 环境变量

所有字段均可通过 `PICOCLAW_CHANNELS_YUANBAO_` 前缀的环境变量覆盖：

| 环境变量 | 对应字段 |
| -------- | -------- |
| `PICOCLAW_CHANNELS_YUANBAO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_YUANBAO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_YUANBAO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## 运行时行为

- PicoClaw 与元宝服务器保持活跃的 WebSocket 连接。
- 收到的文本消息由 Agent 处理，并通过元宝 API 发送回复。
- 私信直接发送给用户。
- 群消息发送到群聊。
- 重复消息会被检测并抑制。

---

## 故障排除

### 连接失败

- 确认 `app_id` 和 `app_secret` 是否正确。
- 确保你的元宝应用已启用所需权限。
- 检查服务器能否访问元宝的 WebSocket 端点。

### 消息未收到

- 检查 `allow_from` 是否阻止了发送者。
- 确保 `channels.yuanbao.enabled` 设置为 `true`。
- 验证 `app_id` 和 `app_secret` 非空。
- 对于群聊，确保 `group_trigger.mention_only` 配置正确。
