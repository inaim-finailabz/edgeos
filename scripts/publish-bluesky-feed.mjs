#!/usr/bin/env node

const handle = process.env.BLUESKY_HANDLE;
const password = process.env.BLUESKY_APP_PASSWORD;
const pds = (process.env.BLUESKY_PDS || "https://bsky.social").replace(/\/$/, "");
const host = (process.env.BLUESKY_FEED_HOST || "edgeos.finailabz.com").replace(/^https?:\/\//, "").replace(/\/$/, "");
const rkey = process.env.BLUESKY_FEED_RKEY || "energy-clock-ai";

if (!handle || !password) {
  console.error("Set BLUESKY_HANDLE and BLUESKY_APP_PASSWORD in the environment.");
  process.exit(1);
}

async function request(url, init) {
  const response = await fetch(url, init);
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(`${response.status}: ${data.message || JSON.stringify(data)}`);
  return data;
}

const session = await request(`${pds}/xrpc/com.atproto.server.createSession`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ identifier: handle, password }),
});

const record = {
  $type: "app.bsky.feed.generator",
  did: `did:web:${host}`,
  displayName: "Energy Clock AI",
  description: "AI-curated US SPR, Brent oil, gasoline and EIA updates. Ranked through EdgeOS; verify market data before acting.",
  createdAt: new Date().toISOString(),
};

const result = await request(`${session.didDoc?.service?.[0]?.serviceEndpoint || pds}/xrpc/com.atproto.repo.putRecord`, {
  method: "POST",
  headers: { "Content-Type": "application/json", Authorization: `Bearer ${session.accessJwt}` },
  body: JSON.stringify({ repo: session.did, collection: "app.bsky.feed.generator", rkey, record }),
});

const feedUri = `at://${session.did}/app.bsky.feed.generator/${rkey}`;
console.log(JSON.stringify({ feedUri, recordUri: result.uri, installUrl: `https://bsky.app/profile/${handle}/feed/${rkey}` }, null, 2));
