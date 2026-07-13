# API reference

Full machine-readable contract: [`docs/openapi.yaml`](openapi.yaml). This
doc is the human-readable companion: auth setup, how routing decisions get
made, and a quickstart in whichever language you use.

## Connecting — pick your language

All of these hit the same endpoint, `POST /v1/chat/completions`, and are
verified working against a live EdgeOS stack:

| Language | File | Notes |
|---|---|---|
| Python | [`examples/python/chat.py`](../examples/python/chat.py) | Official `openai` SDK, unmodified except `base_url` |
| Node.js | [`examples/node/chat.js`](../examples/node/chat.js) | Official `openai` npm SDK, same deal |
| Go | [`examples/go/chat.go`](../examples/go/chat.go) | stdlib only — no official OpenAI Go SDK to lean on |
| Browser | [`examples/web/chat.html`](../examples/web/chat.html) | `fetch()` + manual SSE parsing, no SDK, no build step |
| curl | [`examples/curl/chat.sh`](../examples/curl/chat.sh) | plain HTTP, for scripting or debugging |

The short version, in curl:

```sh
curl -N http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"<id from GET /v0/nodes>","messages":[{"role":"user","content":"hi"}],"stream":true}'
```

`model` must exactly match a model id some node reports as `state:
"loaded"` — check `GET /v0/nodes` (or `edgeos nodes`, or the dashboard)
first. There's no fuzzy matching or aliasing in v0.

## Auth — what needs a token, what doesn't

v0 has exactly one credential: a shared static bearer token
(`-management-token` / `$EDGEOS_MANAGEMENT_TOKEN`), and it only gates
*writes*. Everything read-only, and the inference path itself, is open:

| Endpoint | Auth? |
|---|---|
| `POST /v1/chat/completions` | No |
| `GET /v0/nodes`, `GET /v0/nodes/{id}`, `GET /v0/capabilities` | No |
| `POST /v0/nodes`, `DELETE /v0/nodes/{id}` | **Yes** |
| `POST /v0/nodes/{id}/actions/{action}`, `POST /v0/actions/{stop,reload}` | **Yes** |

**Set it identically on every agent and the router** — the router forwards
your token as-is when it proxies an action to an agent, so a mismatch
fails at the agent, not the router:

```sh
export EDGEOS_MANAGEMENT_TOKEN=$(openssl rand -hex 16)
./agent  -model ...              # picks it up from the env
./router                          # same
```

**Check it's actually working** — an authenticated call should succeed,
and the same call with no header should 401 (not 404 — 404 means the
token isn't configured on that component at all):

```sh
curl -i -X DELETE http://localhost:8081/v0/nodes/doesnotexist \
  -H "Authorization: Bearer $EDGEOS_MANAGEMENT_TOKEN"
# -> 404 "node not found" if the token matched (it checked auth, then looked up the id)
# -> 401 if the token didn't match (wrong value, or header omitted entirely)
```

Separately: if the *router itself* was started with no `-management-token`
at all, every write route 404s regardless of what header you send — that's
the "disabled by default" posture, not an auth failure.

This is intentionally the *only* auth in v0 — no accounts, no roles, no
per-user scoping. See [`docs/BUSINESS_MODEL.md`](BUSINESS_MODEL.md) for why
that's a deliberate line, not an oversight.

## Smart routing — how a request picks a node

`POST /v1/chat/completions` doesn't just proxy — it scores the fleet first
(`router/scoring.go`):

1. **Filter to nodes reporting the requested `model` as `state: "loaded"`.**
   No match anywhere → cloud fallback if configured, else a typed 503.
2. **Filter by context fit.** The router estimates required tokens as
   `(total character count of messages) / 4 + max_tokens` (default 512) —
   a rough heuristic, not a real tokenizer, deliberately: adding a
   tokenizer dependency for a rough ctx-fit check wasn't worth it for v0.
   Any node whose `ctx_max` is smaller than that estimate is dropped.
3. **Rank the rest by `tok_per_sec / (1 + active_requests)`.** `tok_per_sec`
   is the real number from that node's load-time 50-token benchmark —
   never estimated. `active_requests` comes from the agent's own poll of
   its engine's `/slots`, refreshed every 2s. This means an idle node with
   moderate throughput will usually beat a busy node with higher raw
   throughput — the score is about *available* capacity right now, not
   theoretical peak.
4. **Highest score wins.** The request is forwarded to that node's
   `engine.endpoint` unmodified; the response (including an SSE stream) is
   proxied straight through.

If the winning node becomes unreachable *before* any response bytes reach
the client, that's a typed `503` with `Retry-After` — safe for the client
to retry, and the router does not silently try a different node or
restart the stream once one has started. See
[`docs/CAPABILITY_SCHEMA.md`](CAPABILITY_SCHEMA.md) for the full contract
this implements.

## Fleet management

`GET /v0/nodes` (no auth) gives you the full live table plus a KPI
summary — see the `Summary` schema in `openapi.yaml`. `POST /v0/nodes`,
`DELETE /v0/nodes/{id}`, and the per-node `stop`/`reload` actions are the
CRUD + control surface the dashboard is built on; call them directly if
you're scripting fleet management instead of clicking through the UI.
