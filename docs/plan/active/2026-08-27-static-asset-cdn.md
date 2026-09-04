# BodySense Immutable Static Asset CDN

> Date: 2026-08-27
> Status: STAGING + CLOUDFLARE EDGE COMPLETE / PRODUCTION RELEASE VALIDATION PENDING
> Scope: Vite hashed assets + Vanatome atlas distribution
> Privacy boundary: no user-, conversation-, diagnosis-, or BodyState-specific bytes may enter the public asset origin

## 1. Problem

The private staging application is intentionally exposed through Tailscale Serve. That path is appropriate for authenticated HTML/API/SSE traffic, but real browser diagnostics showed that it is a poor transport for large immutable assets on a lossy client-to-GCP route:

- Windows ↔ GCP Tailnet: ~265 ms RTT with observed packet loss;
- a 3.8 MiB gzip anatomy GLB stalled for >20 seconds;
- the ~283 KiB gzip `BodyExplorer3D` lazy JS chunk required a retry roughly one minute later;
- the same anatomy release served through Vanatome's Cloudflare distribution completed normally.

The application and static byte-delivery planes therefore need different network boundaries.

## 2. Target architecture

```text
Browser
  |
  +-- Private app origin (Tailscale staging / production app origin)
  |     index.html
  |     /api/*
  |     auth
  |     SSE / consultation runtime
  |     health data
  |
  +-- Public immutable asset origin (Cloudflare R2 custom domain)
        /web/<git-revision>/assets/*
        /web/<git-revision>/manifest.json
        /anatomy/vanatome/1.4.0/*
```

Invariant:

> Bytes that vary by user remain on the private/application origin. Bytes that are identical for every user may be published to the public immutable asset origin.

## 3. Release identity

Vite assets use the full Git revision as an immutable namespace:

```text
https://assets.bakersean.top/web/<revision>/assets/...
```

The atlas remains independently versioned:

```text
https://assets.bakersean.top/anatomy/vanatome/1.4.0/...
```

Never publish application assets to `latest/`. Rollback is performed by serving an older Web image whose `index.html` points at its own immutable revision namespace.

## 4. Configuration

Build-time public values:

```text
VITE_ASSET_BASE=https://assets.bakersean.top/web/<revision>/
VITE_BODYSENSE_ANATOMY_CATALOG_URL=https://assets.bakersean.top/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json
```

R2 publication credentials stay only in deployment/CI secrets:

```text
R2_ENDPOINT
R2_ACCESS_KEY_ID
R2_SECRET_ACCESS_KEY
R2_BUCKET
```

The CDN base is non-secret and should be a GitHub Environment variable:

```text
STATIC_ASSET_CDN_BASE=https://assets.bakersean.top
```

## 5. Publication order

`assets first, HTML second` is a hard release invariant:

1. resolve exact Git revision;
2. build Web using the final CDN base;
3. generate a SHA-256/size static manifest;
4. upload `/assets/*` into `web/<revision>/assets/`;
5. upload and verify the manifest;
6. verify public CDN URLs return expected files;
7. only then build/publish the Web image whose `index.html` references those CDN URLs;
8. existing coherent image promotion continues afterward.

A release must never expose an `index.html` that references static assets not yet present at the public origin.

## 6. Atlas publication

Use the existing pinned Vanatome acquisition and verification scripts. Publish the byte-verified `1.4.0` tree under the versioned R2 prefix. Preserve all attribution/provenance notices. Atlas URLs must never contain health/user identifiers.

## 7. Cache/CORS

Immutable objects:

```text
Cache-Control: public, max-age=31536000, immutable
```

Public asset CORS allows anonymous `GET`/`HEAD`. No cookies or authorization headers are used.

`index.html` remains private/application-origin content and must continue to use `max-age=0, must-revalidate`.

## 8. Failure/rollback

