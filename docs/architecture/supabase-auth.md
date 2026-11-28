# BodySense: hosted Supabase Auth boundary

**Issue:** #491 (2026-11-02). BodySense has no live account data that needs migration. The existing PostgreSQL database remains the source of truth for health and clinical/business records; Supabase Auth alone handles password/login/session.

## Runtime boundaries

| Surface | Authority |
| --- | --- |
| React sign-up/sign-in/logout/session rotation | `@supabase/supabase-js` against hosted Auth |
| Human verification (sign-up only) | ALTCHA proof-of-work: the proof rides in signUp `user_metadata` and the Go Before User Created hook verifies it (signature, expiry, Redis replay burn) before the account exists. Sign-in carries no captcha and relies on Supabase's own rate limits |
| Access JWT signature and issuer/audience/subject | Go verifies ES256 via project JWKS |
| User ID, health records, authorization and `operator` role | BodySense Go + PostgreSQL |
| Full-account erasure | Durable Go privacy workflow, which first deletes the hosted Supabase identity with a server-only secret |

**JWT trust:** only ES256, configured issuer `$SUPABASE_URL/auth/v1`, audience `authenticated`, signed `sub` UUID, nonempty email, nonanonymous and user `role=authenticated` are accepted. JWT public signing keys come from the hosted JWKS endpoint with bounded caching. Local roles are NOT inferred from JWT metadata. A valid new `sub` is upserted as `public.users.id`; the old local `password_hash` is removed by migration `000002`. Existing patient/consultation/AI contracts continue to use that UUID.

A JWT already issued by Supabase may remain verifiable after logout until its `exp`; unlike the old Redis family cache there is no blanket *instant* server-side revocation. The default Supabase JS browser session persists access and refresh tokens in localStorage; this differs from the former HttpOnly refresh-cookie boundary and makes strong CSP/XSS prevention and dependency hygiene important. Set **JWT expiry to 15 minutes or less** in the Supabase project before public usage. The immutable local privacy-erasure tombstone blocks the still-valid JWT after account deletion.

## Dashboard configuration required

1. **Supabase URL:** `https://qdezojzgajsunjwthifv.supabase.co`. Public publishable API key may be embedded in React; service-role or `sb_secret_` **must never** be compiled or committed into browser assets.
2. **Supabase Authentication > Sign In / Providers > Email:** email-password enabled. **Confirm email disabled for development only** (verified through `/auth/v1/settings`: `external.email=true`, `mailer_autoconfirm=true`). This permits registering an address the user does not own; it is not equivalent to verified email. Require verified email + SMTP before general public signups.
3. **Supabase Authentication > Sessions:** set JWT lifetime to <=15 minutes before production use.
4. **ALTCHA human verification (replaces Cloudflare Turnstile):** the register page runs a self-hosted proof-of-work widget. The Go API issues challenges at `GET /api/v1/captcha/challenge` and verifies the solved proof inside a Supabase **Before User Created** HTTP hook (`POST /api/v1/auth-hooks/before-user-created`) before any account exists. Dashboard steps: under **Authentication > Hooks (Beta)**, add a Before User Created hook pointing at that endpoint and record the issued hook secret. Set `ALTCHA_HMAC_KEY` (self-generated long random value) and `BEFORE_USER_CREATED_HOOK_SECRET` (the dashboard-issued secret) in the Go API environment only — never in browser assets. **Attack Protection CAPTCHA stays disabled**: Supabase's built-in CAPTCHA accepts only Turnstile/hCaptcha tokens, which this deployment no longer issues. Sign-in is not captcha-gated; keep Supabase's built-in limits under **Authentication > Rate Limits** tightened instead.
5. **Web bundle:** no captcha configuration is baked into the browser assets. The widget fetches its challenge from the Go API at runtime, so rotating `ALTCHA_HMAC_KEY` needs only an API restart.
6. **Go permanent account deletion:** set a separate **`SUPABASE_SECRET_KEY`** only in the protected API server environment (Supabase Project Settings > API Keys > Secret). The browser must not read it. If absent, `/api/v1/privacy/erasure` fails closed rather than falsely declaring an identity permanently deleted.
7. **Supabase URL Configuration:** add all approved browser origins and callback URLs as needed. Password recovery is not enabled until a real email sender is configured.

