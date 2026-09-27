# M3 — Thin Grok + operator console

Gate: M2 image integration PASS (`59945e0`, OS build #18).
Do not kick another OVA until M3 has a userspace binary that `go test`s green.

## What the live box just proved

`http://grol5000.local` is GROL hostname + upstream Home Assistant UI.
Supervisor still chooses:

```
ghcr.io/home-assistant/qemux86-64-homeassistant:<tag from version.home-assistant.io>
```

Grok Bot cannot appear in that sidebar until we own frontend **or** we serve our own port.
M3 takes the second path first so we do not wait on a Core image cutover.

## M3 deliverables

1. `grol-ai-gateway` (Go, same packaging pattern as `grol-hostd`)
   - xAI credentials from a provisioned file, never from the model
   - streaming chat completions
   - normalized tool-call events
   - provider-hosted tools off
   - no MCP, no web_search, no Build shell
2. `grol-bot` stub
   - conversation state only
   - talks to gateway, not to HA and not to hostd
3. Operator console on `:8790`
   - LAN HTTP page: identity, gateway health, chat box
   - WebSocket later
   - no mutations in M3

## Explicitly out of M3

- XFCE / KDE / GNOME / noVNC desktop
- flipping Supervisor `images.core`
- forking frontend chrome in the live Core container
- `Hey Grok` wake word
- `grol.build.propose`

## How you iterate

Gateway and console are userspace. Test them on a laptop against the live box
without a 90-minute OVA. Bake into the image only after `go test` and a smoke
page load.
