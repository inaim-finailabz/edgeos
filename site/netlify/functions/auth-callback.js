const { requireEnv, baseURL, parseCookie, makeSessionCookie } = require("./_session");

// GET /api/auth/callback?code=...&state=... -> exchanges the code for a
// GitHub access token server-side (the client secret never reaches the
// browser), fetches the user's identity, sets a signed session cookie, and
// discards the access token -- it's only needed once, to confirm identity.
exports.handler = async (event) => {
  const { code, state } = event.queryStringParameters || {};
  const expectedState = parseCookie(event.headers.cookie, "edgeos_oauth_state");

  if (!code || !state || !expectedState || state !== expectedState) {
    return { statusCode: 400, body: "invalid or missing OAuth state" };
  }

  const tokenResp = await fetch("https://github.com/login/oauth/access_token", {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({
      client_id: requireEnv("GITHUB_CLIENT_ID"),
      client_secret: requireEnv("GITHUB_CLIENT_SECRET"),
      code,
      redirect_uri: `${baseURL(event.headers)}/api/auth/callback`,
    }),
  });
  const tokenData = await tokenResp.json();
  if (!tokenData.access_token) {
    return { statusCode: 502, body: "GitHub did not return an access token" };
  }

  const userResp = await fetch("https://api.github.com/user", {
    headers: {
      Authorization: `Bearer ${tokenData.access_token}`,
      "User-Agent": "edgeos-demo",
      Accept: "application/vnd.github+json",
    },
  });
  if (!userResp.ok) {
    return { statusCode: 502, body: "GitHub did not return a user profile" };
  }
  const user = await userResp.json();

  return {
    statusCode: 302,
    headers: {
      Location: "/",
      "Set-Cookie": makeSessionCookie({ login: user.login, id: user.id }),
    },
    body: "",
  };
};
