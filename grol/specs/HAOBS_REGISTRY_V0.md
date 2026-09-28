# haobs registry identity v0 (M4.2)

Join live Home Assistant state with entity-registry identity.
Does **not** enable mutation.

```
GET /api/states
websocket config/entity_registry/list
        ↓
join by entity_id
        ↓
device projection:
  entity_id
  registry_id     (registry entry `id`)
  domain
  platform
  state
  identity_proven
```

`identity_proven` is true only when registry_id, domain, and platform are all present.
If the websocket list fails, the states snapshot still succeeds with `registry_status: unavailable` and `identity_proven: false`.

Broker grants remain non-authorizing until those three fields are proven and match.
HA service execution stays disabled.
