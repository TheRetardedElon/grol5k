# GROL5K skill: grolctl

Approved local commands (M3D).

```
grolctl.exe --json status
grolctl.exe --json system status
grolctl.exe --json house snapshot
grolctl.exe --json devices list
grolctl.exe --json device get <entity_id>
grolctl.exe --json activity recent
```

Optional flags when loopback observers are not the default:

```
--ha-observer http://127.0.0.1:8786
--observer    http://127.0.0.1:8787
--bot         http://127.0.0.1:8788
```

Refused until M4:

```
grolctl.exe propose ...
grolctl.exe raw-call ...
grolctl.exe turn_on ...
```

Expect `error: mutation_disabled` and exit code 3.

Normative spec: `grol/specs/GROLCTL_V0.md`. Architecture: ADR-0007.
