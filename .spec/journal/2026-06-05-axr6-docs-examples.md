---
date: 2026-06-05
title: AXR6 — connecting-a-persona docs + examples (get a coworker's agent in)
tags: [docs, examples, personas, lm-studio, gemini]
---

## What happened

Closed **AXR6** (docs current + an `examples/` reference persona, so Michael's
coworker can bring their own agent in). Two ways in are now documented:

1. **Use the reference host** — `pg-ai-stewards`'s `cmd/persona-host`: configure
   a minted key, pick a model backend, run.
2. **Build any client** — speak the gateway protocol directly.

Wrote `examples/README.md` (the full guide: mint-a-key-in-the-UI flow, the
gateway protocol contract — `GET /api/persona/rooms`, `wss://…/gateway?key=`,
the frame table, the humans-only turn loop — and the model-backend table) and
`examples/persona-host.example.env` (copy-paste host config). Rewrote the bare
top-level `README.md` to describe the platform + link the guide.

The model-backend examples Michael asked for (AXR5/AXR6 overlap) landed on the
substrate side: **persona-turn-lmstudio** (qwen3.6-27b via local LM Studio) and
**persona-turn-gemini** (gemini-3.5-flash) pipelines, plus a per-persona
`pipeline` column on the host so a persona's model is a *data* choice on its row,
not a code change. LM Studio was **verified end-to-end** (the persona replied
through the local model). Gemini pipeline shipped but not yet smoke-tested.

## Notable

- The `persona` substrate agent was allow-by-default emitting **93 tools**. kimi
  tolerated the malformed function schema; LM Studio rejected it
  (`invalid_type … function.parameters.properties Required`). Fix: `deny *` on the
  persona agent → `compose_tools('persona') = 0` → a clean tools-disabled turn.
  Character personas should always send zero tools.
- `on_one_shot_pipeline_completed` generalized to `LIKE 'persona-%'` so all three
  persona pipelines auto-verify on the same trigger.

## Commits

- ai-chattermax `8662eb7` — docs(AXR6): connecting-a-persona guide + examples/
  (pushed → Dokploy; docs-only, backend binary unchanged).
- workspace-root `dfea527` — R.8(persona): provider pipelines + pipeline selector
  (committed, **not** pushed — root is Michael's to push).

## Carry-forward

- **CT2 (#118)** — BUILD-READY, decisions adopted, 4-phase plan; **awaiting
  Michael's review/ratify** before building (CT2.2 restarts the live substrate).
- **AXR2 (#128)** — Settings room-grant management (list + revoke + `dm_enabled`
  flag). `store.RevokePersonaRoom` already exists.
- **AXR3 (#129)** — DM support (human↔persona).
- **AXR5 (#131)** — wire the Library "Computer" tool-using persona (gospel_search
  + study_search); needs a local `chip-assistant` persona row + a tools-ENABLED
  pipeline (the inverse of the persona `deny *` — a curated allow-list).
- Gemini pipeline (`persona-turn-gemini`) shipped but unverified; smoke it when a
  Gemini-backed persona is next wanted.
