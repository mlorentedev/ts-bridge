---
id: "ts-bridge-sequence-diagrams"
type: diagram
status: active
tags: [architecture, diagrams, mermaid]
created: "2026-05-18"
owner: manu
---

# Sequence Diagrams

> Mermaid sequence diagrams for the three runtime flows that matter most when onboarding contributors or debugging behavior: connection proxy, graceful shutdown, and dial-retry. Renders inline in Obsidian and on the Starlight docs site.

---

## 1. Connection proxy (happy path)

End-to-end from a local RDP client to the remote target through the tsnet tunnel.

```mermaid
sequenceDiagram
    autonumber
    participant C as RDP Client<br/>(local app)
    participant L as net.Listener<br/>(127.0.0.1:33389)
    participant A as AcceptLoop
    participant H as handleConn
    participant D as ReconnectDialer
    participant T as tsnet.Server
    participant R as Remote Target<br/>(100.x.x.x:3389)

    C->>L: TCP connect
    L-->>A: net.Conn (client)
    A->>A: check MaxConnections
    A->>H: spawn goroutine
    H->>H: SetKeepAlive(3min)
    H->>D: Dial(ctx, "tcp", target)
    D->>T: Dial via tsnet
    T->>T: awaitRunning (block if needed)
    T-->>D: net.Conn (remote)
    D-->>H: remote conn
    H->>H: wrap both in idleConn<br/>if cfg.IdleTimeout>0
    H->>H: proxyConnections — start tx+rx goroutines
    par tx direction
        C->>H: bytes
        H->>R: bytes (via tsnet)
    and rx direction
        R->>H: bytes (via tsnet)
        H->>C: bytes
    end
    Note over C,R: Bidirectional I/O until either side closes
    C-->>H: EOF / close
    H->>H: closeAll (sync.Once)
    H->>L: release conn slot<br/>AddActiveConnection(-1)
```

**Highlights:**
- `AcceptLoop` is the only persistent goroutine waiting on the listener; `handleConn` spawns one goroutine per accepted conn plus a child for the rx direction in `proxyConnections`.
- `awaitRunning` is the implicit transient-recovery layer (see [adr-006-dialer-interface-extraction](https://github.com/mlorentedev/ts-bridge/blob/master/docs/adr/adr-006-dialer-interface-extraction.md) and the ARCH-004 spec).
- `idleConn` wrapping is invisible when `cfg.IdleTimeout == 0` (it returns the underlying conn unchanged).

---

## 2. Graceful shutdown + drain

What happens when the operator presses Ctrl-C or a SIGTERM arrives.

```mermaid
sequenceDiagram
    autonumber
    participant OS as Operator / OS
    participant M as main.run()
    participant SC as signal context
    participant HS as handleShutdown
    participant L as net.Listener
    participant HSvr as Health HTTP Server
    participant AC as activeConns (sync.WaitGroup)
    participant AL as AcceptLoop
    participant TS as tsnet.Server

    OS->>SC: SIGINT / SIGTERM
    SC->>HS: ctx.Done()
    HS->>HS: ready.Store(false)
    HS->>L: listener.Close()
    HS->>HSvr: Shutdown(ctx, 5s)
    L-->>AL: net.ErrClosed on Accept
    AL-->>M: return nil
    M->>M: drainActiveConnections(cfg, &activeConns)
    Note over M,AC: Wait up to cfg.DrainTimeout for in-flight conns to finish
    alt all conns finish in time
        AC-->>M: done
        M->>M: log "all active connections drained gracefully"
    else timeout
        M->>M: log "drain timeout exceeded, forcing shutdown"
    end
    M->>TS: server.Close()
    M->>M: return errAccept (usually nil)
```

**Highlights:**
- The order matters: stop accepting (`listener.Close`) BEFORE draining, otherwise new conns keep arriving forever.
- `ready.Store(false)` flips the readiness probe immediately so an upstream load balancer (if any) stops sending traffic. Liveness stays true.
- `TS.Close()` is the LAST thing — only after all proxied I/O has drained or been forced shut, so we don't kill in-flight tunnels prematurely.
- In v1.5.1+, this same `TS.Close()` also runs on the error path of `initTailscale` to prevent the Windows file-lock leak (see [error-auth-failure](https://github.com/mlorentedev/ts-bridge/blob/master/docs/troubleshooting/error-auth-failure.md)).

---

## 3. Dial retry with exponential backoff (ARCH-004)

What ReconnectDialer does when the underlying tsnet dial fails transiently.

```mermaid
sequenceDiagram
    autonumber
    participant H as handleConn
    participant RD as ReconnectDialer
    participant T as tsnet.Server
    participant Clk as time.After

    H->>RD: Dial(ctx, network, addr)
    loop attempt = 0..MaxRetries
        RD->>T: Dial(ctx, network, addr)
        alt success
            T-->>RD: net.Conn
            RD-->>H: net.Conn
        else error
            T-->>RD: err
            alt isPermanentDialError(err)
                Note over RD: DNS NXDOMAIN, AddrError,<br/>tsnet terminal state
                RD-->>H: err (no retry)
            else ctx.Err() != nil
                RD-->>H: ctx.Err()
            else last attempt
                RD-->>H: fmt.Errorf("dial failed after N attempts: %w", err)
            else
                RD->>RD: backoff = computeBackoff(attempt, base, max)<br/>= min(base<<attempt, max) + rand[0, half]
                alt attempt == 0
                    RD->>RD: logger.Debug("dial retry")
                else
                    RD->>RD: logger.Warn("dial retry")
                end
                RD->>Clk: time.After(backoff)
                alt ctx cancelled during sleep
                    Clk-->>RD: ctx.Done()
                    RD-->>H: ctx.Err()
                else timer fires
                    Clk-->>RD: tick
                    Note over RD: continue loop, next attempt
                end
            end
        end
    end
```

**Highlights:**
- The retry loop is **inside** `Dial()` — invisible to `handleConn`. From the proxy's perspective, a successful `Dial` either succeeded on the first attempt or after some hidden backoffs; `AddTotalConnection()` increments once per accepted conn, not per attempt.
- `isPermanentDialError` short-circuits to avoid retrying on hopeless cases (typo'd target hostname, terminal tsnet backend state, etc.). Conservative: anything unknown is treated as transient.
- The `select` on `ctx.Done()` vs `time.After(backoff)` is what makes context cancellation respond promptly even mid-sleep — verified by `TestReconnectDialer_ContextCancellationAbortsLoop`.

---

## Generating these diagrams

These are mermaid sequence diagrams (`sequenceDiagram`). They render natively in:
- Obsidian preview (`mermaid` plugin built-in since v0.12)
- GitHub markdown (since 2022)
- Starlight via `astro-mermaid` integration (already in `site/`)

To edit visually, paste a block into <https://mermaid.live/> and iterate.

## Related

- [adr-001-tsnet-userspace](https://github.com/mlorentedev/ts-bridge/blob/master/docs/adr/adr-001-tsnet-userspace.md) — why tsnet is the L3/L4 layer
- [adr-006-dialer-interface-extraction](https://github.com/mlorentedev/ts-bridge/blob/master/docs/adr/adr-006-dialer-interface-extraction.md) — the abstraction that makes ReconnectDialer plug-and-play
- [adr-007-multi-package-split](https://github.com/mlorentedev/ts-bridge/blob/master/docs/adr/adr-007-multi-package-split.md) — why these flows live in `internal/proxy/`
- `specs/archive/REL-003/` — idleConn wrapping in flow 1
- `specs/archive/ARCH-004/` — full design for flow 3
