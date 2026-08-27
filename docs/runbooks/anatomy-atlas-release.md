# Anatomy Atlas Release Runbook

> Scope: BODY3D-1113 — Vanatome atlas acquisition, verification, Web-image
> packaging, immutable same-origin serving, provenance, and rollback.
>
> Current approved atlas: **Vanatome Human Atlas 1.4.0**.

## 1. Selected distribution strategy

BodySense uses:

```text
checked-in SHA-256 manifest
        |
        v
Docker build stage fetches exact immutable upstream files
        |
        v
byte/sha256/catalog-reference verification
        |
        v
BodySense Web OCI image (ACR)
        |
        v
Nginx same-origin versioned static path
```

This is strategy **C: CI/build-time verified acquisition -> Web image**.

### Why not ordinary Git

The real 1.4.0 inventory is 26 upstream files and 96,981,412 bytes
(92.49 MiB). Nearly all of that is binary GLB data. Committing the binaries to
ordinary Git would permanently inflate repository history and clone/fetch cost.

### Why not Git LFS

Git LFS is technically viable, but it adds approximately 92.5 MiB of LFS
storage/download traffic for each clean consumer and introduces a second
artifact transport path. The repository did not previously require LFS for this
asset. BodySense already has a coherent OCI delivery plane, so LFS would add
cost/operational surface without improving runtime independence.

### Why not external object storage

Object storage/CDN would also work, but adds provisioning, credentials/domain
configuration, and another deployment unit. The atlas is small enough to fit
comfortably in the Web OCI artifact and changes infrequently. An external CDN
can be reconsidered only if BODY3D-1114 measurements show image/distribution
cost is material.

### Why the selected strategy fits

- no new paid service;
- no large binary Git commits;
- exact version pin;
- SHA-256 verification before the asset enters the image;
- release build fails closed on missing or changed upstream bytes;
- staging and production use the same Nginx/static behavior;
- runtime does not depend on `atlas.vanatome.vixotic.in`;
- the existing ACR/Web-image rollback unit also rolls back the atlas.

## 2. Verified 1.4.0 inventory

Inventory captured 2026-08-27 from:

`https://atlas.vanatome.vixotic.in/releases/1.4.0/catalog.json`

Catalog identity:

```text
atlas id:      vanatome-human
version:       1.4.0
build id:      994e6cc8ffbb212e
catalog bytes: 8,712
catalog sha256:d33c4722029c0c238a13a4a9418f7b7bc3a479366e8593c3e58d2f107b722397
catalog ETag:  "d711eeba7620dd1d65bbc05f3a842717"
```

Totals:

| Measure                    |      Value |
| -------------------------- | ---------: |
| Upstream files             |         26 |
| Total bytes                | 96,981,412 |
| Total MiB                  |      92.49 |
| GLB bytes                  | 96,402,980 |
| Metadata bytes             |    568,574 |
| GLB files                  |         12 |
| Metadata files             |         12 |
| Catalog files              |          1 |
| Upstream attribution files |          1 |

All 12 GLB downloads matched both the `bytes` and `sha256` values embedded in
the upstream catalog. Metadata/catalog/attribution files have BodySense-recorded
SHA-256 values in the manifest. Observed `ETag`, `Last-Modified`, Content-Type,
and Content-Length are also recorded when supplied by upstream, but **SHA-256
is the integrity authority**.

The complete per-file inventory lives in:

`scripts/anatomy/vanatome-1.4.0.manifest.json`

## 3. Immutable on-image layout

BodySense deliberately wraps the untouched upstream directory hierarchy in one
additional version directory:

```text
/static/anatomy/vanatome/1.4.0/
├── ATTRIBUTION.txt
├── BODYSENSE_PROVENANCE.txt
├── LICENSES/
│   └── CC-BY-SA-4.0.txt
├── models/
│   ├── z-anatomy-1.4.0-full-body.glb
│   └── ... 11 other GLB bundles
└── releases/
    └── 1.4.0/
        ├── catalog.json
        └── ... 12 metadata JSON files
```

This preserves the upstream catalog byte-for-byte. For example the upstream
catalog's `../../models/...` still resolves correctly inside the outer BodySense
`1.4.0` directory, while `../../ATTRIBUTION.txt` resolves to the version-scoped
upstream attribution file.

Viewer catalog URL:

`/static/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json`

No `/latest/` alias exists.

## 4. Acquisition and verification commands

Fetch a clean release into a temporary or local public directory:

```bash
node scripts/anatomy/sync.mjs \
  --version 1.4.0 \
  --output /tmp/bodysense-atlas/1.4.0
```

The command removes the target release directory first, downloads only the URLs
listed in the checked-in manifest, and fails if:

- an HTTP request fails after bounded retry;
- Content-Length disagrees when upstream provides it;
- byte count differs;
- SHA-256 differs;
- catalog identity/build ID differs;
- a catalog model/metadata/attribution reference is not pinned by the manifest;
- a model's catalog-declared bytes/hash disagree with the manifest;
- required redistribution notices are absent.

Verify an already acquired release without network access:

