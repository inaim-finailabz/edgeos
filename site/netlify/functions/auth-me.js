const { sessionFromRequest } = require("./_session");

// GET /api/auth/me -> whether the browser holds a valid session, and who as.
exports.handler = async (event) => {
  const session = sessionFromRequest(event.headers);
  return {
    statusCode: 200,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(
      session ? { authenticated: true, login: session.login } : { authenticated: false }
    ),
  };
};
