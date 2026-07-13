# Public demo site (marketing + live read-only demo)

`site/` is a static marketing page (hero carousel, features, how-it-works)
plus a GitHub-gated live demo showing real data from a running EdgeOS
stack. Deploys to Netlify; the demo backend is your own machine, reached
through a Cloudflare Tunnel — nothing is port-forwarded on your home router.

## Architecture

```
Browser --> Netlify (site/ + Netlify Functions)
              |  /api/auth/*   -> GitHub OAuth (login/callback/me/logout)
              |  /api/demo/data -> proxies, server-side only, to:
              v
        Cloudflare Tunnel (https://xxxx.trycloudflare.com)
              v
        dashboard -public  (this machine, read-only, no write route works)
              v
        router -> agent -> real llama-server
```

Three independent layers of safety, since this exposes a real device to the
internet:
1. **GitHub sign-in required** before the browser gets any live data at all
   (`/api/demo/data` checks a signed session cookie, returns 401 otherwise).
2. **The tunnel URL is a server-side secret** (`DEMO_BACKEND_URL`, a Netlify
   env var) — the browser never sees it, so nobody can bypass the sign-in
   gate by just finding the tunnel address.
3. **The dashboard itself runs in `-public` mode**: its reverse proxy
   rejects every non-GET request with 403 before it ever reaches the
   router, regardless of the management token. Nothing reachable through
   the tunnel can stop, reload, or remove your node.

## One-time setup (manual — needs your own accounts/credentials)

I can't do these for you: they need your GitHub account and your Netlify
account.

**1. Create a GitHub OAuth App** at
[github.com/settings/developers](https://github.com/settings/developers) →
"New OAuth App":
- Homepage URL: `https://<your-site>.netlify.app`
- Authorization callback URL: `https://<your-site>.netlify.app/api/auth/callback`
- Note the **Client ID**, and generate a **Client Secret**.

**2. Deploy `site/` to Netlify.** `netlify.toml` at the repo root already
points the build at `site/` and `site/netlify/functions`:
```sh
netlify init      # or connect the repo via the Netlify web UI
netlify deploy --prod
```

**3. Set Netlify environment variables** (Site configuration → Environment
variables):
| Variable | Value |
|---|---|
| `GITHUB_CLIENT_ID` | from step 1 |
| `GITHUB_CLIENT_SECRET` | from step 1 — never commit this |
| `SESSION_SECRET` | any long random string, e.g. `openssl rand -hex 32` |
| `DEMO_BACKEND_URL` | the tunnel URL from step 4 below |

**4. Run the demo backend on this machine and tunnel it:**
```sh
export EDGEOS_MANAGEMENT_TOKEN=$(openssl rand -hex 16)   # any value; not exposed publicly
./dist/darwin-arm64/cli up -model ~/.edgeos/models/<your-model>.gguf
./dist/darwin-arm64/router -addr :8081
./dist/darwin-arm64/dashboard -addr :8092 -router http://localhost:8081 -public

cloudflared tunnel --url http://localhost:8092
```
`cloudflared` prints a `https://<random>.trycloudflare.com` URL — put that
in `DEMO_BACKEND_URL` (step 3) and redeploy/trigger a function redeploy so
it picks up the new env var.

Note: a quick `cloudflared tunnel --url` like this is ephemeral — the URL
changes every time you restart it. For a stable long-running demo, set up
a named tunnel tied to a Cloudflare account and a domain you own instead
(`cloudflared tunnel create`); that's a further step beyond this v0 setup.

## What the demo actually shows

Read-only: node id, hostname, platform, accel, model, state, tok/s,
health, plus the fleet KPI strip. No stop/reload/remove/add-node controls
are rendered, and none would work even if someone crafted the request
directly — see the three-layer safety note above.
