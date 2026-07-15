const test = require("node:test");
const assert = require("node:assert/strict");
const { parseRanking, deterministicScore } = require("./_bluesky-feed");

test("parseRanking extracts valid unique indices", () => {
  assert.deepEqual(parseRanking('result: [{"index":2,"score":99},{"index":0,"score":80},{"index":2,"score":1}]', 3), [2, 0]);
  assert.deepEqual(parseRanking("not json", 2), []);
});

test("official energy evidence outranks generic chatter", () => {
  const official = { author: { handle: "energy.gov" }, record: { text: "EIA Strategic Petroleum Reserve inventory update", embed: { external: { uri: "https://eia.gov/petroleum" } } } };
  const generic = { author: { handle: "person.test" }, record: { text: "oil looks interesting" } };
  assert.ok(deterministicScore(official) > deterministicScore(generic));
});
