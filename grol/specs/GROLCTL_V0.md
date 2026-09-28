# GROLCTL v0

Narrow command vocabulary official Grok Bot may invoke.
Normative with ADR-0007.

## Transport

```
--ha-observer      default http://127.0.0.1:8786
--observer         default http://127.0.0.1:8787
--bot              default http://127.0.0.1:8788
--broker           default http://127.0.0.1:8785
--operator-token   operator grant/confirm only; never used by Bot
```

## Verbs

| Verb | Reads | Mutates |
|---|---|---|
| `status` | process vs live reachability (`house_reachable`, `system_reachable`, `broker`) | no |
| `system status` | healthd `/v1/snapshot` | no |
| `house snapshot` / `devices list` | haobs snapshot | no |
| `device get <id>` | one entity + snapshot envelope | no |
| `activity recent` | bot activity | no |
| `propose SERVICE ENTITY` | creates broker proposal | no HA call |
| `proposal get ID` | broker proposal record | no |
| `grants list` | grant drafts / records | no |
| `audit recent` | broker audit | no |
| `grant add` / `grant revoke` | operator-only | grant store only |
| `proposal confirm` | operator-only | confirmation record only |
| `raw-call` / `turn_on` / `apply` | forbidden | exit 3 |

## Status semantics (#31)

- `haobs`: grol-haobs process answers
- `house_reachable`: successful HA snapshot
- `healthd`: grol-healthd process answers
- `system_reachable`: successful system snapshot
- `bot`: local grol-bot answers
- `broker`: grol-action-broker answers (informational; does not by itself degrade `status`)
- `status: degraded`: any required live path above except broker is down

`ok: true` on `status` means the command ran. It does **not** mean every subsystem is healthy.

## Exit codes

- 0: `ok: true` (status stays 0 even when `status: degraded`)
- 2: unknown/invalid verb
- 3: `mutation_disabled`
- 4: supported command completed with `ok: false` (including reads, propose, and operator broker commands)

## Propose

Does not call Home Assistant.

`ok: true` on propose means the request was received and recorded, not approved.
Decision/state is the policy result: `denied`, `not_granted`, `pending_confirmation`, `confirmed`, `expired`.

Unhealthy haobs is `haobs_unavailable`, never `unknown_entity`.

## Operator vs Bot

Bot-approved: the read verbs, `propose`, `proposal get`, `grants list`, `audit recent`.

Operator-only (`--operator-token` matching broker `GROL_OPERATOR_TOKEN` plus `X-GROL-Actor: operator`):

- `grant add` / `grant revoke`
- `proposal confirm`

Without that token, those verbs return `operator_required`. Confirm must present the frozen `confirm_digest`. Apply stays disabled.

## Grant identity (M4.3)

`grant add ENTITY SERVICE` does not take caller identity fields.
The broker resolves live haobs identity and stores:

- `registry_id`
- `domain`
- `platform`
- current `entity_id` as an alias only

Canonical grant key is `registry_id + service`.
`identity_proven` is required for `authorizing: true`.
A rename with the same registry_id/domain/platform keeps the grant.
`grant revoke` resolves the current entity through haobs and removes the stable registry grant plus stale entity aliases.
If haobs is down, revoke fails closed (`haobs_unavailable`) and the grant remains.
Unproven live identity stores a non-authorizing draft.
Apply stays disabled. No Home Assistant service call.
