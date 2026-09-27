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
| `status` | haobs + healthd + bot health | no |
| `system status` | healthd `/v1/snapshot` | no |
| `house snapshot` | haobs `/v1/snapshot` | no |
| `devices list` | haobs devices | no |
| `device get <id>` | one sanitized entity | no |
| `activity recent` | bot `/v1/activity` | no |
| `propose ...` | n/a | refused until M4 |

Unknown verbs exit 2.
Mutation-shaped verbs exit 3 with `mutation_disabled`.

## Output

Default: short human text.
`--json`: one JSON object, `ok`, `untrusted`, `mutation_capable`.

Official Grok Bot local execution should use `--json`.
