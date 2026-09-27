# ADR-0007: Official Grok Bot is the human agent; GROL is the authority

Status: accepted
Date: 2026-09-27
Amends: ADR-0003, ADR-0006

## Decision

**Official Grok Bot** (Cursor/xAI desktop + mobile client) is the primary
human-facing agent of GROL5000.

**GROL5000** remains the trusted OS and the only policy authority.

```
Official Grok Bot     = intelligence / human agent (Cursor Pro / SuperGrok)
GROL5000              = trusted OS + policy authority
grolctl               = approved local command bridge
grol-bot / grol-bridge = local resident runtime (not the official product)
grol-ai-gateway       = OPTIONAL_API_MODE
broker                = sole mutation authority (M4)
```

Primary mode does **not** require `XAI_API_KEY` or `api.x.ai` credits.
Cursor Pro already includes official Grok Bot usage. An empty xAI developer
wallet must not block operating the house OS.

## What official Grok Bot is

A cloud-hosted teammate with a desktop/mobile remote. Its computer runs in
Cursor's cloud. It can run a command on the operator's local machine only when
**Execution on Local Computer** is enabled (default: ask every time).

It is not:

- a Linux kernel component
- PID 1
- the HA admin token holder
- a thing we bake into Build #19
- a full desktop session inside the HAOS appliance

## Topology

```
OFFICIAL GROK BOT          (Windows / macOS / Linux PC)
Cursor account, voice, memory, routines, cloud computer
        |
        | approved local execution
        v
   grolctl(.exe)
        |
        | HTTP to local observers (M3D now)
        | later: authenticated bridge on grol5000.local
        v
   GROL5000 appliance
        |
   +----+-----+----------+
   v          v          v
 haobs     healthd     broker (M4)
   |          |          |
   v          v          v
 HA Core    hostd     controlled action
```

Today the M3 daemons still run on the operator PC against the live VM.
`grolctl` talks to those loopback ports. Same verbs later target the appliance.

## Forbidden now

- Weston/Cage/KDE/GNOME/XFCE on the appliance image
- shipping the official Grok Bot desktop binary in the OVA
- `grolctl raw-call` / `POST /api/services/...`
- giving official Grok Bot `HA_TOKEN`, Docker, or hostd
- treating `grol-bot.exe` as if it *is* Cursor's product

## Allowed now (M3D)

Read-only `grolctl` verbs:

- `status`
- `system status`
- `house snapshot`
- `devices list`
- `device get <entity_id>`
- `activity recent`

`--json` is the official-Bot-facing mode. Human text is secondary.

`grolctl propose ...` is reserved. M3D must refuse it.

## Optional API mode

`grol-ai-gateway` remains for headless / server-to-server inference when a
user explicitly provisions `XAI_API_KEY`. It is not the primary brain.

## Consequences

- Grok Bot (the product the user already pays for) is the mouth.
- GROL never surrenders mutation authority.
- Build #19 stays packaging/auth/persistence, not a Cursor kiosk.
- A later appliance kiosk seat would be a separate ADR after Windows
  local-execution + `grolctl` is proven.