## Local development

Use an ignored local environment file (`.env.dev.local` in the BodySense repo or appropriate dev host) for public browser values:

```dotenv
SUPABASE_URL=https://qdezojzgajsunjwthifv.supabase.co
VITE_SUPABASE_URL=https://qdezojzgajsunjwthifv.supabase.co
VITE_SUPABASE_PUBLISHABLE_KEY=<paste your sb_publishable_ key>
# Supabase server-side secret is optional for basic login, but REQUIRED to delete accounts:
SUPABASE_SECRET_KEY=
```

The ALTCHA secrets belong to the Go API environment (never the browser): `ALTCHA_HMAC_KEY` signs and verifies challenges, and `BEFORE_USER_CREATED_HOOK_SECRET` is the secret Supabase issues when the Before User Created hook is created. While either is unset the challenge endpoint answers 503 and the hook fails closed, so sign-ups stop rather than slipping through unverified.

Use `pnpm install --frozen-lockfile` then `pnpm nx run @bodysense/web:typecheck` and `(cd apps/api && go test ./internal/auth ./internal/middleware ...)`. No Supabase service-role key is needed for normal Go API requests.

**Production build:** `.github/workflows/candidate-publish.yml` injects the public project URL/publishable key into *both* the R2 static-assets build and the immutable Docker image build. Runtime-only environment changes to the web nginx container will NOT change the baked Vite `import.meta.env` constants. The ALTCHA secrets are API-runtime-only and are not part of the web build at all.

## Verification evidence (2026-11-02)

- Existing Chromium browser/profile E2E passed against fresh isolated PostgreSQL/Redis and a CI-only Supabase ES256/JWKS test double: signup, Go /me, local UUID provisioning, profile and BodyState business operations, session persistence; containers cleaned.
- Disposable API smoke passed against the same isolated stack: signup → Go /me (200) → hosted identity + local data erasure (completed) → original JWT rejected by tombstone (503) → deleted identity cannot sign in → missing bearer rejected (401).
- Full Go test suite, focused web auth/component tests (17), TypeScript typecheck and production Vite build passed. The Supabase SDK is lazy-loaded so the initial production bundle stays within the 500 KB repo limit.
- The isolated auth double is active only when `APP_ENV=test` and `BODYSENSE_E2E_STUB_AUTH=1`; the Go verifier additionally requires a loopback origin for this exception. All ordinary/staging/production Supabase URLs require HTTPS.

## Not yet verified

- Live Cloudflare Site Key and Supabase CAPTCHA secret were not made available at implementation time: full CAPTCHA flow is **blocked**, not passed.
- No backend secret key available: *hosted identity deletion* is not live-verified; missing key fails closed.
- No live Supabase test user was created yet, to avoid polluting an external hosted account store without a deletion credential.
- Browser → Go → Python consultation with a real hosted Supabase JWT must be re-run after the above configuration. Do not claim full end-to-end verified until that is complete.

## Security and privacy

- The old Go username/password and refresh endpoints `/api/v1/auth/{register,login,refresh,logout}` are retired; there must be no fallthrough to them.
- Staging/CI synthetic tests should never use the real hosted Supabase project or generate bulk permanent users. For a live test, use a disposable email and clean it through the Supabase dashboard/admin API afterwards.
- Do not point a development worktree at the actively running production database. The migration removes `users.password_hash` and intentionally does not support legacy login.
- Keep full privacy-erasure workflow: delete hosted identity, delete health data and uploads, then retain anonymous irreversible audit/tombstone. A user who deletes health data must not be able to re-provision their prior identity via a still-valid signed token.
