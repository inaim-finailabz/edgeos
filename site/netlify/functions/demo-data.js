const { sessionFromRequest, requireEnv } = require("./_session");

// GET /api/demo/data -> the live fleet KPIs, but only for a signed-in
// visitor. DEMO_BACKEND_URL (the cloudflared tunnel to the -public
// dashboard) is a server-side-only env var: the browser never sees it, so
// nobody can bypass the GitHub gate by just finding the tunnel URL.
exports.handler = async (event) => {
  const session = sessionFromRequest(event.headers);
  if (!session) {
    return { statusCode: 401, body: JSON.stringify({ error: "sign in required" }) };
  }

  const backend = requireEnv("DEMO_BACKEND_URL").replace(/\/$/, "");
  const resp = await fetch(`${backend}/api/v0/nodes`);
  if (!resp.ok) {
    return { statusCode: 502, body: JSON.stringify({ error: "demo backend unreachable" }) };
  }
  const data = await resp.json();

  return {
    statusCode: 200,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  };
};
