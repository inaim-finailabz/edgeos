# Getting started (v0, pre-release)

This walks through building and running the agent on a device today, and is
explicit about what's wired up versus what's still a stub — v0 is under
active development, tracked in the repo's commit history.

## What works right now

- `agent` builds and serves `GET /v0/capabilities` per
  [docs/CAPABILITY_SCHEMA.md](CAPABILITY_SCHEMA.md).
- The response is schema-correct but **not yet live data**: `node.hostname`
  and `node.platform` are read from the machine; `accel`, memory, the models
  list, and `tok_per_sec` are hardcoded placeholders.

## What's not wired up yet

- mDNS announce/discovery (`_edgeos._tcp.local`).
- Supervising a real `llama-server` process and reporting its actual state.
- The load-time 50-token benchmark that measures real `tok_per_sec`.
- `router` and `cli` are empty scaffolds — no request routing yet.

If you're integrating a device today, you're validating the capability
contract and build, not yet getting live routing — that's the honest state
of things until the items above land.

## Prerequisites

- Go 1.25+
- (optional, for `benchmarks/run.sh`) a running
  [llama.cpp](https://github.com/ggml-org/llama.cpp) `llama-server` and `jq`

## Build

```sh
make build          # agent, router, cli for your local OS/arch -> dist/
make cross           # all three for linux/arm64, linux/amd64, darwin/arm64
```

Cross-compiled binaries land in `dist/<os>-<arch>/<component>` — copy the
one matching your device (Pi 5 -> `linux-arm64`, Jetson Orin Nano ->
`linux-arm64`, Mac -> `darwin-arm64`, 4070 box -> `linux-amd64`).

## Run the agent

```sh
./dist/agent -addr :8090
```

Then from another terminal, or from another machine on the network:

```sh
curl http://<device-ip>:8090/v0/capabilities
```

You should get back JSON matching the shape in
[docs/CAPABILITY_SCHEMA.md](CAPABILITY_SCHEMA.md). That response is what the
router will eventually poll every 2s to build its live node table.

## Benchmark a real llama-server

Once you have `llama-server` running with a model loaded:

```sh
./benchmarks/run.sh -u http://localhost:8080 -n 50
```

This extracts measured prompt-eval and generation tok/s from the server's
own `timings` block — the same mechanism the agent's load-time benchmark
will use once it supervises `llama-server` directly, per
[CLAUDE.md](../CLAUDE.md).

## Running the test suite

```sh
make check    # gofmt check, go vet, go test
```

## Questions / contributing

v0 scope is intentionally narrow — see [CLAUDE.md](../CLAUDE.md) for what's
explicitly out of scope (registry sync, vector store, GPU scheduling,
phone-as-node, auth). Open an issue before building against anything not
listed here; the schema doc and this guide will be updated as each piece
lands.
