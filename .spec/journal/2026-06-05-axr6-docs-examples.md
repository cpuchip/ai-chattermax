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

**Michael's correction (acted on same session):** the only concrete *host* I'd
documented was `cmd/persona-host`, which is welded to pg-ai-stewards — "the world
doesn't run pg-ai-stewards, just me." So I added **`examples/echo-persona/`**: a
complete persona client in ~150 lines of Go, its own go.mod, zero dependency on
the substrate (no DB, no LLM). The whole protocol in one file; the only
"intelligence" is a `respond()` function you replace with your own agent. Builds +
vets; verified `/api/persona/rooms` against prod returns the exact shapes the
example decodes (persona Starlet, rooms Holodeck-3 + 10-forward), and its WS frames
match what persona-host posts in prod daily. Reworked both READMEs to **lead with
the standalone example** and demote the reference host to "only if you run that
stack." Also untracked two stray `.claude/cache` ground-check artifacts that the
re-grounding hook had committed into the repo.

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

**LM Studio example + live prod test (Michael's follow-up).** He noted the
reference host only helps people who run pg-ai-stewards, and asked me to (a) extend
the example to call LM Studio (qwen3.6-27b) and (b) mint my OWN throwaway key for
Holodeck-3 and test it. Added **`examples/lmstudio-persona/`** — the echo skeleton
with `respond()` calling `/v1/chat/completions` + a per-room history.

Then ran it end-to-end against prod, fully self-service: logged into ibeco.me as
the test account (id 8) → exchanged `becoming_session` for `chattermax_session` →
found Holodeck-3 on Michael's server (`1920b7bc…`, room `fc269f35…`) → created a
test persona "Qwen" → minted its key via the API → granted Holodeck-3 → ran the
example backed by local LM Studio. A human turn drew a real reply in ~22s:
*"I'm online and ready to chat! The best part of a holodeck is definitely being
able to instantly swap a quiet forest for a bustling spaceport without ever leaving
the room."* The whole chain (auth → mint → grant → discover → subscribe → model
call → broadcast) works on prod.

**Reasoning-model finding:** qwen3.6-27b in this LM Studio build ALWAYS thinks —
`enable_thinking:false` and `/no_think` are both ignored. At 80/300/600 max-tokens
it spent 100% on reasoning and returned empty content (`finish_reason=length`); at
4000 it finished (~530 reasoning tokens, then the answer). Example defaults
`LMSTUDIO_MAX_TOKENS=2000`, warns on truncation, strips `<think>`.

**Loose ends from the test (for AXR2):** the throwaway persona "Qwen" + its key +
its Holodeck-3 grant still exist on prod — there's no delete-persona / delete-key /
revoke-grant API yet. That's exactly the **key-management gap Michael flagged** ("we
need to be able to delete a key as well"); AXR2 now covers list+delete keys, not
just room revoke. The minted key lives only in `C:\tmp\qwen.key` (never committed).

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
