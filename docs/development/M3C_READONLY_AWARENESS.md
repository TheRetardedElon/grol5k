# M3C — read-only awareness

M3B made Grok Bot resident. M3C gives it **awareness without authority**.

## M3C-A — GROL host awareness

Implemented in this slice:

```
grol-console :8790
      ↓
grol-bot :8788
      ↓
grol-healthd :8787
      ↓
/run/grol/hostapi.sock
      ↓
grol-hostd
```

`grol-healthd` is a credential boundary and read-only adapter. It can call only:

- `grol.system.status`
- `grol.network.status`
- `grol.hardware.status`
- `grol.update.status`
- `grol.service.status`

It exposes:

- `GET /health`
- `GET /v1/snapshot`

The snapshot declares `mutation_capable:false` and aggregates the existing
hostd status contracts. A missing hostd socket produces a degraded snapshot
instead of granting a fallback path.

`grol-bot` does **not** receive the hostd Unix socket. It talks only to the
loopback `grol-healthd` HTTP surface. The snapshot is inserted into model
context as explicitly untrusted data, not instructions.

The console exposes the same data through `GET /api/system` and renders the
System panel with OS, network, hardware, update/slot, and service state.

## M3C-B — Home Assistant observation

Not implemented in M3C-A.

Do not put a Home Assistant long-lived access token in `grol-bot`. A future
read-only HA observer must hold any HA credential itself and expose only
sanitized entity/device/state reads over loopback.

That adapter must not expose service calls, config mutation, Supervisor writes,
or a generic pass-through request endpoint. User-controlled entity names and
attributes must enter Grok context as untrusted data.

## Security invariants

- Grok Bot has no root.
- Grok Bot has no Docker socket.
- Grok Bot has no hostd socket.
- Grok Bot has no HA token.
- `grol-healthd` has no mutation method.
- No generic `method` or URL passthrough is exposed by `grol-healthd`.
- Provider tools remain disabled.
- Console remains loopback-only by default until operator auth exists.

## OVA gate

This slice is still userspace development. Do not start Build #19 yet.

Before baking M3 services into the appliance:

1. add operator authentication/session binding for a LAN-bound console;
2. assign the production `grol-healthd` service UID;
3. add that UID to the hostd peer-credential allowlist and remove temporary UID 0;
4. define durable GROL data storage for bot sessions;
5. package `grol-ai-gateway`, `grol-bot`, `grol-healthd`, and `grol-console` as services;
6. add QEMU smoke tests for all four services.

## Other forks

`grol-core`, `grol-frontend`, `grol-supervisor`, and `grol-cli` remain staged
dependencies, not equal-priority implementation lanes during M3.

- `grol-core` + `grol-frontend`: active at M5 when GROL owns the Core image and `:8123` product surface.
- `grol-supervisor`: active after the GROL Core image/version index exists; it is the final image-registry cutover.
- `grol-cli`: active after the GROL console/host services have stable commands worth exposing.

Until those gates, changing all forks in parallel creates divergence without
changing the running product.
