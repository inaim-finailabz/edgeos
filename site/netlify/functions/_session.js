// Shared session helpers for the GitHub-gated live demo. No dependencies:
// Node's built-in crypto is enough for an HMAC-signed cookie, and the
// Netlify Functions runtime has fetch built in for the GitHub API calls.
const crypto = require("crypto");

const COOKIE_NAME = "edgeos_session";
const SESSION_TTL_SECONDS = 24 * 60 * 60; // 24h

function sign(payload) {
  const secret = requireEnv("SESSION_SECRET");
  const body = Buffer.from(JSON.stringify(payload)).toString("base64url");
  const sig = crypto.createHmac("sha256", secret).update(body).digest("hex");
  return `${body}.${sig}`;
}

function verify(token) {
  if (!token) return null;
  const [body, sig] = token.split(".");
  if (!body || !sig) return null;

  const secret = requireEnv("SESSION_SECRET");
  const expected = crypto.createHmac("sha256", secret).update(body).digest("hex");
  const sigBuf = Buffer.from(sig, "hex");
  const expBuf = Buffer.from(expected, "hex");
  if (sigBuf.length !== expBuf.length || !crypto.timingSafeEqual(sigBuf, expBuf)) {
    return null;
  }

  let payload;
  try {
    payload = JSON.parse(Buffer.from(body, "base64url").toString("utf8"));
  } catch {
    return null;
  }
  if (!payload.exp || Date.now() / 1000 > payload.exp) return null;
  return payload;
}

function makeSessionCookie(user) {
  const payload = { login: user.login, id: user.id, exp: Math.floor(Date.now() / 1000) + SESSION_TTL_SECONDS };
  const token = sign(payload);
  return `${COOKIE_NAME}=${token}; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=${SESSION_TTL_SECONDS}`;
}

function clearSessionCookie() {
  return `${COOKIE_NAME}=; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=0`;
}

function parseCookie(cookieHeader, name) {
  if (!cookieHeader) return null;
  for (const part of cookieHeader.split(";")) {
    const [k, ...rest] = part.trim().split("=");
    if (k === name) return rest.join("=");
  }
  return null;
}

function sessionFromRequest(headers) {
  const token = parseCookie(headers.cookie, COOKIE_NAME);
  return verify(token);
}

function requireEnv(name) {
  const v = process.env[name];
  if (!v) throw new Error(`missing required env var ${name}`);
  return v;
}

// baseURL reconstructs this deploy's own origin from the request, so the
// OAuth redirect_uri is correct on every preview/branch deploy without a
// hardcoded site-URL env var.
function baseURL(headers) {
  const proto = headers["x-forwarded-proto"] || "https";
  const host = headers["x-forwarded-host"] || headers.host;
  return `${proto}://${host}`;
}

module.exports = {
  COOKIE_NAME,
  sign,
  verify,
  makeSessionCookie,
  clearSessionCookie,
  parseCookie,
  sessionFromRequest,
  requireEnv,
  baseURL,
};
