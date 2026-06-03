# Coder proof — WebSocket chat-room core

**Substrate-generated. NOT the ratified ai-chattermax implementation.** This is a
learning artifact: the pg-ai-stewards **coder** (the `code-write` pipeline)
built this autonomously from a one-line task, to prove the coding capability
handles app-shaped code before we tackle the real chat build.

## How it was made

- **Date:** 2026-06-03
- **Built by:** pg-ai-stewards `code-write` pipeline (plan → implement → verify), agent `dev`, model `kimi-k2.6` (opencode_go).
- **Task given (the entire input):** *"Write a self-contained Go WebSocket chat-room server in package main — the core 'the room is the message bus' piece of a multi-party chat. … an HTTP server exposing a /ws endpoint that upgrades to WebSocket using github.com/gorilla/websocket; a Hub that registers/unregisters clients and broadcasts every received message to all connected clients via goroutine + channels; a main() listening on :8080; a unit test for the Hub broadcast logic. Build+test: go mod init chatroom && go mod tidy && go build ./... && go test ./..."*
- **What the agent did, on its own:** provisioned an isolated sandbox, wrote `main.go` + `hub_test.go`, fetched the external `gorilla/websocket` dependency over the network (`go.mod`/`go.sum`), built it, and iterated the build/test loop to green.

## Verified (by hand, not trusting the "verified" flag)

- `go build ./...` → OK; `go test ./...` → `ok chatroom`.
- **Live integration:** ran the server, connected two real WebSocket clients; client 1 sent a message and **client 2 received the broadcast** ("hello room, from client 1"). It works as a multi-party room, not just compiles.

## What this proves

The coder handles real app-shaped code — a third-party dependency, a concurrent
hub (register/unregister/broadcast over channels with a `select` loop), slow-client
backpressure (`default: close(send); delete`), readPump/writePump with clean
teardown, and a concurrency test. This is the canonical gorilla/websocket hub
pattern, correctly implemented. Far beyond the FizzBuzz/calc proofs.

## What to learn from it (toward the real build)

- **Known patterns are easy; the real test is novel code.** The gorilla hub is
  well-represented in training data — kimi-k2.6 nailed it. The harder validation
  is code with no canonical template (the classifier gate, the persona handshake).
  Watch model quality there; a stronger opencode_go model may be warranted.
- **Fresh-sandbox + from-scratch worked.** But the real chat build needs the coder
  to **work in the existing `ai-chattermax` repo and land output as reviewable PRs**
  (the "code-write v2" upgrade) — this proof was built in an empty `/work`, not into
  the project.
- **It validates design Q1's working bet** (WebSocket + turn-taking; the room IS the
  message bus). Still unbuilt + unratified: the classifier prompt-gate (Q3), the
  ibeco.me cookie auth, the persona registration handshake + sub-tokens (Q2/Q5),
  rate-cap/quiet-periods (Q4).

## Status

Design-only project; this experiment does **not** change that. The real
ai-chattermax build waits on (1) ratifying the five design questions and (2) the
coder v2 (work-in-repo + PR). See `../../.spec/proposals/chat-server-design.md`.
