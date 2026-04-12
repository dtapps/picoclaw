> 返回 [README](../../../README.zh.md)

# 微博频道

PicoClaw 支持通过微博官方 API over WebSocket 连接微博作为频道。

## 支持的功能

- 通过微博接收和发送私信
- 基于 WebSocket 的实时通信
- 文本消息处理
- 发送者白名单过滤
- 推理输出路由到独立会话

> 无需公网 Webhook 回调地址。PicoClaw 主动向微博服务器建立出站 WebSocket 连接。

---

## 快速开始

### 获取凭证

1. 打开微博客户端（手机 App 或网页版）
2. 给 **@微博龙虾助手** 发送私信
3. 发送消息：**连接龙虾**
4. 你将收到包含凭证的回复：

```
您的应用凭证信息如下：

AppId: your-app-id
AppSecret: your-app-secret
```

> 如需重置凭证，请发送消息"重置凭证"。

### 配置

在 `config.json` 的 `channels` 部分添加以下内容：

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

然后启动网关：

```bash
picoclaw gateway
```

---

## 配置项说明

| 字段 | 类型 | 默认值 | 说明 |
| ---- | ---- | ------ | ---- |
| `enabled` | bool | `false` | 启用微博频道。 |
| `app_id` | string | — | 微博应用 App ID。启用时必填。 |
| `app_secret` | string | — | 微博应用 App Secret。加密存储于 `.security.yml`。启用时必填。 |
| `allow_from` | array | `[]` | 发送者白名单。为空时允许所有人。 |
| `reasoning_channel_id` | string | `""` | 可选，将推理/思考内容路由到指定会话 ID。 |

### 环境变量

所有字段均可通过 `PICOCLAW_CHANNELS_WEIBO_` 前缀的环境变量覆盖：

| 环境变量 | 对应字段 |
| -------- | -------- |
| `PICOCLAW_CHANNELS_WEIBO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_WEIBO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_WEIBO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_WEIBO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_WEIBO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## 运行时行为

- PicoClaw 与微博服务器保持活跃的 WebSocket 连接。
- 收到的文本消息由 Agent 处理，并通过微博 API 发送回复。
- 收到的媒体文件会下载到本地媒体存储后再传给 Agent。
- 重复消息会被检测并抑制。

---

## 故障排除

### 连接失败

- 确认 `app_id` 和 `app_secret` 是否正确。
- 确保你的微博账号已获得授权。
- 检查服务器能否访问微博的 WebSocket 端点。

### 消息未收到

- 检查 `allow_from` 是否阻止了发送者。
- 确保 `channels.weibo.enabled` 设置为 `true`。
- 验证 `app_id` 和 `app_secret` 非空。

### 需要重置凭证

- 给 @微博龙虾助手 发送消息"重置凭证"来获取新凭证。
