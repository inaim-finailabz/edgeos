# edgeos-dashboard (Go)
Read-only fleet view plus stop/reload/evict management actions, proxied to the router. Static HTML/JS, no build step, no new dependency. See docs/CAPABILITY_SCHEMA.md for the management API and auth model.

Also runs in Docker (`Dockerfile`, build from the repo root) — see docs/GETTING_STARTED.md. It's the one component worth containerizing; the agent needs direct hardware access to supervise `llama-server` so it isn't.
