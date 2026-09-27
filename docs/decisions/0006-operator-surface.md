# ADR-0006: Grok Bot operator surface

Status: accepted
Date: 2026-09-26

## Decision

Grok Bot is commanded from a **web operator surface**, not from a Linux desktop environment on the appliance.

Rejected for the appliance image:

- XFCE, KDE Plasma, GNOME, or any X11/Wayland session
- nested "web desktop" (noVNC to a full DE)
- putting Grok Bot inside the Linux kernel or initrd

Accepted:

```
Browser
  │
  ├─ http://<host>:8123     inherited HA / later GROL frontend
  └─ http://<host>:8790     grol-bot console (M3/M4)
         │
         ▼
      grol-bot
         │
         ▼
   grol-ai-gateway  →  xAI / Grok
         │
         ▼
  grol-action-broker → HA services / grol-hostd / later grol-buildd
```

`:8790` is a GROL-owned HTTP/WebSocket console on the appliance LAN.
It is not the Supervisor API and not the Home Assistant frontend.
Later `grol-frontend` embeds this console as a first-class sidebar, then replaces HA chrome.

## Why not a desktop

GROL5000 is a headless appliance (UEFI → Linux → Docker → Supervisor → Core).
A DE would add hundreds of megabytes, a local attack surface, and a second product
that is not how users already reach this box (`grol5000.local`).

The operator already has a browser. That is the desktop.

## Live proof (Build #18 box)

```
Detect Home Assistant Operating System 18.4.dev… / BootSlot A
Fetching update data from https://version.home-assistant.io/dev.json
Updating image …landingpage to …qemux86-64-homeassistant:2026.10.0.dev…
```

Hostname and console are GROL. Core, Supervisor, and onboarding are still upstream HA.
That is expected until M5 registry/index cutover.

## Consequences

- M3 ships `grol-ai-gateway` + a stub `grol-bot` HTTP console. No house mutations.
- M4 wires the broker so the console can request `light.*` / `switch.*` through policy.
- M5/M6 replace HA onboarding chrome via `grol-frontend` built into `grol-core`.
- Voice (`Hey Grok`) stays after M5. Push-to-talk can land in the console earlier.