```bash
node scripts/anatomy/verify.mjs \
  --version 1.4.0 \
  --root /tmp/bodysense-atlas/1.4.0
```

There is no fallback to `latest` and no hash learning during a production build.
A new upstream release requires a separately reviewed new manifest.

## 5. Docker/release integration

`docker/Dockerfile.web` has an isolated `anatomy-atlas` stage. It fetches and
verifies the approved version before the Vite build, then copies the verified
release into `apps/web/public/static/anatomy/vanatome/`. Vite copies that static
content into `dist`, and the final Nginx image contains it.

The separate stage is intentional: Docker/Buildx can cache the ~92 MiB atlas
layer across ordinary application code changes as long as the atlas scripts,
manifest, or selected version do not change.

Production release workflow pin:

```text
BODYSENSE_ANATOMY_ATLAS_VERSION=1.4.0
```

Staging Compose exposes the same value as a build arg and defaults to 1.4.0.
Only a version with a checked-in `vanatome-<version>.manifest.json` can build.

## 6. Viewer configuration seam

The Web build exports:

```text
VITE_BODYSENSE_ANATOMY_CATALOG_URL=/static/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json
```

The Viewer lane should read this environment seam rather than hard-code an
upstream host. The catalog URL contains no user ID, BodyState ID, symptom text,
health status, diagnosis/treatment information, conversation ID, or query
string.

For local direct development, `.env.example` documents the same seam. A local
self-host test can sync into:

```bash
node scripts/anatomy/sync.mjs \
  --version 1.4.0 \
  --output apps/web/public/static/anatomy/vanatome/1.4.0
```

The generated binary directory is ignored by both Git and Docker build context;
the Docker build always performs its own verified acquisition.

## 7. Nginx behavior

`docker/nginx.conf` owns a dedicated `^~ /static/anatomy/vanatome/` location.
It provides:

```text
Cache-Control: public, max-age=31536000, immutable
Content-Type: application/json          # .json
Content-Type: model/gltf-binary         # .glb
Content-Type: text/plain                # .txt
X-Content-Type-Options: nosniff
```

`try_files $uri =404` is mandatory. A missing atlas file must return an actual
404 and must never fall through to the SPA `index.html` response.

Same-origin delivery means no CORS dependency is needed for production atlas
loading.

## 8. License and provenance

Repository notice:

`THIRD_PARTY_NOTICES.md`

Packaged release notices:

```text
ATTRIBUTION.txt                 # untouched upstream notice
BODYSENSE_PROVENANCE.txt        # BodySense redistribution/modification statement
LICENSES/CC-BY-SA-4.0.txt       # CC BY-SA 4.0 legal code
```

License boundary:

```text
Vanatome viewer/software -> MIT
Vanatome/Z-Anatomy-derived atlas -> CC BY-SA 4.0
```

BodySense modification statement for 1.4.0: **no upstream atlas bytes are
modified**. BodySense only mirrors the verified files under its own immutable
version-scoped static prefix and adds notices/license material.

## 9. Upgrade and rollback

### Upgrade 1.4.0 -> future 1.x

1. inventory the exact new version;
2. create `scripts/anatomy/vanatome-<version>.manifest.json` with SHA-256 for all
   referenced assets;
3. verify catalog references and provenance;
4. review mapping compatibility separately;
5. change the explicit atlas build pin;
6. build and validate staging;
7. promote the coherent application release only after integration acceptance.

Never introduce `/latest/`.

### Rollback future 1.x -> 1.4.0

Preferred production rollback is the previous coherent Web/application OCI
release, which already contains atlas 1.4.0 and the compatible mapping/viewer
set. A rebuild can also explicitly set the atlas build version back to `1.4.0`.

Do not point a mapping validated against atlas N at atlas M. Atlas and mapping
pins move together at the integration/release boundary.

## 10. Validation checklist

After a Web image is built and started locally or in staging:

```bash
BASE=http://127.0.0.1:<port>
CATALOG=/static/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json
GLB=/static/anatomy/vanatome/1.4.0/models/z-anatomy-1.4.0-full-body.glb

curl -fsSI "$BASE$CATALOG"
curl -fsSI "$BASE$GLB"
curl -fsS "$BASE$CATALOG" | jq '.atlas'
curl -fsS -o /dev/null -w '%{http_code}\n' \
  "$BASE/static/anatomy/vanatome/1.4.0/not-found.glb"
```

Required observations:

- catalog: HTTP 200, `application/json`;
- GLB: HTTP 200, `model/gltf-binary`;
- both: `Cache-Control: public, max-age=31536000, immutable`;
- missing asset: HTTP 404, not 200/index.html;
- catalog atlas version: exactly `1.4.0`;
- no atlas request URL contains user/health data.

## 11. BODY3D-1114 boundary

This lane records distribution inputs only:

- upstream asset bytes/file count;
- image acquisition/build behavior;
- static HTTP behavior.

It does **not** declare BODY3D-1114 complete. Viewer lazy-chunk size, browser
first-usable-body timing, GPU/CPU/memory behavior, WebGL recovery, and visual
acceptance require the integrated Viewer and remain part of BODY3D-1114.
