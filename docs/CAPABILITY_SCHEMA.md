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

`POST /v0/parallel-completions` — fan out N independent chat completions
concurrently (`{"requests": [...]}`, max 32), each scored and routed like a
standalone `/v1/chat/completions` call. Splitting a task into sub-prompts
and combining results is the caller's job, not EdgeOS's — this is a
fan-out primitive, not a prompt-engineering framework. Always
non-streaming (each sub-request's `stream` is forced `false`); one entry
failing doesn't fail the others. See `docs/API_REFERENCE.md` for the full
request/response shape.

`GET /v0/nodes` — the router's live fleet view plus a KPI summary:

```json
{ "nodes": [ { "id": "a1b2c3d4", "cap_url": "http://192.168.1.42:8090/v0/capabilities",
               "cap": { /* same shape as GET /v0/capabilities above */ },
               "last_seen": "2026-07-13T10:03:06Z", "misses": 0 } ],
  "summary": { "total_nodes": 1, "healthy_nodes": 1, "total_active_requests": 0,
               "total_tok_per_sec": 11.4, "distinct_models": 1,
               "requests_served_local": 0, "requests_served_cloud": 0,
               "requests_failed": 0 } }
```

`summary` is computed fresh from the node list on every request, plus three
router-level request counters (reset on restart, not persisted — good
enough for "since this router started" in v0). Used by `edgeos nodes` and
the dashboard; not part of the agent contract above.

## Node CRUD + management API (dashboard)

Real CRUD for the fleet, plus the engine actions. These are the only
authenticated endpoints in v0 — everything above (`/v0/capabilities`,
`/v1/chat/completions`, `GET /v0/nodes`) stays open, unchanged. This is a
deliberate, narrow exception to "no auth in v0" in `CLAUDE.md`: one shared
static bearer token gates writes, nothing more (no accounts, no roles). Set
the identical token via `-management-token` (or `$EDGEOS_MANAGEMENT_TOKEN`)
on every agent and on the router; leaving it unset on a component disables
its management routes (404), which is the default.

Router, requires `Authorization: Bearer <token>`:
- `POST /v0/nodes` — Create. Body is `{"cap_url": "..."}` or
  `{"host": "...", "port": N}` (port defaults to 8090). Fetches that
  address's capabilities once synchronously to validate it's reachable and
  learn its real node id; registers it exactly like an mDNS discovery
  would, and clears any prior removal for that id. This is for nodes mDNS
  can't reach — a different subnet, a cloud box, wherever multicast
  doesn't — not a general substitute for discovery.
- `GET /v0/nodes/{id}` — Read one (unauthenticated, like `GET /v0/nodes`).
- `DELETE /v0/nodes/{id}` — Delete. Removes the node immediately *and*
  blacklists its id, so mDNS rediscovery won't silently bring it back —
  unlike a miss-threshold eviction, this is permanent until the id is
  re-added (manually, or by clearing the blacklist another way).

Agent, requires the same token:
- `POST /v0/actions/stop` — stop the running engine. Safe if already stopped.
- `POST /v0/actions/reload` — `{"model_path": "..."}`; stops any running
  engine and starts a new one against that path, re-running the load-time
  benchmark. Same codepath as initial startup. This is the CRUD "Update" —
  capability data itself is never user-editable, only always refreshed by
  polling, so mutating the engine is the only meaningful update in v0.

Router, requires the same token, proxies the two above to the named node's
agent:
- `POST /v0/nodes/{id}/actions/stop`
- `POST /v0/nodes/{id}/actions/reload` — body forwarded unmodified.

The dashboard itself holds no token server-side: it's a static file server
plus a transparent reverse proxy to the router under `/api/` (e.g.
`/api/v0/nodes` → router's `GET /v0/nodes`), forwarding whatever
`Authorization` header the browser sends. The operator enters the token
once in the page (stored in the browser's `localStorage`); the router and
agents are what actually verify it.
