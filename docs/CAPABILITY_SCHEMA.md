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
