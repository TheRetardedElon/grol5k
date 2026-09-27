# ADR-0006: GROL operator surface and Grok Bot

Status: accepted
Date: 2026-09-26

## Decision

The GROL web operator surface is the **primary human interface** to GROL5000.
Grok Bot is a **first-class resident component** of that interface, not a chat
box bolted onto Home Assistant.

Rejected for the appliance image:

- XFCE, KDE Plasma, GNOME, or any X11/Wayland session
- nested "web desktop" (noVNC to a full DE)
- Grok / Grok Bot in the kernel, initrd, or with Docker/hostd credentials

Long-term the user only needs:

```
http://grol5000.local
```

They should not have to know about 8123, 8790, Supervisor, Core, hostd,
gateway, or broker.

Until M5 owns `:8123` via `grol-frontend` → `grol-core` → `grol-supervisor`,
the operator surface lives on `:8790`.

```
                  GROL5000
                     │
         ┌───────────┴───────────┐
         │                       │
    GROL Frontend            Grok Bot
         │                       │
         ├─── Devices            ├── Grok (intelligence)
         ├─── Automations        ├── context / sessions
         ├─── System             ├── proposals
         ├─── Build              └── tool requests
         │                       │
         └──────────┬────────────┘
                    ▼
            Capability Broker
              │            │
              ▼            ▼
          GROL Core     grol-hostd
```

M3 split:

- M3A `grol-ai-gateway` — xAI auth, streaming, tool-call normalize, health.
  Grok does not get root, Docker socket, or hostd.
- M3B `grol-bot` — persistent local agent (sessions, context, proposals).
- M3C operator console on `:8790` — Overview / Grok Bot / System / Devices / Activity.
  Read-only except chat. No house mutations.

M4 is the broker. M5 is `:8123` becoming GROL.
`Hey Grok` is another transport into the same bot runtime, not a second assistant.

## Live proof (Build #18 box)

```
Detect Home Assistant Operating System 18.4.dev… / BootSlot A
Fetching update data from https://version.home-assistant.io/dev.json
Updating image …landingpage to …qemux86-64-homeassistant:2026.10.0.dev…
```

Hostname is GROL. Product chrome is still upstream HA until M5.
