# Grok Bot product plan

**Status:** accepted planning doc (amended 2026-09-27, ADR-0007).  

## Product sentence

GROL5000 is Home Assistant turned into a home operating system you talk
to. **Official Grok Bot** (Cursor/xAI desktop client) is the human agent.
GROL is the trusted OS and the only policy authority. Grok Build is a
native construction runtime behind a broker. Home Assistant is ancestry
and the integration ecosystem, not the ceiling.

## End-state

```text
                 YOU
                  |
                  v
         Official Grok Bot
         (Cursor Pro / SuperGrok)
                  |
           approved grolctl
                  v
          GROL Capability Layer
                  |
      +-----------+------------+
      v           v            v
 GROL Core   GROL Supervisor   Host OS
                  v
           Physical house
```

## Four different Grok things

Do not collapse these names.

| Name | What it is | Role on GROL5000 |
|---|---|---|
| **Official Grok Bot** | Cursor/xAI desktop+mobile product | Primary human agent; voice/memory/routines |
| **Grok** | xAI models via optional API | OPTIONAL_API_MODE behind `grol-ai-gateway` |
| **grol-bot** | Local GROL resident/bridge | Sessions + observers on the box; not the official product |
| **Grok Build** | Native `grol-buildd` plus coding models | How work is constructed; brokered apply |

Primary operation does not require `XAI_API_KEY`.

## Voice

Official Grok Bot already has dictate and voice chat. GROL does not need
a paid xAI Voice API for the primary path. Appliance wake-word remains later.

## Sequencing

1. M1-B2 / M2 appliance substrate — done.
2. M3A–M3C local observers and console — done as proof.
3. **M3D official Grok Bot + grolctl** — current.
4. M4 broker / `grolctl propose`.
5. M5 GROL Core / Supervisor / frontend ownership.
6. M6 UX / onboarding / CLI face.
7. M7 retire remaining HA product identity.
8. M8 GROL-owned distribution.

Build #19 is packaging/auth/persistence. It is not a Cursor kiosk image.

## Non-goals for the next engineering month

- unsandboxed `grok` CLI on the live appliance rootfs
- official Grok Bot binary inside the OVA
- desktop compositor on the appliance
- `grolctl raw-call` / HA service POST
- spoken "yes" as confirmation
- pretending Core is off-limits forever
