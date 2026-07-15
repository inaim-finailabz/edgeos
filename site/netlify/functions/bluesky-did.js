const { json, feedHost, serviceDid } = require("./_bluesky-feed");

exports.handler = async () => json(200, {
  "@context": ["https://www.w3.org/ns/did/v1"],
  id: serviceDid(),
  service: [{
    id: "#bsky_fg",
    type: "BskyFeedGenerator",
    serviceEndpoint: `https://${feedHost()}`,
  }],
});
