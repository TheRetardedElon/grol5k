# GROL Services

| Service | Purpose | Earliest milestone |
|---|---|---|
| `grol-hostd` | Host API on `/run/grol/hostapi.sock` | M2 |
| `grol-healthd` | health aggregation | M2 / M3C-A |
| `grol-identity` | version/build/identity | M2 |
| `grol-provision` | first-run + credentials | M2 |
| `grol-haobs` | read-only HA observer | M3C-B |
| `grolctl` | official Grok Bot local command bridge | M3D |
| `grol-ai-gateway` | OPTIONAL_API_MODE provider/session | M3 optional |
| `grol-bot` | local resident bridge/runtime (not Cursor's product) | M3B |
| `grol-console` | local operator surface | M3A |
| `grol-action-broker` | policy/execution | M4 |

`grolctl` is read-only in M3D. Official Grok Bot on the operator PC may invoke it after Execution on Local Computer is approved. See ADR-0007.

`grol-hostd` is Go, packaged like OS Agent (`golang-package`, local tree).
No CPython on the appliance for this daemon.
