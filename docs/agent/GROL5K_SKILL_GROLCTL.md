# GROL5K skill: grolctl

Reads stay approved.

```
grolctl.exe --json status
grolctl.exe --json system status
grolctl.exe --json house snapshot
grolctl.exe --json devices list
grolctl.exe --json device get <entity_id>
grolctl.exe --json activity recent
```

Propose / inspect (Bot-approved):

```
grolctl.exe --json propose light.turn_on light.kitchen
grolctl.exe --json proposal get <id>
grolctl.exe --json grants list
grolctl.exe --json audit recent
```

States: `denied` `not_granted` `pending_confirmation` `confirmed` `expired`.
`ok: true` on propose means the request was *received and recorded*, not approved.
Decision/state are the policy result. Never say a proposal was "accepted" unless `decision` is `confirmed`.

Operator-only (do not invent these as Bot self-service):

```
grolctl.exe --json grant add light.kitchen light.turn_on
grolctl.exe --json proposal confirm <id>
```

Confirm does not actuate. `apply_enabled` stays false. No HA_TOKEN.

Forbidden: raw-call, apply, curl to HA.
