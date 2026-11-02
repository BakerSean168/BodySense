# BodySense: hosted Supabase Auth boundary

**Issue:** #491 (2026-11-02). BodySense has no live account data that needs migration. The existing PostgreSQL database remains the source of truth for health and clinical/business records; Supabase Auth alone handles password/login/session.

## Runtime boundaries

| Surface | Authority |
| --- | --- |
| React sign-up/sign-in/logout/session rotation | `@supabase/supabase-js` against hosted Auth |
| Human verification | Cloudflare Turnstile `captchaToken`; validated by hosted Supabase, not Go |
| Access JWT signature and issuer/audience/subject | Go verifies ES256 via project JWKS |
| User ID, health records, authorization and `operator` role | BodySense Go + PostgreSQL |
| Full-account erasure | Durable Go privacy workflow, which first deletes the hosted Supabase identity with a server-only secret |

**JWT trust:** only ES256, configured issuer `$SUPABASE_URL/auth/v1`, audience `authenticated`, signed `sub` UUID, nonempty email, nonanonymous and user `role=authenticated` are accepted. JWT public signing keys come from the hosted JWKS endpoint with bounded caching. Local roles are NOT inferred from JWT metadata. A valid new `sub` is upserted as `public.users.id`; the old local `password_hash` is removed by migration `000002`. Existing patient/consultation/AI contracts continue to use that UUID.

A JWT already issued by Supabase may remain verifiable after logout until its `exp`; unlike the old Redis family cache there is no blanket *instant* server-side revocation. The default Supabase JS browser session persists access and refresh tokens in localStorage; this differs from the former HttpOnly refresh-cookie boundary and makes strong CSP/XSS prevention and dependency hygiene important. Set **JWT expiry to 15 minutes or less** in the Supabase project before public usage. The immutable local privacy-erasure tombstone blocks the still-valid JWT after account deletion.

## Dashboard configuration required

1. **Supabase URL:** `https://qdezojzgajsunjwthifv.supabase.co`. Public publishable API key may be embedded in React; service-role or `sb_secret_` **must never** be compiled or committed into browser assets.
2. **Supabase Authentication > Sign In / Providers > Email:** email-password enabled. **Confirm email disabled for development only** (verified through `/auth/v1/settings`: `external.email=true`, `mailer_autoconfirm=true`). This permits registering an address the user does not own; it is not equivalent to verified email. Require verified email + SMTP before general public signups.
3. **Supabase Authentication > Sessions:** set JWT lifetime to <=15 minutes before production use.
4. **Cloudflare Turnstile:** create a Managed widget for each actual browser hostname (`localhost`, your public app domain). Record its public **Site Key** (never Turnstile Secret Key in React). Enable CAPTCHA under **Supabase Authentication > Attack Protection** using the matching Cloudflare **Secret Key**. Test that missing/invalid tokens are rejected by hosted Supabase and successful Turnstile sign-ups work. Both signUp and signInWithPassword pass `captchaToken`.
5. **GitHub release environment variable:** `BODYSENSE_TURNSTILE_SITE_KEY` (public). The candidate-publish workflow builds the public JS bundle with this value. Frontend site key **is not** dynamically loaded from the Go container.
6. **Go permanent account deletion:** set a separate **`SUPABASE_SECRET_KEY`** only in the protected API server environment (Supabase Project Settings > API Keys > Secret). The browser must not read it. If absent, `/api/v1/privacy/erasure` fails closed rather than falsely declaring an identity permanently deleted.
7. **Supabase URL Configuration:** add all approved browser origins and callback URLs as needed. Password recovery is not enabled until a real email sender is configured.

## Local development

Use an ignored local environment file (`.env.dev.local` in the BodySense repo or appropriate dev host) for public browser values:

```dotenv
SUPABASE_URL=https://qdezojzgajsunjwthifv.supabase.co
VITE_SUPABASE_URL=https://qdezojzgajsunjwthifv.supabase.co
VITE_SUPABASE_PUBLISHABLE_KEY=<paste your sb_publishable_ key>
VITE_TURNSTILE_SITE_KEY=<paste public Cloudflare Site Key, if already configured>
# Supabase server-side secret is optional for basic login, but REQUIRED to delete accounts:
SUPABASE_SECRET_KEY=
```

Use `pnpm install --frozen-lockfile` then `pnpm nx run @bodysense/web:typecheck` and `(cd apps/api && go test ./internal/auth ./internal/middleware ...)`. No Supabase service-role key is needed for normal Go API requests.

**Production build:** `.github/workflows/candidate-publish.yml` injects the public project URL/publishable key into *both* the R2 static-assets build and the immutable Docker image build. The Turnstile site key comes from a GitHub environment variable. Runtime-only environment changes to the web nginx container will NOT change the baked Vite `import.meta.env` constants.

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
