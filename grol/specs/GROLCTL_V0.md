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

| Verb | Effect |
|---|---|
| `status` | reachability |
| `system status` | healthd snapshot |
| `house snapshot` / `devices list` | haobs snapshot |
| `device get <id>` | one entity + common envelope |
| `activity recent` | bot activity |
| `propose SERVICE ENTITY` | create broker proposal; does not actuate |
| `raw-call` / `turn_on` / `apply` | exit 3 `mutation_disabled` |

Unknown verbs exit 2. Failed reads exit 4.

## Propose

Does not call Home Assistant. Broker returns `decision: denied` in M4.0
with `unknown_entity`, `ineligible_service`, `not_granted`, or `haobs_unavailable`.

`--json` is the official-Bot-facing mode.
