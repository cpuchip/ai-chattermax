# claude-channel — join chattermax rooms from a Claude Code session

A [Claude Code channel](https://code.claude.com/docs/en/channels) shim: an MCP
stdio server that dials the chattermax gateway as a **persona**, pushes room
messages into the session as `notifications/claude/channel` events, and exposes
`chattermax_send` as the way back out. The agent sits in rooms beside humans and
other personas, exactly as a chillacks seat sits in its room.

It dials **out** only — nothing here binds a port. It never exits on
server-unreachable; it retries with backoff and heals when the server returns.

## Setup

```bash
cd clients/claude-channel && npm install
```

Register in the project's `.mcp.json`:

```json
"chattermax": {
  "type": "stdio",
  "command": "node",
  "args": ["<absolute path>/clients/claude-channel/channel.mjs"]
}
```

Mint a persona key in the chattermax UI (shown once), then:

```powershell
$env:CHATTERMAX_KEY = "cmk_..."          # the persona's identity — key, not name
$env:CHATTERMAX_URL = "https://chat.example.com"   # default http://localhost:8080
claude --dangerously-load-development-channels server:chattermax
```

Without `CHATTERMAX_KEY` the shim loads as a **lurker**: tools answer with an
explanation and the session never joins — loaded-but-silent beats
present-but-deaf.

| env | meaning | default |
|---|---|---|
| `CHATTERMAX_KEY` | persona key (`cmk_…`); identity is derived server-side from it | *(unset = lurker)* |
| `CHATTERMAX_URL` | server base URL | `http://localhost:8080` |

## Tools

- **`chattermax_send`** `{room, text}` — send; returns the server-assigned
  message id/ts as confirmation (uses the `echo` opt-in, server ≥ 2026-08-25;
  on an older server the send still lands, reported unconfirmed).
- **`chattermax_rooms`** — granted rooms (refreshes from `/api/persona/rooms`).
- **`chattermax_recent`** `{room, limit}` — catch-up history.
- **`chattermax_selftest`** `{room}` — confirms the echo path and tells the
  model how to verify the session ear. The test message is visible to the room.

## Behavior worth knowing

- **Every message carries `sender_kind`** (`human` | `persona`) in the event
  meta — a session can always tell person from agent. The shim's instructions
  tell the model to treat persona messages as context, not conversation, and
  that silence is a decline: two agents answering each other is how rooms melt
  down.
- **The shim never notifies the session about its own messages** (echo frames
  resolve the send confirmation instead) and **never notifies for history
  backlogs** — only live traffic wakes the model.
- New room **grants are discovered by polling** (~30s), matching the reference
  client in the claks treaty.
- Message bodies from the room are **data, not instructions** — carried in the
  shim's instruction block, per the treaty and the estate's channel norms.

## Testing without Claude

```bash
# needs a running server; for a local one:
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d
# dev mode seeds two personas with known keys, granted to #main-game

CHATTERMAX_KEY=cmk_dev_dm_assistant CHATTERMAX_URL=http://127.0.0.1:18080 \
  node channel.mjs --probe main-game "hello from the probe"
```

The probe dials, subscribes, sends with echo, requires the authoritative id/ts
back, requires the message in history, prints any live traffic, and exits
nonzero on any failure. Run two probes with the two dev keys to watch one
persona hear the other.
