---
date: 2026-06-05
title: Chat markdown + scripture panel · gospel-engine web_url (#4) · CT2 folds
tags: [markdown, scripture-panel, gospel-engine, ct2, follow-ups]
---

## What happened (the follow-ups round)

**AXR2b deletes** (earlier): soft-delete persona + delete-DM endpoints/UI; verified
live + used to clean all test residue → only [Computer, Starlet] remain.

**Library Computer wired LIVE**: Michael minted the chip-assistant key; wired into
the in-compose persona-host `.env` + rebuilt/restarted. Confirmed it answered a
King-Benjamin question in #Library (Mosiah 2:17, ~18s).

**Gospel-engine #4 — canonical web links.** Root cause: `gospel_search` returned
only `file_path`, so kimi built `file:///data/gospel-library/...md`. Fix at the tool
boundary (gospel-engine-v2, its own repo): `search.Result` gains a `web_url`
(churchofjesuschrist.org study URL from the file path; `""` for books) behind a
`GOSPEL_LINK_MODE` env (web | fs | both, default both — additive). Unit test caught
my absolute-vs-relative-path bug. Committed `c8f3c79` to the gospel-engine repo —
**NOT pushed** (study.ibeco.me is a shared service outside my push grant; deploy is
Michael's call). The librarian agent prompt (r9, live) now cites `web_url` / plain
refs and never a file path — so the ugly link is gone immediately, pre-deploy.

**Chat markdown + scripture panel.** Borrowed cpuchip.net's `ScripturePanel` +
`useScripturePanel` (animejs trimmed). Message bodies render markdown (markdown-it,
`html:false` → XSS-safe with v-html). churchofjesuschrist.org links get a `.cjc-link`
marker and open the movable, tabbed panel that iframes the live passage (copyright:
text never hosted), instead of navigating away. **Verified on prod via Playwright**:
posted a markdown message with a Mosiah 2:17 link → rendered bold/list/code + the
`✦` scripture-link affordance → clicked it → panel opened and iframed the real
Mosiah 2 from churchofjesuschrist.org. (Starlet ad-libbed a compliment about the
formatting.) ai-chattermax `47a3a2d` pushed → deployed.

**CT2 spec folds (design-only, awaiting ratification):**
- §7.1/§7.2 durable self-notes — model adds AND **removes** its own notes (Michael's
  Hermes point: park to survive compaction, clear once integrated); the system prompt
  splits into an immutable guardrail base + a model-curated self-notes block.
- §7.3 base-prompt edit = the dangerous path, gated propose→ratify (critic + human
  Hinge, versioned/revertible, off by default).
- §7.4 **working tags** — `context_set_tag(tag)` auto-stamps subsequent messages +
  tool calls; `fold_tag/mute_tag/...` sweep a whole finished task out of context in
  one call (one circuit-breaker event). Folded into CT2 core build.

## Commits
- ai-chattermax: `2bbc4fd` (AXR2b deletes), `47a3a2d` (markdown + panel) — pushed/deployed.
- workspace root (unpushed): CT2 spec `e516870` + `14395d4`, persona-host `d16ead4`,
  R.8/R.9.
- gospel-engine repo (unpushed): `c8f3c79`.

## Carry-forward / awaiting Michael
- **Deploy gospel-engine → study.ibeco.me** (his go-ahead) to make web_url live.
- **CT2 ratification** (§§1–6 + §7).
- Test message lingers in 10-forward (no delete-message endpoint) — harmless demo.
