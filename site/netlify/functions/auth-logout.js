const { clearSessionCookie } = require("./_session");

exports.handler = async () => ({
  statusCode: 302,
  headers: { Location: "/", "Set-Cookie": clearSessionCookie() },
  body: "",
});
