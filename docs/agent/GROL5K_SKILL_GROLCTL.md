# GROL5K skill: grolctl

```
grolctl.exe --json status
grolctl.exe --json devices list
grolctl.exe --json device get <entity_id>
grolctl.exe --json propose light.turn_on light.kitchen
```

Propose creates a broker record. It does not turn anything on.
Expect `decision: denied` and `error: unknown_entity` until a granted
`light.*` / `switch.*` exists and M4 apply is enabled.

Never:

```
grolctl.exe raw-call ...
grolctl.exe apply ...
curl.exe http://.../api/services/...
```
