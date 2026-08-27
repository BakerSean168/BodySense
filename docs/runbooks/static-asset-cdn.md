# BodySense immutable static asset CDN runbook

> Applies to public, user-independent bytes only: Vite hashed assets and the pinned Vanatome atlas.

## 1. Security boundary

The public asset origin must never receive or encode:

- user IDs;
- conversation IDs;
- BodyState/fact/observation IDs;
- symptom text;
- diagnosis/treatment content;
- auth cookies or bearer tokens.

`index.html`, `/api/*`, auth and SSE remain on the application origin. The CDN contains only bytes that are identical for every user.

## 2. Cloudflare resources

Create one R2 bucket:

```text
bodysense-static
```

Connect a production custom domain:

```text
assets.bakersean.top
```

Do not use a moving `/latest` prefix.

Cloudflare supports R2 through its S3-compatible API endpoint:

```text
https://<ACCOUNT_ID>.r2.cloudflarestorage.com
```

Generate an R2 Object Read & Write credential scoped only to `bodysense-static`.

## 3. CORS

The bucket is public read-only through the custom domain. Upload/delete permissions exist only through the S3 API credential.

Cloudflare currently exposes **two different JSON shapes** for the same R2 CORS policy:

- Dashboard **Settings → CORS Policy → JSON** uses the AWS/S3-style array with `AllowedOrigins`, `AllowedMethods`, `ExposeHeaders`, and `MaxAgeSeconds`. Paste `scripts/static-assets/r2-cors-dashboard.json` there.
- Wrangler CLI uses Cloudflare's API wrapper shape with `rules[].allowed`. Keep using `scripts/static-assets/r2-cors.json` with `wrangler r2 bucket cors set`.

Dashboard JSON:

```json
[
  {
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET", "HEAD"],
    "ExposeHeaders": ["Content-Length", "Content-Range", "ETag"],
    "MaxAgeSeconds": 86400
  }
]
```

Wrangler CLI:

```bash
npx wrangler r2 bucket cors set bodysense-static \
  --file scripts/static-assets/r2-cors.json

npx wrangler r2 bucket cors list bodysense-static
```

The public files do not use credentials, so wildcard read CORS is intentional. `AllowedHeaders` is intentionally omitted because browsers only perform anonymous `GET`/`HEAD` reads for these assets and do not send application-specific request headers.

After changing CORS on an already cached custom domain, purge that hostname's cache before browser validation.

## 4. Resource Timing header

For useful browser performance diagnostics on the cross-origin CDN, add a Cloudflare Response Header Transform Rule for `assets.bakersean.top`:

```text
Timing-Allow-Origin: *
```

Without this header, browsers intentionally hide detailed cross-origin Resource Timing fields. BodySense diagnostics already marks those measurements as restricted instead of fabricating TTFB/transfer values.

## 5. GitHub production Environment

Add Environment variable:

```text
STATIC_ASSET_CDN_BASE=https://assets.bakersean.top
```

Add Environment secrets:

```text
R2_ENDPOINT=https://<ACCOUNT_ID>.r2.cloudflarestorage.com
R2_ACCESS_KEY_ID=<R2 S3 access key>
R2_SECRET_ACCESS_KEY=<R2 S3 secret>
R2_BUCKET=bodysense-static
```

Do not place the R2 credential in application `.env` files or Docker images.

Until `STATIC_ASSET_CDN_BASE` exists, the production workflow intentionally retains the current same-origin Web behavior.

## 6. Object layout

```text
bodysense-static/
├── web/
│   └── <40-char-git-revision>/
│       ├── manifest.json
│       └── assets/
│           ├── index-<hash>.js
│           ├── BodyExplorer3D-<hash>.js
│           ├── index-<hash>.css
│           └── ...
└── anatomy/
    └── vanatome/
        └── 1.4.0/
            ├── release-manifest.json
            ├── ATTRIBUTION.txt
            ├── models/
            └── releases/1.4.0/
```

Every object is immutable and receives:

