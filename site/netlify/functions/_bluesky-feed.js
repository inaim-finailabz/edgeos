// api.bsky.app is the public Bluesky AppView. The cached public.* hostname
// can return 403 for search from some serverless/cloud egress ranges.
const PUBLIC_BSKY_API = "https://api.bsky.app";
const DEFAULT_QUERIES = [
  '"strategic petroleum reserve"',
  'Brent oil',
  'US gasoline price',
  'EIA petroleum inventory',
];
const POSITIVE_TERMS = [
  "strategic petroleum reserve", "spr", "brent", "crude oil", "gasoline",
  "eia", "energy information administration", "petroleum", "refinery",
  "oil inventory", "barrel", "opec", "iea",
];
const SOURCE_TERMS = ["eia.gov", "energy.gov", "doe.gov", "iea.org", "opec.org", "reuters.com"];
const CACHE_TTL_MS = 2 * 60 * 1000;
const DEFAULT_PUBLISHER_DID = "did:plc:dck5pc6nw3zv2cgqslwtddqa";

let cache = { at: 0, posts: [] };

function json(statusCode, body, extraHeaders = {}) {
  return {
    statusCode,
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      "Cache-Control": statusCode === 200 ? "public, max-age=60" : "no-store",
      ...extraHeaders,
    },
    body: JSON.stringify(body),
  };
}

function feedHost() {
  return (process.env.BLUESKY_FEED_HOST || "edgeos.finailabz.com")
    .replace(/^https?:\/\//, "")
    .replace(/\/$/, "");
}

function serviceDid() {
  return `did:web:${feedHost()}`;
}

function feedUri() {
  return process.env.BLUESKY_FEED_URI || `at://${DEFAULT_PUBLISHER_DID}/app.bsky.feed.generator/energy-clock-ai`;
}

function deterministicScore(post) {
  const text = `${post.record?.text || ""} ${(post.record?.embed?.external?.uri || "")}`.toLowerCase();
  let score = 0;
  for (const term of POSITIVE_TERMS) if (text.includes(term)) score += term.length > 8 ? 3 : 1;
  for (const source of SOURCE_TERMS) if (text.includes(source)) score += 5;
  if (post.author?.handle?.endsWith(".gov")) score += 7;
  if (post.author?.handle === (process.env.BLUESKY_PUBLISHER || "isnai.bsky.social")) score += 10;
  score += Math.min(4, Math.log10(1 + (post.likeCount || 0) + (post.repostCount || 0) * 2));
  return score;
}

function compactPost(post, index) {
  return {
    index,
    author: post.author?.handle || "unknown",
    text: String(post.record?.text || "").slice(0, 500),
    link: post.record?.embed?.external?.uri || null,
    likes: post.likeCount || 0,
    reposts: post.repostCount || 0,
  };
}

function parseRanking(content, maxIndex) {
  const match = String(content || "").match(/\[[\s\S]*\]/);
  if (!match) return [];
  let parsed;
  try { parsed = JSON.parse(match[0]); } catch { return []; }
  if (!Array.isArray(parsed)) return [];
  const seen = new Set();
  return parsed
    .map(item => typeof item === "number" ? item : item?.index)
    .filter(index => Number.isInteger(index) && index >= 0 && index < maxIndex && !seen.has(index) && seen.add(index));
}

async function rankWithEdgeOS(posts) {
  const router = (process.env.EDGEOS_ROUTER_URL || "").replace(/\/$/, "");
  const model = process.env.EDGEOS_MODEL || "";
  if (!router || !model || posts.length === 0) return [];

  const prompt = [
    "Rank these Bluesky posts for an Energy Clock public-information feed.",
    "Prefer verifiable US SPR, EIA petroleum inventory, Brent oil, refinery, and US gasoline-price information.",
    "Prioritize official sources and specific numbers. Reject spam, unsupported predictions, duplicates, partisan bait, and investment advice.",
    "Return ONLY a JSON array of at most 40 objects shaped {\"index\":number,\"score\":0-100}. Best first.",
    JSON.stringify(posts.map(compactPost)),
  ].join("\n\n");

  const headers = { "Content-Type": "application/json" };
  if (process.env.EDGEOS_API_KEY) headers.Authorization = `Bearer ${process.env.EDGEOS_API_KEY}`;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 8000);
  try {
    const response = await fetch(`${router}/v1/chat/completions`, {
      method: "POST",
      headers,
      signal: controller.signal,
      body: JSON.stringify({
        model,
        stream: false,
        temperature: 0,
        max_tokens: 700,
        messages: [{ role: "user", content: prompt }],
      }),
    });
    if (!response.ok) throw new Error(`EdgeOS returned ${response.status}`);
    const body = await response.json();
    return parseRanking(body.choices?.[0]?.message?.content, posts.length);
  } finally {
    clearTimeout(timeout);
  }
}

