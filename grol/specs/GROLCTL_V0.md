# GROLCTL v0

Narrow command vocabulary official Grok Bot may invoke on the operator PC
(M3D) and later on the appliance.

Normative with ADR-0007.

## Transport

M3D: HTTP GET to loopback observers.

```
--ha-observer   default http://127.0.0.1:8786
--observer      default http://127.0.0.1:8787
--bot           default http://127.0.0.1:8788
```

Later: same verbs against an authenticated GROL bridge on `grol5000.local`.

## Verbs

| Verb | Reads | Mutates |
|---|---|---|
| `status` | haobs + live house reachability + healthd + live system reachability + bot health | no |
| `system status` | healthd `/v1/snapshot` | no |
| `house snapshot` | haobs `/v1/snapshot` | no |
| `devices list` | haobs devices | no |
| `device get <id>` | one sanitized entity | no |
| `activity recent` | bot `/v1/activity` | no |
| `propose ...` | n/a | refused until M4 |

## Status semantics

`status` distinguishes process reachability from useful live state:

- `haobs`: the grol-haobs process answers.
- `house_reachable`: haobs can currently read a successful HA snapshot.
- `healthd`: the grol-healthd process answers.
- `system_reachable`: healthd can currently return a successful system snapshot.
- `bot`: the local grol-bot process answers.
- `status: degraded`: one or more of those live paths is unavailable.

A responding observer process is not proof that its upstream dependency is healthy.

## Exit codes

- 0: requested read/status command completed with `ok: true`.
- 2: unknown/invalid verb.
- 3: mutation-shaped command refused with `mutation_disabled`.
- 4: supported read command completed with `ok: false` (unavailable, degraded, not found, or other read failure).

Callers must inspect JSON as the source of detail; exit code 4 prevents automation from treating a failed read as success.

## Output

Default: short human text.
`--json`: one JSON object with `ok`, `untrusted`, and `mutation_capable`.
Where applicable it also includes `status` and explicit process/live-state reachability fields.

Official Grok Bot local execution should use `--json`.
