> Retour à [README](../../../README.fr.md)

# Canal Tencent Yuanbao

PicoClaw prend en charge la connexion à Tencent Yuanbao comme canal via l'API officielle Yuanbao Bot sur WebSocket.

## Fonctionnalités supportées

- Envoi et réception de messages directs et de groupe
- Communication temps réel basée sur WebSocket avec Yuanbao
- Traitement des messages texte
- Configuration du déclenchement de groupe (mode mention uniquement)
- Filtrage par liste blanche des expéditeurs
- Routage des sorties de raisonnement vers une conversation séparée

> Aucune URL de rappel webhook publique n'est requise. PicoClaw établit une connexion WebSocket sortante vers le serveur Yuanbao.

---

## Démarrage rapide

### Prérequis

Vous devez obtenir les identifiants de votre bot Yuanbao :
- **App ID** (`app_id`)
- **App Secret** (`app_secret`)

### Configuration

Ajoutez ce qui suit à votre `config.json` sous `channel_list` :

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

Puis lancez la passerelle :

```bash
picoclaw gateway
```

---

## Configuration

| Champ | Type | Défaut | Description |
| ----- | ---- | ------ | ----------- |
| `enabled` | bool | `false` | Activer le canal Yuanbao. |
| `app_id` | string | — | L'App ID de votre application Yuanbao. Requis si activé. |
| `app_secret` | string | — | L'App Secret de votre application Yuanbao. Stocké chiffré dans `.security.yml`. Requis si activé. |
| `allow_from` | array | `[]` | Liste blanche des expéditeurs. Vide signifie autoriser tout le monde. |
| `group_trigger` | object | `{}` | Paramètres de déclenchement de groupe. |
| `reasoning_channel_id` | string | `""` | ID de conversation optionnel pour router la sortie de raisonnement/pensée. |

### Configuration du déclenchement de groupe

```json
{
  "group_trigger": {
    "mention_only": true
  }
}
```

| Champ | Type | Défaut | Description |
| ----- | ---- | ------- | ----------- |
| `mention_only` | bool | `true` | Quand true, le bot répond uniquement quand il est mentionné dans les chats de groupe. |

### Variables d'environnement

Tous les champs peuvent être écrasés via des variables d'environnement avec le préfixe `PICOCLAW_CHANNELS_YUANBAO_` :

| Variable d'environnement | Champ correspondant |
| ----------------------- | -------------------- |
| `PICOCLAW_CHANNELS_YUANBAO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_YUANBAO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_YUANBAO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## Comportement runtime

- PicoClaw maintient une connexion WebSocket active avec les serveurs Yuanbao.
- Les messages texte entrants sont traités par l'agent et les réponses sont envoyées via l'API Yuanbao.
- Les messages directs sont envoyés directement à l'utilisateur.
- Les messages de groupe sont envoyés au chat de groupe.
- Les messages en double sont détectés et supprimés.

---

## Dépannage

### La connexion échoue

- Vérifiez que `app_id` et `app_secret` sont corrects.
- Assurez-vous que votre application Yuanbao a les autorisations requises.
- Vérifiez que votre serveur peut atteindre le point de terminaison WebSocket de Yuanbao.

### Les messages n'arrivent pas

- Vérifiez si `allow_from` bloque l'expéditeur.
- Assurez-vous que `channels.yuanbao.enabled` est défini à `true`.
- Vérifiez que `app_id` et `app_secret` ne sont pas vides.
- Pour les chats de groupe, asegurez-vous que `group_trigger.mention_only` est configuré correctement.
