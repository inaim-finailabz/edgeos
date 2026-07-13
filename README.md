# EdgeOS

**One API for AI inference across every device you own.**

EdgeOS is an open orchestration layer for Edge AI. Install a small agent on each device — Raspberry Pi, Jetson, Mac, PC — and they become a single OpenAI-compatible endpoint. EdgeOS routes each request to the best available node and falls back to cloud only when nothing local qualifies. Privacy by default, offline-first, vendor-neutral.

> Status: pre-release. Public launch lands with the working demo — watch this repo.

<!-- DEMO GIF GOES HERE: unplug the 4070, stream keeps flowing -->

## Why

Ollama is Docker for LLMs — one machine at a time. EdgeOS is the layer above: discovery, routing, failover, and policy across heterogeneous devices, with cloud as just another tier.

## Architecture (v0)

- **Node agent** — single static binary; mDNS discovery, capability reporting (measured tok/s, ctx limits), engine supervision. Control plane only — never in the token path.
- **Router** — one OpenAI-compatible endpoint; live node table, request scoring, stream proxying, failover.
- **CLI** — `edgeos up`, `edgeos nodes`, `edgeos pull <model>`.

Engine: llama.cpp (CUDA / Metal / Vulkan / CPU). Engines are pluggable by design.

## Measured throughput

| Node | Model | Quant | tok/s |
|---|---|---|---|
| Raspberry Pi 5 16GB | Qwen2.5-3B-Instruct | Q4_K_M | _pending_ |
| Jetson Orin Nano 8GB | Llama-3.1-8B-Instruct | Q4_K_M | _pending_ |
| MacBook M1 16GB | Llama-3.1-8B-Instruct | Q4_K_M | _pending_ |
| RTX 4070 12GB | Qwen2.5-14B-Instruct | Q4_K_M | _pending_ |

## License

Apache-2.0
