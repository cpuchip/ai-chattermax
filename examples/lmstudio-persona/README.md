# lmstudio-persona — echo-persona, but it actually thinks

The same gateway client as [`../echo-persona`](../echo-persona), with one change:
`respond()` now calls a real model over the OpenAI-compatible API
([LM Studio](https://lmstudio.ai/) by default), so replies come from
`qwen3.6-27b` (or any model you've loaded) instead of a canned echo.

Still standalone — no database, no substrate. The only dependency beyond the chat
gateway is a local model server.

## Run it

1. In **LM Studio**: load a model (e.g. `qwen/qwen3.6-27b`) and start the local
   server (port `1234`).
2. In the chat UI: **Settings → New persona → Grant + mint** a key for a channel.
3. ```sh
   export CHATTERMAX_KEY=cmk_xxxxxxxx
   export CHATTERMAX_GATEWAY=wss://chat.ibeco.me     # default
   export LMSTUDIO_MODEL="qwen/qwen3.6-27b"          # default; any loaded model id
   go mod tidy && go run .
   ```
4. In that channel, address the persona by name ("Qwen, are you online?"). It
   replies from the local model.

| env | default | meaning |
|---|---|---|
| `CHATTERMAX_KEY` | — (required) | the persona key you minted |
| `CHATTERMAX_GATEWAY` | `wss://chat.ibeco.me` | the platform gateway |
| `LMSTUDIO_URL` | `http://localhost:1234` | OpenAI-compatible base URL |
| `LMSTUDIO_MODEL` | `qwen/qwen3.6-27b` | model id (see LM Studio's loaded models) |
| `LMSTUDIO_MAX_TOKENS` | `2000` | response budget (see the reasoning-model note) |
| `PERSONA_PROMPT` | a friendly default | the persona's character (system prompt) |

## What it adds over echo-persona

- **`callModel()`** — one POST to `/v1/chat/completions`. Swap this for *any*
  agent/model; the chat protocol around it never changes.
- **Per-room history** — the last few turns are kept and sent as context, so the
  model follows the conversation. Speaker names are prefixed so it can tell who's
  talking in a group chat.
- **Addressed-only** — it answers when its name appears in the message, so a slow
  model isn't invoked on every line. Delete that check in `respond()` to make it
  answer everything.

## Reasoning-model gotcha (learned the hard way)

`qwen3.6` is a **reasoning** model — it spends a few hundred tokens thinking before
it answers, and in this LM Studio build neither `enable_thinking:false` nor
`/no_think` stops it. With a small `max_tokens` it burns the whole budget on
hidden reasoning and returns **empty content** (`finish_reason: "length"`). That's
why `LMSTUDIO_MAX_TOKENS` defaults to 2000 and the code warns when it's still
truncated. A non-reasoning instruct model (a gemma or mistral) is faster, cheaper,
and needs far less headroom.

> Verified live: a `qwen3.6-27b`-backed persona named "Qwen" minted via the API,
> granted to a channel, and replying in ~22s on prod chat.ibeco.me.
