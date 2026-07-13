# EdgeOS capability schema — v0

Discovery: mDNS service `_edgeos._tcp.local`, TXT: `v=0`, `id=<short-uuid>`, `cap=/v0/capabilities`.
Full state is pulled over HTTP from the agent. mDNS is only the doorbell.

`GET /v0/capabilities`:

```json
{
  "schema": "edgeos/v0",
  "node": { "id": "a1b2c3d4", "hostname": "orin-nano", "platform": "linux/arm64",
            "accel": "cuda", "mem_total_mb": 7620, "mem_free_mb": 4100 },
  "engine": { "kind": "llama.cpp", "endpoint": "http://192.168.1.42:8080", "healthy": true },
  "models": [ { "id": "llama-3.1-8b-instruct-q4_k_m", "state": "loaded",
                "ctx_max": 8192, "tok_per_sec": 11.4 } ],
  "load": { "active_requests": 0, "queue_depth": 0 }
}
```

Rules:
- `tok_per_sec` is measured at model load (50-token benchmark), never estimated.
- Router polls every 2s; 3 misses = node evicted.
- Router talks directly to `engine.endpoint`; the agent is never in the token path.
- `queue_depth` is always `0` in v0: llama-server has no native queue-depth
  metric to report. `active_requests` is real (from the engine's slots).

## Router API

`POST /v1/chat/completions` — OpenAI-compatible. The router reads only
`model`, `messages`, and `max_tokens` from the body to score and route the
request; the rest is forwarded to the winning node (or the cloud fallback)
unmodified, and the response — including an SSE stream — is proxied
straight through.

Scoring: among nodes reporting the requested `model` as `state: "loaded"`
with `ctx_max` large enough for a v0 heuristic token estimate (~4 chars/token
of the messages, plus `max_tokens`), pick the highest
`tok_per_sec / (1 + active_requests)`. No real tokenizer is used in v0.

Failover: if no local node qualifies and a cloud fallback is configured, the
request goes there instead. If nothing qualifies at all, or the chosen
upstream is unreachable before any response bytes are sent, the router
returns a typed JSON error (OpenAI-style `{"error": {"type", "message"}}`)
with `Retry-After` and lets the client retry. If a stream has already
started when the upstream connection drops, the router does **not** retry
or restart it — the response just ends. This is the hard rule from
`CLAUDE.md`: no silent stream restart.

`GET /v0/nodes` — the router's live fleet view:

```json
{ "nodes": [ { "id": "a1b2c3d4", "cap_url": "http://192.168.1.42:8090/v0/capabilities",
               "cap": { /* same shape as GET /v0/capabilities above */ },
               "last_seen": "2026-07-13T10:03:06Z", "misses": 0 } ] }
```

Used by `edgeos nodes`; not part of the agent contract above.
