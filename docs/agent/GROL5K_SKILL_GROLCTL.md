# GROL5K skill: grolctl

Approved local commands.

Reads:

```
grolctl.exe --json status
grolctl.exe --json system status
grolctl.exe --json house snapshot
grolctl.exe --json devices list
grolctl.exe --json device get <entity_id>
grolctl.exe --json activity recent
```

Propose (does not actuate):

```
grolctl.exe --json propose light.turn_on light.kitchen
```

Expect `decision: denied` until a granted light/switch exists and apply is enabled.
Typical current house: `error: unknown_entity`.
If haobs is down: `error: haobs_unavailable`, never invent devices.

Forbidden:

```
grolctl.exe raw-call ...
grolctl.exe apply ...
curl.exe http://.../api/services/...
```
