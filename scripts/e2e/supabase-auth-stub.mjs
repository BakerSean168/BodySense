#!/usr/bin/env node
// CI-only Supabase Auth wire-protocol double. It does not contact Supabase,
// persist users, or accept public traffic. Existing Playwright tests use it
// instead of creating hundreds of durable users in the real hosted project.
import { createServer } from "node:http";
import {
  generateKeyPairSync,
  randomUUID,
  sign as cryptoSign,
} from "node:crypto";

if (
  process.env.BODYSENSE_E2E_STUB_AUTH !== "1" ||
  process.env.APP_ENV !== "test"
) {
  throw new Error(
    "Supabase Auth stub requires both APP_ENV=test and BODYSENSE_E2E_STUB_AUTH=1",
  );
}

const port = Number(process.env.BODYSENSE_E2E_SUPABASE_PORT || 18120);
if (!Number.isSafeInteger(port) || port < 1024 || port > 65535) {
  throw new Error("invalid local-only Auth stub port");
}

const issuer = `http://127.0.0.1:${port}/auth/v1`;
const keyId = "bodysense-ci-supabase-es256";
const keys = generateKeyPairSync("ec", { namedCurve: "prime256v1" });
const jwk = keys.publicKey.export({ format: "jwk" });
const users = new Map();
const byId = new Map();
const refreshTokens = new Map();

function json(w, status, value) {
  const data = JSON.stringify(value);
  w.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(data),
    "Cache-Control": "no-store",
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Headers":
      "apikey, authorization, content-type, x-client-info, x-supabase-api-version",
    "Access-Control-Allow-Methods": "GET,POST,DELETE,OPTIONS",
  });
  w.end(data);
}
function token(payload) {
  const header = Buffer.from(
    JSON.stringify({ alg: "ES256", typ: "JWT", kid: keyId }),
  ).toString("base64url");
  const body = Buffer.from(JSON.stringify(payload)).toString("base64url");
  const signed = `${header}.${body}`;
  const sig = cryptoSign(null, Buffer.from(signed), {
    key: keys.privateKey,
    dsaEncoding: "ieee-p1363",
  });
  return `${signed}.${sig.toString("base64url")}`;
}
function claims(user) {
  const now = Math.floor(Date.now() / 1000);
  return {
    iss: issuer,
    sub: user.id,
    aud: "authenticated",
    role: "authenticated",
    email: user.email,
    iat: now,
    exp: now + 900,
    is_anonymous: false,
  };
}
function publicUser(user) {
  const now = new Date().toISOString();
  return {
    id: user.id,
    aud: "authenticated",
    role: "authenticated",
    email: user.email,
    email_confirmed_at: now,
    confirmed_at: now,
    created_at: now,
    updated_at: now,
    last_sign_in_at: now,
    app_metadata: { provider: "email", providers: ["email"] },
    user_metadata: {},
    identities: [],
  };
}
function newSession(user) {
  const refresh = randomUUID();
  refreshTokens.set(refresh, user.id);
  return {
    access_token: token(claims(user)),
    token_type: "bearer",
    expires_in: 900,
    expires_at: Math.floor(Date.now() / 1000) + 900,
    refresh_token: refresh,
    user: publicUser(user),
  };
}
async function body(req) {
  let size = 0;
  const chunks = [];
  for await (const chunk of req) {
    size += chunk.length;
    if (size > 16384) throw new Error("body too large");
    chunks.push(chunk);
  }
  const data = Buffer.concat(chunks).toString("utf8");
  return data ? JSON.parse(data) : {};
}
function authenticatedUser(req) {
  const bearer = req.headers.authorization?.replace(/^Bearer /, "") || "";
  const encoded = bearer.split(".")[1];
  if (!encoded) return null;
  try {
    const p = JSON.parse(Buffer.from(encoded, "base64url").toString("utf8"));
    return byId.get(p.sub) || null;
  } catch {
    return null;
  }
}

const server = createServer(async (req, res) => {
  try {
    if (req.method === "OPTIONS") {
      return json(res, 200, {});
    }
    const uri = new URL(req.url, issuer);
    if (
      uri.pathname === "/auth/v1/.well-known/jwks.json" &&
      req.method === "GET"
    ) {
      return json(res, 200, {
        keys: [{ ...jwk, kid: keyId, alg: "ES256", use: "sig" }],
      });
    }
    if (uri.pathname === "/auth/v1/settings" && req.method === "GET") {
      return json(res, 200, {
        mailer_autoconfirm: true,
        external: { email: true },
      });
    }
    if (uri.pathname === "/auth/v1/signup" && req.method === "POST") {
      const input = await body(req);
      const email = String(input.email || "")
        .trim()
        .toLowerCase();
      const password = String(input.password || "");
      if (!email.includes("@") || password.length < 8) {
        return json(res, 400, {
          code: "validation_failed",
          msg: "Invalid signup",
        });
      }
      if (users.has(email)) {
        return json(res, 422, {
          code: "user_already_exists",
          msg: "Already registered",
        });
      }
      const user = { id: randomUUID(), email, password };
      users.set(email, user);
      byId.set(user.id, user);
      return json(res, 200, newSession(user));
    }
    if (uri.pathname === "/auth/v1/token" && req.method === "POST") {
      const input = await body(req);
      const kind = uri.searchParams.get("grant_type");
      let user;
      if (kind === "password") {
        user = users.get(
          String(input.email || "")
            .trim()
            .toLowerCase(),
        );
        if (!user || user.password !== String(input.password || "")) {
          return json(res, 400, {
            code: "invalid_credentials",
            msg: "Invalid credentials",
          });
        }
      } else if (kind === "refresh_token") {
        const refresh = String(input.refresh_token || "");
        const id = refreshTokens.get(refresh);
        refreshTokens.delete(refresh);
        user = byId.get(id);
        if (!user)
          return json(res, 400, {
            code: "invalid_refresh_token",
            msg: "Invalid refresh",
          });
      } else {
        return json(res, 400, {
          code: "unsupported_grant_type",
          msg: "Invalid grant",
        });
      }
      return json(res, 200, newSession(user));
    }
    if (uri.pathname === "/auth/v1/user" && req.method === "GET") {
      const user = authenticatedUser(req);
      if (!user) return json(res, 401, { msg: "Not authenticated" });
      return json(res, 200, publicUser(user));
    }
    if (uri.pathname === "/auth/v1/logout" && req.method === "POST") {
      return json(res, 200, {});
    }
    const match = uri.pathname.match(
      /^\/auth\/v1\/admin\/users\/([0-9a-f-]+)$/,
    );
    if (match && req.method === "DELETE") {
      if (
        req.headers.apikey !== "ci-supabase-admin-only" ||
        req.headers.authorization !== "Bearer ci-supabase-admin-only"
      ) {
        return json(res, 403, { msg: "Forbidden" });
      }
      const user = byId.get(match[1]);
      if (!user) return json(res, 404, { msg: "Not found" });
      byId.delete(user.id);
      users.delete(user.email);
      res.writeHead(204);
      return res.end();
    }
    if (uri.pathname === "/health") return json(res, 200, { ok: true });
    return json(res, 404, { msg: "Not found" });
  } catch {
    return json(res, 400, { code: "request_failed", msg: "Bad request" });
  }
});

server.listen(port, "127.0.0.1", () => {
  console.log(`CI-only Supabase Auth stub ready at http://127.0.0.1:${port}`);
});
