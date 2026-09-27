# GROLCTL v0

Narrow command vocabulary official Grok Bot may invoke.
Normative with ADR-0007.

## Transport

```
--ha-observer   default http://127.0.0.1:8786
--observer      default http://127.0.0.1:8787
--bot           default http://127.0.0.1:8788
--broker        default http://127.0.0.1:8785
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
| `raw-call` / `turn_on` / `apply` | forbidden | exit 3 |

## Status semantics (#31)

- `haobs`: grol-haobs process answers
- `house_reachable`: successful HA snapshot
- `healthd`: grol-healthd process answers
- `system_reachable`: successful system snapshot
- `bot`: local grol-bot answers
- `broker`: grol-action-broker answers (informational; does not by itself degrade `status`)
- `status: degraded`: any required live path above except broker is down

## Exit codes

- 0: `ok: true` (status stays 0 even when `status: degraded`)
- 2: unknown/invalid verb
- 3: `mutation_disabled`
- 4: supported read/propose completed with `ok: false`

## Propose

Does not call Home Assistant. Broker returns `decision: denied` in M4.0
with `unknown_entity`, `ineligible_service`, `not_granted`, or `haobs_unavailable`.
Unhealthy haobs is never classified as `unknown_entity`.