async function searchPosts(cursor) {
  const configured = (process.env.BLUESKY_FEED_QUERIES || "")
    .split("|").map(s => s.trim()).filter(Boolean);
  const queries = configured.length ? configured : DEFAULT_QUERIES;
  const calls = queries.map(async q => {
    const params = new URLSearchParams({ q, limit: "50", sort: "latest" });
    const response = await fetch(`${PUBLIC_BSKY_API}/xrpc/app.bsky.feed.searchPosts?${params}`);
    if (!response.ok) throw new Error(`Bluesky search returned ${response.status}`);
    return (await response.json()).posts || [];
  });
  const settled = await Promise.allSettled(calls);
  const deduped = new Map();
  for (const result of settled) {
    if (result.status !== "fulfilled") continue;
    for (const post of result.value) {
      const indexedAt = Date.parse(post.indexedAt || post.record?.createdAt || 0);
      if (cursor && indexedAt >= cursor) continue;
      if (post.uri && !deduped.has(post.uri)) deduped.set(post.uri, post);
    }
  }

  // Always include the Energy Clock publisher's own recent posts. This gives
  // the feed a trustworthy backbone and prevents a blank timeline if public
  // search is temporarily unavailable.
  try {
    const params = new URLSearchParams({
      actor: process.env.BLUESKY_PUBLISHER || "isnai.bsky.social",
      limit: "25",
      filter: "posts_no_replies",
    });
    const response = await fetch(`${PUBLIC_BSKY_API}/xrpc/app.bsky.feed.getAuthorFeed?${params}`);
    if (response.ok) {
      const authorFeed = (await response.json()).feed || [];
      for (const item of authorFeed) {
        const post = item.post;
        const indexedAt = Date.parse(post?.indexedAt || post?.record?.createdAt || 0);
        if (post?.uri && (!cursor || indexedAt < cursor)) deduped.set(post.uri, post);
      }
    }
  } catch (error) {
    console.warn("publisher feed fallback unavailable:", error.message);
  }
  return [...deduped.values()];
}

async function curatedPosts(cursor) {
  if (!cursor && Date.now() - cache.at < CACHE_TTL_MS && cache.posts.length) return cache.posts;
  const posts = await searchPosts(cursor);
  let rankedIndices = [];
  try { rankedIndices = await rankWithEdgeOS(posts); } catch (error) { console.warn("EdgeOS ranking fallback:", error.message); }

  const selected = rankedIndices.length
    ? rankedIndices.map(index => posts[index])
    : posts
        .map(post => ({ post, score: deterministicScore(post) }))
        .filter(item => item.score >= 3)
        .sort((a, b) => b.score - a.score || Date.parse(b.post.indexedAt) - Date.parse(a.post.indexedAt))
        .map(item => item.post);
  if (!cursor) cache = { at: Date.now(), posts: selected };
  return selected;
}

module.exports = {
  json, feedHost, serviceDid, feedUri, parseRanking, deterministicScore,
  curatedPosts,
};
