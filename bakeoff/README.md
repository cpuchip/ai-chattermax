# chatcore — multi-room chat core

Zero-dependency stdlib packages extracted from the ai-chattermax design proposal
(build-plan items 1, 3, and 10).

- **room** — concurrency-safe multi-room broadcast Hub with a transport-agnostic
  `Client` interface.
- **presence** — room-scoped participant tracking with typed `Kind` (human vs
  persona) and a `Thinking` flag for AI personas.
- **ratelimit** — per-participant hard rate ceiling with a rolling window and
  an injectable clock for deterministic tests.

## Build / test

```bash
cd bakeoff
go build ./...
go test -race -count=1 ./...
```
