# Business model — open questions

Nothing here is decided. This is a brainstorm of viable paths and their
tradeoffs, to be revisited once there's usage data to argue from. The core
(agent, router, CLI) is Apache-2.0 per the README regardless of which path
gets picked — the question is what, if anything, sits alongside it.

## Monetization angles considered

### Open-core + hosted cloud fallback
Charge for the cloud endpoint that EdgeOS fails over to when no local node
qualifies. The router already has a natural seam for this (`docs/CAPABILITY_SCHEMA.md`'s
fallback path) — a hosted option could be one line of router config away.
- *For*: usage-based, low friction, doesn't touch the OSS core's value prop.
- *Against*: only monetizes the failure case; if local coverage is good,
  there's little to charge for.

### Open-core + paid enterprise features
Core stays free; a paid tier adds fleet management, auth/policy, multi-site
orchestration.
- *For*: standard open-core playbook, clear upgrade path for larger orgs.
- *Against*: several of the obvious paid features (auth, GPU scheduling,
  registry sync) are explicitly out of v0 scope per `CLAUDE.md` — this path
  implies a roadmap decision, not just a pricing decision.

### Support / services only
Stay fully open source; revenue from support contracts, integration
consulting, managed deployments on the customer's own hardware.
- *For*: no OSS/paid tension, easiest to reconcile with "vendor-neutral" in
  the README's pitch.
- *Against*: doesn't scale the way product revenue does; ties revenue to
  headcount.

## Target customer segments considered

- **Individual devs / hobbyists** — bottom-up OSS adoption (Pi + a GPU box
  at home), monetization comes later if at all.
- **Startups/SMBs with edge hardware** — retail, robotics, IoT teams who
  want inference orchestration without building it in-house.
- **Enterprises with data residency needs** — on-prem/private inference for
  compliance, cloud strictly as fallback; higher touch, longer sales cycle,
  and the auth/policy gap in v0 matters most here.

## Why this is unresolved on purpose

Picking a segment or pricing model now would be guessing ahead of any usage
signal. The more useful near-term move is probably: ship v0, see who shows
up and how they use the fallback path, then revisit this doc with real
data instead of priors.
