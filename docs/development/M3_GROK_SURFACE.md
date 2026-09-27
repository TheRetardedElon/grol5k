# M3 — Thin Grok + operator surface

Gate: M2 image PASS (`59945e0`, OS build #18).
Do not bake into an OVA until `go test` is green on the userspace trees.

## Sequence

| Slice | Ships |
| --- | --- |
| M3A | `grol-ai-gateway` + console skeleton (this branch) |
| M3B | `grol-bot` persistent sessions / proposals |
| M3C | console panels filled with live hostd + HA read-only status |
| M4 | action broker, `light.*` / `switch.*` |
| M5 | grol-frontend in grol-core, `:8123` is GROL |

## M3A rules

- stdlib only
- listen on loopback / explicit bind, default `127.0.0.1:8789` (gateway) and `0.0.0.0:8790` (console)
- credentials from file/env, never from the model
- provider-hosted tools off
- no Docker socket, no hostd, no broker mutations
- degraded/offline response when no xAI key is provisioned
