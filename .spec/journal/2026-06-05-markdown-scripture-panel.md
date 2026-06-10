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
my absolute-vs-relative-path bug. Committed `c8f3c79` to the gospel-engine repo,
**pushed + DEPLOYED** (Michael authorized: "push gospel-engine-v2 now it'll build
and publish automatically"). The gospel-engine lives at **engine.ibeco.me** — NOT
study.ibeco.me; my working note had the wrong domain, and the authoritative URL is
`.mcp.json` → `GOSPEL_ENGINE_URL=https://engine.ibeco.me`. Verified live:
`engine.ibeco.me/api/version` → 200 (new build), and `gospel_search` results now
carry a canonical `web_url` (e.g. `tg/faith` → `…/study/scriptures/tg/faith?lang=eng`).
The librarian agent prompt (r9, live) already cited `web_url` / plain refs, never a
file path — so the ugly link was gone even before the engine picked up the new field.

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
- gospel-engine repo: `c8f3c79` — **pushed + deployed to engine.ibeco.me** (done 15:17).

## Carry-forward / awaiting Michael
- ✅ **gospel-engine deployed** (engine.ibeco.me); web_url live + verified. DONE.
- **CT2 ratification** (§§1–6 + §7).
- ✅ **ibeco.me `web` build break — diagnosed + fixed + deployed green.** NOT transient:
  the `web` app auto-deploys off the workspace-ROOT monorepo, and root commit `e516870`'s
  subject contained an apostrophe ("Michael's") that closed the `-X 'main.ReleaseNotes=$MSG'`
  single-quote grouping in `scripts/becoming/Dockerfile` → linker usage dump, exit 1.
  Fixed by sanitizing NOTES (`tr -d '\047\042'`), commit `2b98b4c`; reproduced + verified
  via a minimal repro AND the live redeploy (HEAD `3cde2ee`, `/version` shows the commit
  subject baked in correctly). becoming was the only Dockerfile injecting the commit msg
  into ldflags. Dokploy skill + a memory ([[reference_ibeco_deploy_topology]]) updated.
  The ibeco apps are on the NOCIX Dokploy (`server.ibeco.me`, `DOKPLOY_NOCIX_API_KEY`) —
  NOT hmslogs; gospel-engine = engine.ibeco.me.
- Test message lingers in 10-forward (no delete-message endpoint) — harmless demo.
