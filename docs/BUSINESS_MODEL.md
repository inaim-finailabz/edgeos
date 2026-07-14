# Business model

## Three tiers, not two: Community, Business, EdgeOS Cloud

The original two-tier framing (free core vs. enterprise SaaS) leaves a real
gap: SMBs with the exact profile this project targets — medical practices,
law firms, accounting firms, or a systems-integration house building a
product on top of EdgeOS for one of those clients — usually want their own
infrastructure (compliance, data residency) but don't have in-house
engineering to install and run it themselves. That's neither "download it
and DIY" (Community) nor "become a SaaS tenant" (EdgeOS Cloud) — it's
**professional services on top of the same free software**:

- **Business** (contact us / custom pricing): on-premise implementation
  (installing and configuring agents on the customer's own hardware,
  integrating with their existing systems) plus an ongoing support
  contract (troubleshooting, updates, monitoring guidance). Still fully
  self-hosted, on the customer's own infrastructure — no multi-tenant
  cloud component, no new software capability at all. This tier sells
  *services*, not features; the product being installed is identical to
  what Community users download themselves. It's the "pure services"
  option from the original three alternatives below, now folded in
  alongside the SaaS tier rather than instead of it.

This sits between the other two:

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
  - **per-person usage**: token/request KPIs and throttling *by individual
    user*. This is not separable from multi-user auth above — v0 has no
    concept of "a person" at all (one shared token), so per-person
    anything is downstream of accounts existing, not a separate feature.
  - multi-site / multi-cluster fleet visibility (v0 is one router's view)
  - audit logs (per-request logging, who-did-what) and SLA-backed support
  - **custom/fine-tuned model deployment** — hosting and serving a
    customer's own trained weights through EdgeOS Cloud. Distinct from the
    others: those are ops/organizational features on top of the same
    software, this is an actual hosting capability (storage, serving
    infra for arbitrary uploaded weights) — closer to the cloud-fallback
    bullet's shape than the auth/ops bullets, but its own line of work.

  Note: several of these (real auth, multi-site orchestration, per-person
  anything) are explicitly out of v0 scope per `CLAUDE.md`. That's fine for
  the OSS core — it's exactly the set of things that becomes roadmap work
  for the SaaS tier once there's enterprise demand pulling on it. The
  differentiator shifted from "has a dashboard at all" to "supports a
  team/org, not just one operator" once the free dashboard shipped.

## Precedent: Ollama's cloud tier

Ollama is the closest comparable — fully open core (MIT), and its
monetization is **Ollama Cloud / Turbo**: a hosted tier that runs models too
big for local hardware, billed per-use, reached through the *same* CLI and
API surface users already have. No separate product, no separate
integration — the free local tool and the paid cloud tier are one
continuous experience.

That maps onto EdgeOS almost exactly, and sharpens the first SaaS bullet
above from "a hosted cloud-fallback endpoint" into something concrete:
**EdgeOS Cloud as the first-party `-cloud-endpoint`.** v0 already has the
mechanism — the router fails over to whatever `-cloud-endpoint` is
configured when no local node qualifies (see `docs/CAPABILITY_SCHEMA.md`).
Today that has to be the user's own OpenAI/Anthropic/etc. key. The Ollama-
style move is to offer an EdgeOS-run endpoint as *a* option for that same
flag — billed per-token or by subscription — so someone who outgrows their
local hardware upgrades by changing one flag, not by adopting a different
product. This is the cleanest version of "the free tool and the paid tier
are the same software": the fallback seam was already there, unmonetized,
and it needs no new integration surface, just an EdgeOS-run endpoint on
the other end of a flag that already exists.

This doesn't replace the enterprise-ops SaaS bullets above (multi-user
auth, multi-site visibility, SLA support) — those target teams. The cloud-
fallback tier targets the same individual/hobbyist user the OSS core
already serves, the moment their own hardware isn't enough. Two different
upgrade paths, both consistent with "the free and paid tiers are the same
software."

## What the Apache-2.0 grant actually covers

Worth being explicit about, since the dashboard shows locked "Enterprise"
items (Multi-user & SSO, Audit logs, Multi-site view, Usage & billing,
Custom models, Priority support): **none of those have any implementation
anywhere in this repository, gated or otherwise.** They are UI-only
previews of a roadmap for a separate, not-yet-built hosted product
(EdgeOS Cloud). There is no license check, no feature flag, no dormant
code path to find or remove — clicking one just opens a modal explaining
it's part of a different product. The Apache-2.0 grant on this repo covers
exactly what's implemented here (agent, router, CLI, dashboard including
that upgrade-modal UI), full stop.

This matters for two reasons: (1) it's honest — nobody should go looking
for enterprise functionality "hidden" in the OSS code, because it isn't
there; and (2) it's the only design that's actually robust. A real
license-gated feature *inside* Apache-2.0 code would be pointless (anyone
can read the check and delete it — see the earlier node-cap discussion)
and would muddy what the license grants. Keeping enterprise functionality
entirely out of this repo, in a separate product, sidesteps both problems:
there's nothing to break because there's nothing here to break.

## Why hybrid over the alternatives

Two of three narrower options that were on the table are rejected outright
(open-core with paid features baked into the core creates OSS/paid tension
inside the same binary; hosted-fallback-only caps revenue to a single seam
rather than the broader enterprise ops surface). The third — pure services
— isn't rejected, it's **folded in as the Business tier above**: services
revenue and SaaS revenue aren't mutually exclusive, they target different
buyers (an SMB that wants someone else to run the install vs. a team that
wants a hosted multi-tenant product), so both stay on the table
simultaneously rather than picking one.

## Sequencing

1. v0 ships fully open, no paid tier yet — the goal right now is developer
   adoption and real usage data, not revenue.
2. **Business (services) can start taking inbound whenever someone asks** —
   it needs no new software, just people-time, so unlike the SaaS tier it
   isn't gated on building anything first.
3. The EdgeOS Cloud SaaS tier gets scoped once there's inbound demand from
   teams asking for the things above (fleet visibility, auth, support) —
   not speculatively ahead of that signal.
