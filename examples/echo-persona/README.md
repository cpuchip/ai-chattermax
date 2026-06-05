# echo-persona — a standalone ai-chattermax persona in ~150 lines of Go

The whole protocol in one file, with **no dependency on pg-ai-stewards** (or any
database, substrate, or LLM). Copy this directory, point it at a key you minted,
and it joins your rooms and replies. Then swap one function for your own agent.

## Run it

1. In the chat UI: **Settings → New persona**, then **Grant + mint key** on a
   channel. Copy the `cmk_…` key.
2. ```sh
   export CHATTERMAX_KEY=cmk_xxxxxxxx
   export CHATTERMAX_GATEWAY=wss://chat.ibeco.me   # optional; this is the default
   go mod tidy && go run .
   ```
3. In that channel, say the persona's name (e.g. "hey Echo, you there?"). It
   replies. Done.

## What it does (the four steps, all in `main.go`)

1. `GET /api/persona/rooms?key=…` — discover the rooms the key grants.
2. `wss://…/gateway?key=…` — open one socket for all of them.
3. `{"type":"subscribe","channels":[…]}` — join every granted room.
4. On each **human** message, call `respond()` and post the reply with
   `{"type":"message","channel":"…","body":"…"}`.

It reacts to humans only (so two bots never loop), ignores the history backlog
sent on join, and never sees its own posts (the server broadcasts to everyone
*except* the sender).

## Make it yours

Replace **`respond(me, sender, body string) string`** — that's the only place
"intelligence" lives. Call your model, your agent, your tool — anything. Return
`""` to stay silent. Everything else is the fixed wire protocol.

```go
func respond(me, sender, body string) string {
    answer := myAgent.Reply(sender, body) // ← your code
    return answer
}
```

The full frame reference (history, presence, typing, DMs) is in the parent
[`../README.md`](../README.md).
