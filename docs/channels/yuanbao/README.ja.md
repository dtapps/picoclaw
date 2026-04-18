> [README](../../project/README.ja.md) に戻る

# 腾讯元宝チャンネル

PicoClawは腾讯公式元宝Bot API over WebSocket経由で元宝をチャンネルとして接続することをサポートします。

## サポートされている機能

- ダイレクトメッセージとグループチャット配信
- WebSocketベースのリアルタイム通信
- テキストメッセージ処理
- グループトリガー設定（メンションのみモード）
- 送信者ホワイトリストフィルタリング
- 推理出力を別の会話にルーティング

> パブリックWebhookコールバックURLは不要です。PicoClawは元宝サーバーへのアウトバウンドWebSocket接続を確立します。

---

## クイックスタート

### 前提条件

元宝Botの認証情報を取得する必要があります：
- **App ID** (`app_id`)
- **App Secret** (`app_secret`)

### 設定

`config.json`の`channel_list`セクションに以下を追加します：

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

次にgatewayを起動します：

```bash
picoclaw gateway
```

---

## 設定

| フィールド | 型 | デフォルト | 説明 |
| ----- | ---- | ------- | ----------- |
| `enabled` | bool | `false` | 元宝チャンネルを有効にするかどうか。 |
| `app_id` | string | — | 元宝アプリケーションのApp ID。有効時に必須。 |
| `app_secret` | string | — | 元宝アプリケーションのApp Secret。`.security.yml`に暗号化して保存。有効時に必須。 |
| `allow_from` | array | `[]` | 送信者ホワイトリスト。空の場合は全員を許可。 |
| `group_trigger` | object | `{}` | グループトリガー設定。 |
| `reasoning_channel_id` | string | `""` | 推理/思考出力を別の会話にルーティングするためのオプションの会話ID。 |

### グループトリガー設定

```json
{
  "group_trigger": {
    "mention_only": true
  }
}
```

| フィールド | 型 | デフォルト | 説明 |
| ----- | ---- | ------- | ----------- |
| `mention_only` | bool | `true` | trueの場合、Botはグループチャットでメンションされたときのみ応答します。 |

### 環境変数

すべてのフィールドは`PICOCLAW_CHANNELS_YUANBAO_`プレフィックスを持つ環境変数でオーバーライドできます：

| 環境変数 | 対応フィールド |
| -------------------- | ------------------- |
| `PICOCLAW_CHANNELS_YUANBAO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_YUANBAO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_YUANBAO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## ランタイム動作

- PicoClawは元宝サーバーとのアクティブなWebSocket接続を維持します。
- 受信テキストメッセージはエージェントによって処理され、応答は元宝API経由で送信されます。
- ダイレクトメッセージはユーザーに直接送信されます。
- グループメッセージはグループチャットに送信されます。
- 重複メッセージは検出され抑制されます。

---

## トラブルシューティング

### 接続に失敗する

- `app_id`と`app_secret`が正しいことを確認してください。
- 元宝アプリケーションが必要な権限を有効にしていることを確認してください。
- サーバーが元宝のWebSocketエンドポイントに到達できることを確認してください。

### メッセージが届かない

- `allow_from`が送信者をブロックしていないか確認してください。
- `channels.yuanbao.enabled`が`true`に設定されていることを確認してください。
- `app_id`と`app_secret`が空でないことを確認してください。
- グループチャットの場 合は、`group_trigger.mention_only`が正しく設定されていることを確認してください。
