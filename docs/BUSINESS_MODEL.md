# Business model

## Decision: hybrid — OSS-led adoption, enterprise SaaS monetization

- **agent, router, CLI, dashboard all stay Apache-2.0**, free, full-featured.
  This is the bottom-up growth engine: individual devs and hobbyists (Pi + a
  GPU box at home) adopt it because it's genuinely useful with zero cost and
  no lock-in, per the README's "vendor-neutral" pitch. The dashboard —
  live fleet view plus single-shared-token stop/reload/evict — ships free;
  it was a deliberate call to put basic single-operator fleet visibility in
  the OSS core rather than holding it back for the paid tier, since it's
  the natural companion to the CLI and core to "genuinely useful."
- **Monetization is a hosted/managed SaaS tier aimed at enterprises**, not a
  feature-gated fork of the core. The free core and the paid tier are the
  same software; the SaaS plan sells the operational and organizational
  parts a single operator doesn't need:
  - hosted cloud-fallback endpoint (the router's existing failover seam —
    see `docs/CAPABILITY_SCHEMA.md`)
  - multi-user auth with accounts/roles/SSO (v0's dashboard auth is one
    shared static token — fine for one operator, not a team)
  - multi-site / multi-cluster fleet visibility (v0 is one router's view)
  - audit logs and SLA-backed support

  Note: several of these (real auth, multi-site orchestration) are
  explicitly out of v0 scope per `CLAUDE.md`. That's fine for the OSS core —
  it's exactly the set of things that becomes roadmap work for the SaaS
  tier once there's enterprise demand pulling on it. The differentiator
  shifted from "has a dashboard at all" to "supports a team/org, not just
  one operator" once the free dashboard shipped.

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
