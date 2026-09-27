# M3B — grol-bot

M3A proved the provider boundary and live Grok stream. M3B inserts the resident
local agent between the operator surface and the credential-holding gateway.

```
:8790 grol-console
        →  :8788 grol-bot
                 →  :8789 grol-ai-gateway
                          →  api.x.ai
```

## Bot owns

- opaque session IDs issued by the bot
- multi-turn conversation context (last 32 stored turns)
- resident GROL5000 identity system prompt
- activity history (last 200 local events)
- empty proposal list (`M4` fills this)

The browser stores the bot-issued session ID and sends it on the next turn.
Unknown client-provided session IDs are not authoritative; the bot reissues one.

For streamed chat the bot emits a GROL-owned `session` SSE event before
forwarding normalized gateway `delta` / `done` / `error` events.

Read-only bot APIs:

- `GET /health`
- `GET /v1/sessions`
- `GET /v1/activity`
- `POST /v1/chat`

The console proxies sessions/activity on `/api/sessions` and `/api/activity`.

## Bot does not own

- xAI credentials
- Docker socket
- grol-hostd credentials or calls
- Home Assistant tokens/services
- mutation authority
- Build shell
- MCP/provider-hosted tools

## Persistence boundary

M3B v0 sessions are resident **in the grol-bot process** and survive browser
turns/reloads when the same bot process remains running. They are not yet
durable across a bot/OS restart.

Do not choose a random host-root path for durable chat history on the immutable
appliance. The OVA integration slice will define the GROL writable data location
(e.g. under the persistent data partition) and add atomic on-disk state there.

## Laptop validation

Run all three userspace services; do not build another OVA for this slice.

```bash
# terminal 1
cd grol/services/grol-ai-gateway
XAI_API_KEY="..." go run .

# terminal 2
cd grol/services/grol-bot
go run .

# terminal 3
cd grol/services/grol-console
go run .
```

Open `http://<development-machine>:8790`.

Expected request path:

```
browser session id
      ↓
grol-console
      ↓
grol-bot context + identity
      ↓
grol-ai-gateway credentials/provider boundary
      ↓
Grok
```

No M3B code may mutate the house or host.
