const { json, feedUri, curatedPosts } = require("./_bluesky-feed");

exports.handler = async (event) => {
  if (event.httpMethod !== "GET") return json(405, { error: "MethodNotAllowed" }, { Allow: "GET" });
  const expected = feedUri();
  if (!expected) return json(503, { error: "FeedNotConfigured", message: "BLUESKY_FEED_URI is not set" });

  const requested = event.queryStringParameters?.feed;
  if (requested !== expected) return json(404, { error: "UnknownFeed" });
  const limit = Math.max(1, Math.min(100, Number(event.queryStringParameters?.limit) || 50));
  const rawCursor = event.queryStringParameters?.cursor;
  const cursor = rawCursor && /^\d+$/.test(rawCursor) ? Number(rawCursor) : null;

  try {
    const posts = (await curatedPosts(cursor)).slice(0, limit);
    const feed = posts.map(post => ({ post: post.uri }));
    const last = posts.at(-1);
    const nextCursor = last ? String(Date.parse(last.indexedAt || last.record?.createdAt)) : undefined;
    return json(200, nextCursor ? { cursor: nextCursor, feed } : { feed });
  } catch (error) {
    console.error("feed generation failed", error);
    return json(502, { error: "FeedGenerationFailed", message: "Could not build the feed" });
  }
};
