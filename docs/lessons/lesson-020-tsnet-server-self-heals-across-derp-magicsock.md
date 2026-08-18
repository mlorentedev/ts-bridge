---
id: lesson-020-tsnet-server-self-heals-across-derp-magicsock
type: lesson
status: active
created: "2026-05-01"
owner: manu
tags: [ts-bridge, lesson, tsnet, tailscale, self-healing, design-investigation]
---

# tsnet.Server self-heals across DERP/magicsock transients via awaitRunning

**Context:** ARCH-004 spec required answering "does tsnet self-heal the control-plane tunnel after a drop?" before designing the reconnect layer. The vault spec doc allotted ≤30 min for this investigation.

**Finding:** Reading `tailscale.com/tsnet@v1.80.0/tsnet.go:168-206`, every `Dial()` call invokes `awaitRunning(ctx)` which blocks on `ipn.Notify` state transitions until the backend returns to `ipn.Running`. DERP relay drops and magicsock reconnects are handled at lower layers transparently — by the time `Dial()` returns, the tunnel is healthy. Only terminal states (`Stopped`, `NeedsMachineAuth`) surface as the error `"tsnet: backend in state %v"` from `awaitRunning`. Net effect: a retry layer at the `Dialer` interface covers virtually all transient failures, and a dedicated control-plane supervisor (Tier 2 in the ARCH-004 design) is unnecessary.

**Rule:** Before designing a supervisor for any tsnet-based component, verify what `awaitRunning` already gives you. Tsnet's lifecycle reads "blocks until Running or terminal" — that's already a supervisor in disguise. Saved ~50% of the ARCH-004 implementation surface.

**Tags:** `#tsnet` `#tailscale` `#self-healing` `#design-investigation`
