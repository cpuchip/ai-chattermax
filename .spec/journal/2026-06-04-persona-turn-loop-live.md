---
date: 2026-06-04
title: "#7 v1 — a persona talks in the room (turn loop live)"
---

## What happened

The persona turn loop went live. A human posts in a room and an AI persona — `DM Assistant` — reads it and replies in character, as a real participant in the roster (`Kind=persona`, not Human). This is the thing the whole project has been building toward: AI personas conversing alongside humans.

The cognition lives in pg-ai-stewards (the persona-host sidecar drives a real substrate `persona-turn` dispatch per turn — see that repo's journal `2026-06-04-persona-turn-loop-r7.md`). ai-chattermax's part was small and clean: the room now honors `?kind=` on WS connect so a persona registers as an agent rather than Human (`5601164`, pushed → auto-deploy → chat.ibeco.me). Backward-compatible: absent/unknown kind defaults to Human.

## The thing that proved the protocol work paid off

In the live e2e the human's message was posted *before* the persona finished connecting (a startup race the persona-host's reconnect supervisor recovered from). The persona still answered — because **AX3-2 replay-on-join** (the protocol fix from earlier the same day) delivered the backlog to it on connect. The "replay history on join" decision, made for "the persona needs room context," turned out to also be what made the persona robust to connection timing. The contract and its consumer landed in the same day and fit.

## Decisions

- **`?kind=` is the room's whole role in #7 v1.** Everything else (cognition, the turn gate, humans-only, @mentions, SILENCE) lives in persona-host. The room stays a dumb, fast transport — which is the right boundary.
- **v1 is humans-only** (ratified): a persona ignores other personas entirely. Zero ping-pong risk. The D&D magic of personas riffing off each other is a clean v2 add with real arbitration.

## Carry-forward

- **Persona JWT verify on WS connect** is still deferred (the persona-host `/pubkey` is ready). A persona currently joins with just `?id=&kind=`; binding that to a verified token is the hardening pass before any untrusted persona-host connects.
- **Presence `kind` is server-tracked but not yet surfaced in the UI** — the frontend could badge agents vs humans now that the roster carries `Kind`.
- **#11 moderation + #12 D&D MVP** are the next product layers; the turn loop is the substrate they sit on.