If CDN configuration is absent, local/dev builds retain `base=/` and current same-origin behavior. Production CDN publication becomes mandatory only after R2 Environment variables/secrets are provisioned and the release gate is enabled.

Rollback never overwrites or deletes an old revision prefix.

## 9. Implementation tickets

### STATIC-1201 — Vite asset base

- add normalized `VITE_ASSET_BASE` support;
- leave dev/default behavior `/`;
- verify dynamic imports and CSS/font URLs use the CDN base in a production build.

### STATIC-1202 — Static manifest

- generate deterministic manifest from `dist/assets`;
- include revision, logical prefix, file size and SHA-256;
- validate no files escape the immutable prefix.

### STATIC-1203 — R2 publisher

- S3-compatible upload script;
- immutable cache headers;
- idempotent revision prefix;
- public verification through custom-domain base;
- no destructive `--delete` against historical revisions.

### STATIC-1204 — Atlas publisher

- reuse pinned atlas sync/verify;
- publish `1.4.0` to versioned public prefix;
- publish notices/provenance;
- verify catalog + representative GLB.

### STATIC-1205 — Production release gate

- add static publication before Web image publication;
- pass exact revision-specific `VITE_ASSET_BASE` into Web Docker build;
- keep API/AI/runtime build semantics unchanged;
- fail Web publication if configured CDN asset verification fails.

### STATIC-1206 — Staging integration

- make staging Docker build accept `VITE_ASSET_BASE`;
- when R2 credentials/domain exist, publish the current revision before rebuilding staging Web;
- until then, keep the current official immutable Vanatome CDN workaround for atlas only.

### STATIC-1207 — Runbook and validation

- document R2 bucket/custom-domain/CORS setup;
- document GitHub Environment names;
- verify private API requests remain on the application origin;
- verify Vite chunks/atlas use public asset origin;
- measure cold and warm BodyExplorer timing.

## 10. Acceptance

- Vite hashed JS/CSS/font chunks can be served from an absolute CDN base;
- `BodyExplorer3D` dynamic import resolves to CDN, not Tailnet, when enabled;
- Atlas is version-pinned and can be served from BodySense-controlled R2;
- public asset URLs contain no health/user context;
- CDN publication happens before the Web artifact that references it;
- old revision prefixes remain deployable for rollback;
- default local development remains functional with no R2 credentials.

## 11. Implementation checkpoint — 2026-08-27

Repository implementation, Cloudflare R2 provisioning, staging cutover, edge caching, and browser Resource Timing exposure are complete. The remaining acceptance item is the first real production release through the fail-closed publication gate.

| Ticket      | Repository status                                                               | External status                                                   |
| ----------- | ------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| STATIC-1201 | COMPLETE                                                                        | none                                                              |
| STATIC-1202 | COMPLETE                                                                        | none                                                              |
| STATIC-1203 | COMPLETE — AWS SDK S3 PUT/HEAD/idempotency integration-tested                   | COMPLETE — real R2 writes + HEAD/SHA verification passed          |
| STATIC-1204 | COMPLETE — 26 files / 96,981,412 bytes sync+SHA verify+publish passed           | COMPLETE — 26/26 immutable atlas objects live on R2               |
| STATIC-1205 | COMPLETE — assets-before-image + post-image CDN coherence gate                  | READY — production Environment variable + four R2 secrets present |
| STATIC-1206 | COMPLETE — staging publish/verify/disable commands and public build env overlay | COMPLETE — staging uses revision-scoped R2 CDN assets             |
| STATIC-1207 | COMPLETE for runbook/repository validation                                      | COMPLETE — cache + TAO + browser Resource Timing validated        |

Validated repository behavior:

```text
static tooling tests       5 passed
Web tests                  40 files / 197 tests passed
Web typecheck/lint/build   passed
supply-chain audit         high=0 critical=0
GitHub workflow            YAML parse + actionlint clean
Docker CDN build           passed
Vite ↔ Docker asset bytes  reproducible (filename + SHA-256 match)
Atlas verify               26 files / 96,981,412 bytes PASS
```

