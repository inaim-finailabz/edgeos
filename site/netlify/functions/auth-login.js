const crypto = require("crypto");
const { requireEnv, baseURL } = require("./_session");

// GET /api/auth/login -> redirects to GitHub's OAuth consent screen.
// state is a CSRF nonce: stashed in a short-lived cookie, checked again in
// auth-callback before any token exchange happens.
exports.handler = async (event) => {
  const state = crypto.randomBytes(16).toString("hex");
  const redirectURI = `${baseURL(event.headers)}/api/auth/callback`;

  const authorizeURL = new URL("https://github.com/login/oauth/authorize");
  authorizeURL.searchParams.set("client_id", requireEnv("GITHUB_CLIENT_ID"));
  authorizeURL.searchParams.set("redirect_uri", redirectURI);
  authorizeURL.searchParams.set("scope", "read:user");
  authorizeURL.searchParams.set("state", state);

  return {
    statusCode: 302,
    headers: {
      Location: authorizeURL.toString(),
      "Set-Cookie": `edgeos_oauth_state=${state}; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=300`,
    },
    body: "",
  };
};
