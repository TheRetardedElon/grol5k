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

| Verb | Effect | Failure exit |
|---|---|---|
| `status` | process vs live reachability (`house_reachable`, `system_reachable`) | 0 |
| `system status` | healthd snapshot | 4 if `ok:false` |
| `house snapshot` / `devices list` | haobs snapshot | 4 if `ok:false` |
| `device get <id>` | one entity + snapshot envelope | 4 if missing/unhealthy |
| `activity recent` | bot activity | 4 if bot down |
| `propose SERVICE ENTITY` | broker proposal; no HA call | 4 if broker down |
| `raw-call` / `turn_on` / `apply` | forbidden | 3 |

Unknown verbs exit 2.

`status` stays exit 0 even when `status: degraded` so the Bot can read the health fields.
