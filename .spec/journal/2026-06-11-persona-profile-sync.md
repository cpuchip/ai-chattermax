# 2026-06-11 — Persona profile sync (PATCH /api/persona/profile)

**Why:** Michael renamed the Party persona to **Callie** (she/her, the table's caller). Host↔platform display-name drift is the Codewright/Chattercode silence class — the persona stops recognizing its own name in addressing. Instead of a one-off DB poke, the platform now accepts identity from the persona's host.

**Shipped (`0be51ec` + marker bump `02a66cc`, deployed, verified `persona-profile-sync-2026-06-11`):**
- `SetPersonaDisplayName` (store) — display name only; slug, grants, policy stay admin-owned.
- `PATCH /api/persona/profile` — persona-key auth (mirrors PersonaRoomsHandler), 1–64 chars, no-ops when names match.
- persona-host calls it in `Run()` **before dialing**, so the ready frame and all attribution already carry the registry name.

**Proven live:** persona-host recreate → first connect synced the platform row; `GET /api/persona/rooms` returned `displayName: "Callie"` (platform slug stays `party` — internal only, not an addressing surface; mentions resolve on display names).

**Context (persona-host side, root repo `18b31f7`):** rename was triggered by Party answering a sheet request explicitly @-addressed to the DM — judgment-policy chime-in with no deference rule, amplified by a duplicate persona-host (a parallel session's native exe). Deference gate + nudge live in the host now.

**Carry-forward:** none platform-side. If hosts ever need avatar/slug sync, extend the same endpoint deliberately.
