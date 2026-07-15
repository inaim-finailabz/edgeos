const { json, serviceDid, feedUri } = require("./_bluesky-feed");

exports.handler = async () => {
  const uri = feedUri();
  if (!uri) return json(503, { error: "FeedNotConfigured", message: "BLUESKY_FEED_URI is not set" });
  return json(200, { did: serviceDid(), feeds: [{ uri }] });
};
