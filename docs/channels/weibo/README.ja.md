> [README](../../../README.ja.md) に戻る

# Weibo チャンネル

PicoClawはWeibo公式API over WebSocket経由でWeiboをチャンネルとして接続をサポートします。

## サポートされている機能

- Weibo経由のダイレクトメッセージ送受信
- WebSocketベースのリアルタイム通信
- テキストメッセージ処理
- 送信者ホワイトリストフィルタリング
- 推理出力を別の会話にルーティング

> パブリックWebhookコールバックURLは不要です。PicoClawはWeiboサーバーへのアウトバウンドWebSocket接続を確立します。

---

## クイックスタート

### 認証情報の取得

1. Weiboクライアント（モバイルアプリまたはWeb）を開く
2. **@微博龙虾助手**にダイレクトメッセージを送信
3. メッセージを送信：**接続龙虾**
4. 認証情報 포함한返信が届きます：

```
您的应用凭证信息如下：

AppId: your-app-id
AppSecret: your-app-secret
```

> 認証情報をリセットするには、"重置凭证"と送信してください。

### 設定

`config.json`の`channels`セクションに以下を追加します：

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

次にgatewayを起動します：

```bash
picoclaw gateway
```

---

## 設定

| フィールド | 型 | デフォルト | 説明 |
| ----- | ---- | ------- | ----------- |
| `enabled` | bool | `false` | Weiboチャンネルを有効にするかどうか。 |
| `app_id` | string | — | WeiboアプリケーションのApp ID。有効時に必須。 |
| `app_secret` | string | — | WeiboアプリケーションのApp Secret。`.security.yml`に暗号化して保存。有効時に必須。 |
| `allow_from` | array | `[]` | 送信者ホワイトリスト。空の場合は全員を許可。 |
| `reasoning_channel_id` | string | `""` | 推理/思考出力を別の会話にルーティングするためのオプションの会話ID。 |

### 環境変数

すべてのフィールドは`PICOCLAW_CHANNELS_WEIBO_`プレフィックスを持つ環境変数でオーバーライドできます：

| 環境変数 | 対応フィールド |
| -------------------- | ------------------- |
| `PICOCLAW_CHANNELS_WEIBO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_WEIBO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_WEIBO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_WEIBO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_WEIBO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## ランタイム動作

- PicoClawはWeiboサーバーとのアクティブなWebSocket接続を維持します。
- 受信テキストメッセージはエージェントによって処理され、応答はWeibo API経由で送信されます。
- 受信メディアはエージェントに渡される前にローカルメディアストアにダウンロードされます。
- 重複メッセージは検出され抑制されます。

---

## トラブルシューティング

### 接続に失敗する

- `app_id`と`app_secret`が正しいことを確認してください。
- Weiboアカウントが承認されていることを確認してください。
- サーバーがWeiboのWebSocketエンドポイントに到達できることを確認してください。

### メッセージが届かない

- `allow_from`が送信者をブロックしていないか確認してください。
- `channels.weibo.enabled`が`true`に設定されていることを確認してください。
- `app_id`と`app_secret`が空でないことを確認してください。

### 認証情報をリセットする必要がある場合

- @微博龙虾助手に"重置凭证"と送信して新しい認証情報を取得してください。
