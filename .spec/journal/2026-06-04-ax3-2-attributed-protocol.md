---
date: 2026-06-04
title: AX3-2 — attributed WS protocol (sender envelope, no self-echo, replay)
---

## What happened

Fixed the WS message protocol — the gap the shepherd caught after PR #20 shipped the Vue UI. The room broadcast **raw bytes to everyone including the sender**, so received messages rendered as `sender: 'unknown'` and a sender saw its own message twice (the server echo on top of the client's optimistic push).

Done directly (not via the coder) because this protocol is the **contract #7's persona will speak** — I'm building the persona-host that consumes it, so I own the wire format.

**The fix (server-side only):**
- `room/hub.go`: `BroadcastExcept(roomID, exceptID, msg)`; `Broadcast` delegates with `exceptID=""`.
- `cmd/server`: each message is wrapped as a JSON envelope `{sender, body, ts}` and broadcast to everyone **except** the sender (the client already shows its own optimistically). On join, the room **replays history** to the new client — humans see the backlog, and a persona reads recent turns before its first turn.
- The frontend needed no change: `useChat` already parsed `{sender, body}` and did optimistic push.

**Verification:** a live two-client WebSocket integration test (real dialer) — bob receives `{sender:"alice", body:"hello room"}`, alice gets **no echo**, and a late joiner (carol) receives the **replay**. The assertions are tight to the old bug (raw bytes fail `json.Unmarshal`; an echo to the sender would make alice's read succeed). Plus a `BroadcastExcept` unit test. Full suite green.

**Shipped + live:** committed `83490bd`, pushed to main → Dokploy auto-deploy → `chat.ibeco.me` (deploy status `done`, healthz ok).

## Decisions

- **Wire format = JSON `{sender, body, ts}`**, broadcast **except-sender** (not broadcast-to-all + client de-dup). Simpler, and the existing optimistic-push client already fits it.
- **Replay-on-join** is part of the protocol (not deferred) — #7's persona needs room context on connect, and humans benefit from the backlog.

## Carry-forward

- This is the protocol **#7** builds on: the persona-host connects as a WS client, reads the replayed history + live messages, and posts its turns as `{sender: "DM Assistant", body}`.
- **#7's turn model is richer than reactive** (Michael's call): the persona needs an *aliveness* layer — reactive-with-judgment, @addressing (room/personal/group), delayed/timed/"thinking-out-loud" cadence, pipeline-state hooks, and pileup-avoidance with multiple agents. Michael doesn't have the answers yet → #7 gets a real design pass before building.
- Presence still tags every WS client as `Human`; a persona connection should identify as an agent (a small #7 add — pass a `kind`).
- Token verification on connect is still deferred (the persona-host `/pubkey` is ready whenever we wire it) — a hardening pass, not MVP-blocking.
