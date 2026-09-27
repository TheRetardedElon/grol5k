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


## M3A live development

The gateway uses xAI's Responses API and normalizes provider SSE into a small
GROL-owned stream protocol. The console never parses xAI-specific events.

Environment:

```bash
export XAI_API_KEY="..."
export XAI_MODEL="grok-4.7"       # optional; current M3A default
export XAI_BASE_URL="https://api.x.ai"  # optional
```

Run the services on a development machine:

```bash
cd grol/services/grol-ai-gateway
go test ./...
go run .

# second terminal
cd grol/services/grol-console
go test ./...
go run .
```

Then open `http://<development-machine>:8790`.

Gateway contract:

- `GET 127.0.0.1:8789/health`
- `POST 127.0.0.1:8789/v1/chat`
- request `{"messages":[...],"stream":true}`
- normalized SSE events: `delta`, `done`, `error`
- xAI request uses `store:false`
- no `tools` are sent to xAI; inbound provider tool requests are rejected
- no hostd, Docker, HA service, broker, MCP, web search, X search, or Build path

The console proxies that normalized stream on `POST /api/chat`. Conversation
persistence/session context remains M3B work; M3A intentionally sends only the
current browser turn.
