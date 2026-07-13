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
not a registry), vector store, GPU scheduling, phone-as-node, and any
multi-user auth (the dashboard's management token is a single shared
secret, not accounts/roles). Also: no crash-auto-restart of a dead
`llama-server` process (the agent reports `error` state and stops), and
`queue_depth` is always `0` — llama-server has no native queue-depth metric
to report.

## Prerequisites

- Go 1.25+
- [llama.cpp](https://github.com/ggml-org/llama.cpp)'s `llama-server` on
  your `$PATH` (or pass `-llama-server-bin /path/to/it` — see agent flags
  below)
- a `.gguf` model file (`edgeos pull` can fetch one — see below)
- `jq`, only for `benchmarks/run.sh`

## Build

```sh
make build    # agent, router, cli, dashboard for your local OS/arch -> dist/
make cross    # all four for linux/arm64, linux/amd64, darwin/arm64
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

## Manage the fleet with the dashboard

Stop/reload/evict are authenticated — set the same `-management-token` (or
`$EDGEOS_MANAGEMENT_TOKEN`) on every agent and the router, or those routes
stay disabled (404):

```sh
export EDGEOS_MANAGEMENT_TOKEN=change-me
./dist/darwin-arm64/cli up -model ~/.edgeos/models/qwen2.5-3b-instruct-q4_k_m.gguf   # picks up the env var
./dist/darwin-arm64/router -addr :8081                                              # same
./dist/darwin-arm64/dashboard -addr :8092 -router http://localhost:8081
```

Open `http://localhost:8092`, paste `change-me` into the token field (kept
in the browser's `localStorage`), and you get:

- a **KPI strip** — total/healthy nodes, active requests, fleet-wide tok/s
  capacity, distinct models loaded, requests served (local/cloud) and
  failed — all real numbers computed from the live node list plus the
  router's own request counters, not estimates
- an **Add node by IP/DNS** form, for a node mDNS can't reach (different
  subnet, a cloud box) — `POST /v0/nodes` under the hood
- a live, auto-refreshing table of every node — model, state, tok/s, active
  requests, health — with **Stop**, **Reload** (prompts for a new model
  path), and **Remove** per node. Remove is a real delete (`DELETE
  /v0/nodes/{id}`): unlike a miss-threshold eviction, the node's id is
  blacklisted so mDNS won't silently bring it back — add it again
  (manually, by IP, or automatically once it's back in mDNS range and
  re-added) to undo that.

The dashboard itself holds no secret: it's a static page plus a reverse
proxy to the router under `/api/`, so the token you enter is what the
router and agents actually check.

### Running the dashboard in Docker

The dashboard is the one component that makes sense to containerize — it's
a pure HTTP service with no hardware access to manage (unlike the agent,
which needs to supervise a local `llama-server`). Build and run it from the
repo root (the build needs the whole module, not just `dashboard/`):

```sh
docker build -f dashboard/Dockerfile -t edgeos-dashboard .
docker run -d --name edgeos-dashboard -p 8092:8092 \
  -e EDGEOS_ROUTER_ADDR=http://host.docker.internal:8081 \
  --add-host=host.docker.internal:host-gateway \
  edgeos-dashboard
```

`--add-host`/`host.docker.internal` is how the container reaches a router
running on the host (Docker Desktop and OrbStack both support it on
macOS/Windows; on Linux you may need the container on the host network, or
point `EDGEOS_ROUTER_ADDR` at the host's real IP instead). If the router
runs in its own container too, just use its container/service name instead.

This is single-shared-token auth, not multi-user accounts — see
[docs/BUSINESS_MODEL.md](BUSINESS_MODEL.md) for why that split is
deliberate.

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
