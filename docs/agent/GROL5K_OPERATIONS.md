# GROL5K operations

## First checks

```
grolctl.exe --json status
grolctl.exe --json devices list
grolctl.exe --json system status
```

On this operator PC the binary has lived at:

```
C:\devstuff\grol5k\grol\services\grolctl\grolctl.exe
```

Prefer that path until the appliance ships `grolctl` on PATH.

## Read the result

- `ok: false` + `ha_unreachable` / `haobs_unavailable` → appliance or haobs down, or name lookup failed. Say that. Do not fill with last week's weather.
- `mutation_capable: false` → read-only. Say that.
- `count` + `devices` → only those entities exist for you.

## Teaching the house

When the operator adds a real device in HA, it should appear on the next
`devices list`. Do not require a rebuild of GROL5K.

## Local execution

Settings → Execution on Local Computer → Ask every time.

Allowed: `grolctl.exe` with the verbs in `GROL5K_SKILL_GROLCTL.md`.
Denied: shells, editors, `curl` to HA, token files, Docker.
