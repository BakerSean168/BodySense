# Anatomy Asset Governance and Distribution

> Status: Implemented for pinned atlas acquisition/verification/staging CDN; production release validation pending
> Date: 2026-08-27
> Decision source: ADR 0006
> Scope: Vanatome/Z-Anatomy-derived atlas assets, versioning, provenance, self-hosting, and release controls

## 1. Purpose

BodySense consumes a third-party open anatomy atlas as static product infrastructure. The atlas is not user data, but it is a versioned product dependency with licensing and compatibility consequences.

This document defines how BodySense acquires, validates, pins, hosts, upgrades, attributes, and rolls back anatomy assets.

## 2. License boundary

The initial dependency has two separate surfaces:

### Software

- `@vixotic/vanatome-react`
- `@vixotic/vanatome-atlas`

Upstream states these software packages are MIT-licensed.

### Atlas material

The anatomy atlas is adapted from Z-Anatomy and remains subject to CC BY-SA 4.0 according to upstream Vanatome documentation.

Therefore BodySense must not treat the GLB/catalog/metadata as ordinary MIT application code.

## 3. Repository boundary

Implemented logical release structure (Git retains notices/manifests; binary atlas bytes may live in controlled static/CDN storage):

```text
THIRD_PARTY_NOTICES.md

docs/
└── architecture/
    └── anatomy-asset-governance.md

apps/web/public/
└── anatomy/
    └── vanatome/
        └── 1.4.0/
            ├── catalog.json
            ├── ... atlas metadata
            ├── ... model GLB assets
            ├── ATTRIBUTION.txt
            └── LICENSES/
                └── CC-BY-SA-4.0.txt
```

If binary atlas assets are not committed to Git because of repository size, preserve the same logical directory in static/CDN storage and keep attribution/provenance material in Git.

## 4. Immutable releases

Every production atlas is addressed by an immutable release path.

Example:

```text
/anatomy/vanatome/1.4.0/catalog.json
```

Rules:

- never overwrite `1.4.0` assets in place;
- any changed atlas material gets a new BodySense asset release/version;
- mapping files declare the exact atlas release they target;
- rollback changes both atlas config and mapping version together.

## 5. Upstream acquisition

For each imported release, record:

- upstream project URL;
- upstream atlas release/version;
- acquisition date;
- source catalog URL/path;
- source attribution notice;
- source license;
- hashes for acquired files where practical;
- whether BodySense modified the material.

The current pinned release is Vanatome atlas `1.4.0`; BodySense records upstream provenance and immutable file metadata in `scripts/anatomy/vanatome-1.4.0.manifest.json`.

## 6. Verification before promotion

An atlas candidate must pass:

1. catalog parse/loader validation;
2. required full-body profile availability;
3. referenced metadata and GLB availability;
4. BodySense mapping validation;
5. no unknown anatomy IDs in `vanatome-region-map`;
6. expected attribution/provenance present;
7. browser smoke load;
8. basic selection/focus/isolate test;
9. asset size/transfer measurement;
10. license notice review.

## 7. BodySense modifications

Prefer consuming the official immutable atlas without modifying anatomy material.

If modification becomes necessary, for example:

- deleting structures;
- changing materials embedded in GLB;
- recompressing/re-exporting models;
- changing structure metadata;
- producing a custom atlas build;

then the release record must include:

- what was modified;
- by which tool/process;
- source release;
- output release identity;
- applicable ShareAlike treatment;
- updated attribution notice.

Do not imply that an adapted atlas is an untouched upstream Vanatome release.

## 8. Mapping is BodySense data, not upstream atlas data

`BodyRegionOntology` and `vanatome-region-map` are BodySense-owned semantic artifacts.

They may reference upstream anatomy IDs, but they must declare:

```text
ontology_version
atlas_provider
atlas_release
mapping_schema_version
```

The mapping must be regenerated/revalidated for every atlas upgrade.

## 9. Runtime configuration

Atlas URL must come from environment/runtime config rather than hardcoded component source.

Implemented runtime variable:

```text
VITE_BODYSENSE_ANATOMY_CATALOG_URL
```

Environment intent:

```text
dev/spike    -> explicit upstream immutable URL allowed temporarily
staging      -> BodySense-controlled self-host candidate
production   -> BodySense-controlled immutable self-host URL only
```

Do not use an unversioned moving URL.

## 10. HTTP/cache behavior

Versioned atlas files are immutable static assets.

Recommended CDN/static cache:

```text
Cache-Control: public, max-age=31536000, immutable
```

Use content type appropriate to JSON/GLB files and correct CORS behavior if not same-origin.

Same-origin hosting is preferred.

## 11. Privacy boundary

Atlas requests must not contain:

- user ID;
- conversation ID;
- BodyState IDs;
- region health status;
- symptom text;
- diagnosis/treatment information.

The atlas host only receives requests for public static release paths.

## 12. Supply-chain boundary

Software package upgrades and atlas upgrades are separate reviews.

### Package upgrade review

Check:

- release notes/changelog;
- TypeScript/API compatibility;
- Three/R3F peer compatibility;
- bundle changes;
- security advisories;
- viewer behavior tests.

### Atlas upgrade review

Check:

- anatomy ID compatibility;
- mapping coverage;
- hierarchy changes;
- asset size;
- focus/geometry changes;
- license/provenance;
- visual regression.

Do not automatically pair `latest` software with `latest` atlas in production.

## 13. Rollback

A known-good release record must preserve:

```text
viewer package versions
atlas release
mapping version
BodyRegionOntology version
```

Rollback must restore a compatible set.

Example:

```text
viewer 0.1.x
atlas 1.4.0
mapping v1
ontology v1
```

The exact lockfile commit and application image remain the primary deployment rollback unit.

## 14. Attribution UX

Attribution should be accessible but not noisy.

Recommended UI:

```text
User menu / About BodySense / Anatomy sources
```

Content can state that the 3D anatomy atlas uses Vanatome and Z-Anatomy-derived material and link to the applicable license/source notices.

This UI is supplementary to repository/static-asset attribution requirements, not a substitute.

## 15. Production gate

3D anatomy is not production-accepted until:

- [ ] atlas release is explicit and immutable;
- [ ] BodySense mapping declares exact release;
- [ ] attribution/provenance is present;
- [ ] production does not require upstream static hosting;
- [ ] package and atlas versions are recorded;
- [ ] rollback set is documented;
- [ ] mapping CI passes;
- [ ] browser loading and fallback tests pass;
- [ ] no user health information is sent to atlas host.

## 16. Upstream references

- https://github.com/vixotic/Vanatome
- https://github.com/vixotic/Vanatome/blob/main/ASSET-LICENSE.md
- https://github.com/vixotic/Vanatome/blob/main/docs/atlas-contract.md
- https://github.com/vixotic/Vanatome/blob/main/docs/anatomy-pipeline.md
