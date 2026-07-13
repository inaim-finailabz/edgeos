# Business model

## Decision: hybrid — OSS-led adoption, enterprise SaaS monetization

- **agent, router, CLI stay Apache-2.0**, free, full-featured. This is the
  bottom-up growth engine: individual devs and hobbyists (Pi + a GPU box at
  home) adopt it because it's genuinely useful with zero cost and no
  lock-in, per the README's "vendor-neutral" pitch.
- **Monetization is a hosted/managed SaaS tier aimed at enterprises**, not a
  feature-gated fork of the core. The free core and the paid tier are the
  same software; the SaaS plan sells the operational parts individuals
  don't want to run themselves:
  - hosted cloud-fallback endpoint (the router's existing failover seam —
    see `docs/CAPABILITY_SCHEMA.md`)
  - fleet management / multi-site visibility across many agents
  - auth and policy controls
  - SLA-backed support

  Note: several of these (auth, multi-site/fleet orchestration) are
  explicitly out of v0 scope per `CLAUDE.md`. That's fine for the OSS core —
  it's exactly the set of things that becomes roadmap work for the SaaS
  tier once there's enterprise demand pulling on it. Don't build them into
  the core to avoid a paid/free feature split.

## Why hybrid over the alternatives

This supersedes three narrower options that were on the table (open-core +
paid features baked into the core, hosted-fallback-only, and pure
services): those either created OSS/paid tension inside the same binary, or
capped revenue to a single seam (the fallback path) rather than the
broader enterprise ops surface (fleet, auth, support).

## Sequencing

1. v0 ships fully open, no paid tier yet — the goal right now is developer
   adoption and real usage data, not revenue.
2. The enterprise SaaS tier gets scoped once there's inbound demand from
   teams asking for the things above (fleet visibility, auth, support) —
   not speculatively ahead of that signal.
