# GROL5K skill: grolctl

Bot-approved:

```
grolctl.exe --json status
grolctl.exe --json system status
grolctl.exe --json house snapshot
grolctl.exe --json devices list
grolctl.exe --json device get <entity_id>
grolctl.exe --json activity recent
grolctl.exe --json propose SERVICE ENTITY
grolctl.exe --json proposal get <id>
grolctl.exe --json grants list
grolctl.exe --json audit recent
```

`ok: true` on propose means received and recorded, not approved.

Forbidden for Bot (enforced by broker, not just this doc):

```
grolctl.exe grant add ...
grolctl.exe grant revoke ...
grolctl.exe proposal confirm ...
grolctl.exe raw-call ...
grolctl.exe apply ...
```

Those require `--operator-token` matching `GROL_OPERATOR_TOKEN` and header `X-GROL-Actor: operator`.
Without that, broker returns `operator_required`.

Grants without proven `registry_id` + `platform` are drafts (`authorizing: false`) and do not authorize `pending_confirmation`.
Confirmation must present the frozen `confirm_digest`. Drift is deny. Apply stays disabled.
