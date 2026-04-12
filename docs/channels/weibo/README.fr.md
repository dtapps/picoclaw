> Retour à [README](../../../README.fr.md)

# Canal Weibo

PicoClaw prend en charge la connexion à Weibo en tant que canal via l'API officielle Weibo sur WebSocket.

## Fonctionnalités supportées

- Réception et envoi de messages directs via Weibo
- Communication temps réel basée sur WebSocket
- Traitement des messages texte
- Filtrage par liste blanche des expéditeurs
- Routage des sorties de raisonnement vers une conversation séparée

> Aucune URL de rappel webhook publique n'est requise. PicoClaw établit une connexion WebSocket sortante vers les serveurs Weibo.

---

## Démarrage rapide

### Obtention des identifiants

1. Ouvrez votre client Weibo (application mobile ou web)
2. Envoyez un message direct à **@微博龙虾助手**
3. Envoyez le message : **连接龙虾**
4. Vous recevrez une réponse avec vos identifiants :

```
您的应用凭证信息如下：

AppId: your-app-id
AppSecret: your-app-secret
```

> Pour réinitialiser les identifiants, envoyez le message "重置凭证".

### Configuration

Ajoutez ce qui suit à votre `config.json` sous `channels` :

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

Puis lancez la passerelle :

```bash
picoclaw gateway
```

---

## Configuration

| Champ | Type | Défaut | Description |
| ----- | ---- | ------ | ----------- |
| `enabled` | bool | `false` | Activer le canal Weibo. |
| `app_id` | string | — | L'App ID de votre application Weibo. Requis si activé. |
| `app_secret` | string | — | L'App Secret de votre application Weibo. Stocké chiffré dans `.security.yml`. Requis si activé. |
| `allow_from` | array | `[]` | Liste blanche des expéditeurs. Vide signifie autoriser tout le monde. |
| `reasoning_channel_id` | string | `""` | ID de conversation optionnel pour router la sortie de raisonnement/pensée. |

### Variables d'environnement

Tous les champs peuvent être écrasés via des variables d'environnement avec le préfixe `PICOCLAW_CHANNELS_WEIBO_` :

| Variable d'environnement | Champ correspondant |
| ----------------------- | -------------------- |
| `PICOCLAW_CHANNELS_WEIBO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_WEIBO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_WEIBO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_WEIBO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_WEIBO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## Comportement runtime

- PicoClaw maintient une connexion WebSocket active avec les serveurs Weibo.
- Les messages texte entrants sont traités par l'agent et les réponses sont envoyées via l'API Weibo.
- Les médias entrants sont téléchargés dans le stockage local avant d'être passés à l'agent.
- Les messages en double sont détectés et supprimés.

---

## Dépannage

### La connexion échoue

- Vérifiez que `app_id` et `app_secret` sont corrects.
- Assurez-vous que votre compte Weibo est autorisé.
- Vérifiez que votre serveur peut atteindre le point de terminaison WebSocket de Weibo.

### Les messages n'arrivent pas

- Vérifiez si `allow_from` bloque l'expéditeur.
- Assurez-vous que `channels.weibo.enabled` est défini à `true`.
- Vérifiez que `app_id` et `app_secret` ne sont pas vides.

### Besoin de réinitialiser les identifiants

- Envoyez le message "重置凭证" à @微博龙虾助手 pour obtenir de nouveaux identifiants.
