# M3B — grol-bot

Console no longer calls the gateway directly.

```
:8790 grol-console
        →  :8788 grol-bot
                 →  :8789 grol-ai-gateway
                          →  api.x.ai
```

Bot owns:

- session IDs
- conversation context (last 32 turns)
- resident identity system prompt
- activity history
- empty proposal list (M4 fills this)

Bot does not own:

- xAI credentials
- Docker / hostd / HA tokens
- mutation authority
