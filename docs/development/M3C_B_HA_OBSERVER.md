# M3C-B — read-only Home Assistant observer

```
Home Assistant Core :8123
        │  long-lived token held here only
        ▼
grol-haobs :8786
        │  sanitized GET snapshot
        ▼
grol-bot
        │
        ▼
Grok
```

Not in this process: POST /api/services, Supervisor, config writes,
admin token sharing with the model.