A production-style Docker image built with an absolute CDN base emitted CDN entry/modulepreload/CSS URLs while its application JavaScript retained relative `/api/v1/...` requests. This verifies the privacy/network split before external R2 activation.

The production Environment now has `STATIC_ASSET_CDN_BASE=https://assets.bakersean.top` plus the four R2 publication secrets. The next production release therefore activates the fail-closed assets-first pipeline; runtime hosts still receive no R2 credential.

## 12. Real R2 cutover checkpoint — 2026-08-27

Cloudflare resources and staging publication are now real, not dry-run:

```text
R2 bucket              bodysense-static
custom domain          https://assets.bakersean.top
CORS                    anonymous GET/HEAD validated
Atlas 1.4.0             26/26 objects uploaded and write-after-HEAD verified
Atlas bytes             96,981,412
Web revision            134d8b470ecf07ff6effed258e3d151e388dfc26
Web static files        21/21 uploaded and write-after-HEAD verified
staging index.html      private Tailnet origin
staging /api + SSE      private application origin
Vite JS/CSS/fonts       assets.bakersean.top/web/<revision>/...
Vanatome atlas          assets.bakersean.top/anatomy/vanatome/1.4.0/...
Body3D CDN-aware E2E    PASS
```

The staging coherence verifier confirmed every `/assets` file present in the running Web image exists at its exact revision CDN prefix. Production GitHub Environment now contains the four R2 secrets plus the public `STATIC_ASSET_CDN_BASE=https://assets.bakersean.top` variable; no R2 credential is present on the production runtime.

Cloudflare edge closeout is complete:

- hostname-scoped Cache Rule (`assets.bakersean.top` → Eligible for cache) is active;
- Vite `.js` GET validation: first `MISS`, second `HIT`;
- anatomy `.glb` GET validation: first `MISS`, second `HIT`;
- `Timing-Allow-Origin: *` is present through an HTTP **Response** Header Transform Rule;
- a cross-origin Chromium probe can read `nextHopProtocol=h2`, transfer/encoded/decoded byte counts, and TTFB instead of receiving TAO-restricted Resource Timing.

One uncached TAO probe of the 6,288,144-byte regional GLB reported `transferSize=6,288,444`, `encodedBodySize=decodedBodySize=6,288,144`, `nextHopProtocol=h2`, and measurable TTFB. Production's first real release remains the final release-pipeline acceptance.


## 13. Production CSP / proxy incident closeout — 2026-09-04

The first `v0.12.2` production cutover exposed a release-boundary defect rather than a CDN byte-availability defect: `index.html` correctly referenced revision-scoped JS/CSS on `https://assets.bakersean.top`, but the application-origin CSP still had `script-src 'self'` and did not allow the CDN for stylesheet/static fetch surfaces. Browsers therefore blocked the entry chunks and rendered a black screen even though the CDN objects returned HTTP 200.

A second runtime defect made a simple Caddy reload insufficient. `docker/Caddyfile` is bind-mounted as a single file. Runtime synchronization replaces that host file with the immutable runtime artifact, which can change its inode; a long-lived Caddy container can therefore keep seeing the previously mounted inode. Production was recovered by updating the CSP and force-recreating only the Caddy container, after which a real Chromium probe loaded the page with no CSP errors or failed CDN requests.

Durable contract:

- CSP allowlists only the BodySense-controlled `https://assets.bakersean.top` origin for script/style/font/image/media/fetch/worker static surfaces; no wildcard `https:`/`*` relaxation is accepted.
- every forward deploy and rollback force-recreates Caddy after runtime-file replacement, then reloads/health-checks the current config;
- `scripts/validate-production-proxy.sh` fails repository validation if the CDN origin disappears from CSP or either deploy path stops force-recreating Caddy.
