# Energy Clock AI feed for Bluesky

This integration publishes an installable Bluesky custom feed backed by
EdgeOS. Users open the feed in Bluesky and pin it to Home; Bluesky does not
support arbitrary binary plugins inside its official client.

## Runtime flow

1. Bluesky calls `app.bsky.feed.getFeedSkeleton` on this site's HTTPS origin.
2. The Netlify function searches recent public Bluesky energy posts.
3. It asks the configured EdgeOS OpenAI-compatible router to rank the posts.
4. It returns only AT URIs; Bluesky hydrates and renders the posts itself.
5. If EdgeOS is unavailable, a deterministic source/keyword ranker takes over.

## Netlify environment

| Variable | Required | Purpose |
|---|---:|---|
| `BLUESKY_FEED_URI` | no | overrides the default `@isnai.bsky.social` feed URI |
| `BLUESKY_FEED_HOST` | no | defaults to `edgeos.finailabz.com` |
| `BLUESKY_FEED_QUERIES` | no | `|`-separated search queries |
| `BLUESKY_PUBLISHER` | no | trusted publisher included in every feed; defaults to `isnai.bsky.social` |
| `EDGEOS_ROUTER_URL` | recommended | public HTTPS URL of the EdgeOS router |
| `EDGEOS_MODEL` | recommended | loaded model ID used for ranking |
| `EDGEOS_API_KEY` | no | bearer token when the inference endpoint requires one |

Never expose the router's management token. The feed needs inference only.

## Publish or update the feed record

Run locally with an app password, not the main Bluesky password:

```sh
BLUESKY_HANDLE=isnai.bsky.social \
BLUESKY_APP_PASSWORD='...' \
BLUESKY_FEED_HOST=edgeos.finailabz.com \
node scripts/publish-bluesky-feed.mjs
```

The script uses `putRecord`, so it is safe to rerun when the name or description
changes. If publishing from a different account, copy the printed `feedUri` into
Netlify as `BLUESKY_FEED_URI`, then redeploy. The public install page is
`/bluesky.html`.

## Protocol endpoints

- `/.well-known/did.json`
- `/xrpc/app.bsky.feed.describeFeedGenerator`
- `/xrpc/app.bsky.feed.getFeedSkeleton`
