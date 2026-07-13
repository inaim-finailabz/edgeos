# Getting started (v0, pre-release)

The full v0 pipeline works today: an agent on your device supervises a real
`llama-server`, announces itself via mDNS, and reports live measured
capabilities; a router discovers agents, scores them, and proxies
OpenAI-compatible chat completions (streaming and non-streaming) straight
to the winning node's engine, with a typed-error failover path to a
configured cloud endpoint when nothing local qualifies.

## What's not in v0

Per [CLAUDE.md](../CLAUDE.md), intentionally out of scope: registry sync
(model catalog / fleet-wide sync — `edgeos pull` is a plain file download,
not a registry), vector store, GPU scheduling, phone-as-node, auth. Also:
no crash-auto-restart of a dead `llama-server` process (the agent reports
`error` state and stops), and `queue_depth` is always `0` — llama-server has
no native queue-depth metric to report.

## Prerequisites

- Go 1.25+
- [llama.cpp](https://github.com/ggml-org/llama.cpp)'s `llama-server` on
  your `$PATH` (or pass `-llama-server-bin /path/to/it` — see agent flags
  below)
- a `.gguf` model file (`edgeos pull` can fetch one — see below)
- `jq`, only for `benchmarks/run.sh`

## Build

```sh
make build    # agent, router, cli for your local OS/arch -> dist/
make cross    # all three for linux/arm64, linux/amd64, darwin/arm64
```

Cross-compiled binaries land in `dist/<os>-<arch>/<component>` — copy the
set matching your device (Pi 5 / Jetson Orin Nano -> `linux-arm64`, Mac ->
`darwin-arm64`, 4070 box -> `linux-amd64`). Keep `agent`, `router`, and
`cli` together in the same directory — `edgeos up` looks for `agent`
alongside itself.

## Pull a model

```sh
./dist/darwin-arm64/cli pull Qwen/Qwen2.5-3B-Instruct-GGUF/qwen2.5-3b-instruct-q4_k_m.gguf
# or a direct URL:
./dist/darwin-arm64/cli pull -out ./models/mymodel.gguf https://example.com/model.gguf
```

Saves to `~/.edgeos/models/<filename>` by default.

## Bring up a node

```sh
./dist/darwin-arm64/cli up -model ~/.edgeos/models/qwen2.5-3b-instruct-q4_k_m.gguf
```

This execs the sibling `agent` binary, which spawns `llama-server`, waits
for it to become healthy, runs a real 50-token benchmark for `tok_per_sec`,
and starts announcing via mDNS and serving `GET /v0/capabilities`
(default `:8090`). Check it directly:

```sh
curl http://localhost:8090/v0/capabilities
```

## Bring up the router

```sh
./dist/darwin-arm64/router -addr :8081 -cloud-endpoint https://api.openai.com -cloud-api-key sk-...
```

`-cloud-endpoint`/`-cloud-api-key` are optional — omit them and requests
with no qualifying local node get a typed `503` instead of a cloud
fallback. The router rediscovers agents and polls their capabilities every
`-poll-interval` (default 2s), evicting after `-miss-threshold` (default 3)
consecutive misses.

```sh
./dist/darwin-arm64/cli nodes -router http://localhost:8081
```

## Send a chat completion

```sh
curl http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2.5-3b-instruct-q4_k_m.gguf","messages":[{"role":"user","content":"hi"}],"stream":true}'
```

The router scores every node reporting that model as `loaded` with enough
`ctx_max` for the request (a v0 heuristic token estimate — no tokenizer
dependency), picks the highest `tok_per_sec` adjusted for
`active_requests`, and proxies the response straight through.

## Benchmark a llama-server directly

```sh
./benchmarks/run.sh -u http://localhost:8080 -n 50
```

Same mechanism the agent's own load-time benchmark uses internally.

## Running the test suite

```sh
make check    # gofmt check, go vet, go test
```

## Questions / contributing

v0 scope is intentionally narrow — see [CLAUDE.md](../CLAUDE.md) for what's
explicitly out of scope. Open an issue before building against anything not
covered here or in [docs/CAPABILITY_SCHEMA.md](CAPABILITY_SCHEMA.md).
