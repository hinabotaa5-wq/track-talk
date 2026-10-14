import { createHmac } from "node:crypto";
import { createServer as createHttpServer } from "node:http";

const secret = process.env.SUPABASE_JWT_SECRET ?? "test-jwt-secret-for-ci-only";
const userID = "11111111-1111-4111-8111-111111111111";

function accessToken() {
  const header = Buffer.from(JSON.stringify({ alg: "HS256", typ: "JWT" })).toString("base64url");
  const now = Math.floor(Date.now() / 1000);
  const payload = Buffer.from(
    JSON.stringify({
      sub: userID,
      aud: "authenticated",
      role: "authenticated",
      exp: now + 3600,
      iat: now,
      auth_time: now,
    }),
  ).toString("base64url");
  const data = `${header}.${payload}`;
  const sig = createHmac("sha256", secret).update(data).digest("base64url");
  return `${data}.${sig}`;
}

const user = {
  id: userID,
  aud: "authenticated",
  role: "authenticated",
  email: "runner@example.com",
  app_metadata: { provider: "email", providers: ["email"] },
  user_metadata: {},
  identities: [],
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
};

const server = createHttpServer((req, res) => {
  const origin = req.headers.origin ?? "http://localhost:3000";
  res.setHeader("access-control-allow-origin", origin);
  res.setHeader("access-control-allow-headers", req.headers["access-control-request-headers"] ?? "*");
  res.setHeader("access-control-allow-methods", "GET,POST,OPTIONS");
  res.setHeader("vary", "Origin");
  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }
  const path = new URL(req.url ?? "/", "http://127.0.0.1").pathname;
  if (req.method === "POST" && path === "/auth/v1/token") {
    res.writeHead(200, { "content-type": "application/json" });
    res.end(
      JSON.stringify({
        access_token: accessToken(),
        token_type: "bearer",
        expires_in: 3600,
        expires_at: Math.floor(Date.now() / 1000) + 3600,
        refresh_token: "refresh-token",
        user,
      }),
    );
    return;
  }
  if (req.method === "GET" && path === "/auth/v1/user") {
    res.writeHead(200, { "content-type": "application/json" });
    res.end(JSON.stringify(user));
    return;
  }
  res.writeHead(404, { "content-type": "application/json" });
  res.end(JSON.stringify({ error: "not_found" }));
});

server.listen(54321, "127.0.0.1");