```text
Cache-Control: public, max-age=31536000, immutable
x-amz-meta-sha256: <expected hash>
```

The publisher uses `HeadObject` to verify byte count + SHA-256 metadata after upload.

## 7. Staging publisher credential storage

Keep the staging publisher token outside both the repository and `.env.staging.local`:

```text
~/.config/bodysense/secrets/staging-static-assets.env
```

Recommended permissions:

```bash
umask 077
mkdir -p ~/.config/bodysense/secrets
chmod 700 ~/.config/bodysense/secrets
```

The file contains deployment-only values:

```text
STATIC_ASSET_CDN_BASE=https://assets.bakersean.top
R2_ENDPOINT=https://<ACCOUNT_ID>.r2.cloudflarestorage.com
R2_ACCESS_KEY_ID=<staging publisher access key>
R2_SECRET_ACCESS_KEY=<staging publisher secret key>
R2_BUCKET=bodysense-static
```

`scripts/staging-runtime.sh publish-static` loads this file automatically. It never copies these credentials into Docker build args or the generated `.runtime/staging-static-assets.env`; that generated file contains only the two public CDN URLs.

## 8. Local publication

Prepare a production-style Web build:

```bash
REVISION="$(git rev-parse HEAD)"
export STATIC_ASSET_CDN_BASE=https://assets.bakersean.top
export VITE_ASSET_BASE="${STATIC_ASSET_CDN_BASE}/web/${REVISION}/"
export VITE_BODYSENSE_ANATOMY_CATALOG_URL="${STATIC_ASSET_CDN_BASE}/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json"

pnpm --dir apps/web exec vite build
node scripts/static-assets/manifest.mjs \
  --dist apps/web/dist \
  --revision "$REVISION" \
  --base-url "$VITE_ASSET_BASE"
```

With R2 secrets exported:

```bash
node scripts/static-assets/publish-web-r2.mjs \
  --dist apps/web/dist \
  --public-base "$STATIC_ASSET_CDN_BASE"
```

For the atlas:

```bash
ATLAS_ROOT="$(mktemp -d)"
node scripts/anatomy/sync.mjs --version 1.4.0 --output "$ATLAS_ROOT"
node scripts/anatomy/verify.mjs --version 1.4.0 --root "$ATLAS_ROOT"
node scripts/static-assets/publish-atlas-r2.mjs \
  --version 1.4.0 \
  --root "$ATLAS_ROOT" \
  --public-base "$STATIC_ASSET_CDN_BASE"
```

Use `--dry-run` on either publisher to inspect the immutable object plan without credentials or writes.

## 9. Production release behavior

`.github/workflows/docker-deploy.yml` now enforces:

```text
verified main revision
  -> publish/verify atlas + revision-scoped Vite assets
  -> build immutable Web/API/AI/runtime images
  -> pull immutable Web image
  -> enumerate its /assets files
  -> HEAD every corresponding CDN object
  -> only then allow prod-latest promotion
```

This means an `index.html` cannot become production-eligible while its hashed CDN dependencies are absent.

## 10. Staging

Staging Docker builds accept `VITE_ASSET_BASE`.

Before the R2 credential/custom domain is provisioned, staging continues to use:

- private Tailnet for HTML/API/SSE/Vite chunks;
- Vanatome's pinned public `1.4.0` Cloudflare endpoint for atlas bytes.

After R2 is provisioned:

1. publish the current revision Web assets;
2. set `VITE_ASSET_BASE=https://assets.bakersean.top/web/<revision>/` for that staging build;
3. set the BodySense R2 atlas catalog URL;
4. rebuild only the staging Web service;
5. confirm DevTools shows `assets.bakersean.top` for hashed JS/CSS and atlas files while all `/api/` requests remain on the Tailnet application origin.

## 11. Rollback

Never delete an old `web/<revision>/` prefix as part of normal deployment.

Rolling the Web image back to revision A automatically restores references to:

```text
https://assets.bakersean.top/web/A/assets/...
```

No cache purge or mutable pointer is needed.
