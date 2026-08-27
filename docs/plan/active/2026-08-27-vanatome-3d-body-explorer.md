# Active Plan: Vanatome 3D Body Explorer

Date: 2026-08-27
Status: IMPLEMENTED / staging validated / final anatomy-boundary visual audit pending
Owner scope: BodySense Web + additive BodyState region contract + anatomy static assets
Decision: ADR 0006
Architecture: `../../architecture/body-explorer-3d-anatomy.md`
Feature spec: `../../feature_spec_3d_body_explorer.md`
Ontology: `../../architecture/body-region-ontology.md`
Asset governance: `../../architecture/anatomy-asset-governance.md`

## 1. Goal

Replace the current coarse SVG body map as the primary State visualization with a complete Vanatome-backed 3D Body Explorer that supports:

- full-body 3D interaction;
- canonical BodySense region selection;
- fine anatomy drill-down;
- supported system/layer switching;
- focus / isolate / reset;
- BodyState ↔ 3D bidirectional linking;
- selected region/structure → single AI conversation context;
- accessibility and 2D fallback;
- version-pinned/self-host-ready atlas distribution;
- license/provenance compliance.

This is one complete implementation. Region mode is simple by default; anatomy complexity is progressively disclosed.

## 2. Preconditions

### 2.1 Current worktree safety — RESOLVED 2026-08-27

The dirty Workbench/staging/provider batch has been checkpointed into separate commits and the canonical worktree is clean.

Checkpoint commits:

```text
88173c4c  fix(ops): route staging structured inference through Groq
e3545ed5  feat(web): converge to the single BodySense workbench
9218996b  docs: adopt Vanatome 3D body explorer architecture
```

The 3D implementation integration branch is `feature/vanatome-body-explorer`. Parallel workers must start from the exact shared base SHA supplied in their worker instructions and must not modify the canonical `/home/dev/projects/bodysense` worktree.

Prepared worker worktrees:

```text
body3d/viewer           /home/dev/projects/bodysense-body3d-viewer
body3d/semantics        /home/dev/projects/bodysense-body3d-semantics
body3d/durable-contract /home/dev/projects/bodysense-body3d-durable-contract
body3d/distribution     /home/dev/projects/bodysense-body3d-distribution
```

All four workers share one exact base and own non-overlapping paths/tickets. Integration is performed only after each lane has committed and reported its result.

### 2.2 Architecture read set

Implementation worker must read:

- ADR 0002 — runtime ownership;
- ADR 0004 — Longitudinal BodyState;
- ADR 0006 — Vanatome adoption;
- `architecture/current-longitudinal-system.md`;
- `architecture/web-desktop-workbench-ui-ux.md`;
- `architecture/body-explorer-3d-anatomy.md`;
- `architecture/body-region-ontology.md`;
- `architecture/anatomy-asset-governance.md`;
- `feature_spec_3d_body_explorer.md`;
- current `BodyOverview`, `WorkspaceViewport`, `BodyStateWorkbench`, consultation runtime and contracts.

## 3. Upstream pins for the first implementation

Research snapshot on 2026-08-27:

```text
@vixotic/vanatome-react  0.1.6
@vixotic/vanatome-atlas  0.1.4
atlas release            1.4.0
```

Initial upstream atlas URL observed in the Vanatome README:

```text
https://atlas.vanatome.vixotic.in/releases/1.4.0/catalog.json
```

Rules:

- install exact versions, not floating `latest`;
- lockfile is authoritative after install;
- atlas release version must be represented in code/config;
- no unversioned production URL;
- production cutover requires self-host-ready assets and attribution.

## 4. Work breakdown

The plan is organized by dependency order. Tickets may run in parallel only where ownership boundaries are explicit.

---

## BODY3D-1101 — Integration baseline and dependency spike

### Objective

Prove Vanatome can render inside the existing React 19 / Vite / Tailwind / Workbench stack without changing BodyState semantics.

### Tasks

- add exact dependencies:
  - `@vixotic/vanatome-react`;
  - `@vixotic/vanatome-atlas`;
  - `three`;
  - `@react-three/fiber`;
  - `@react-three/drei`;
- create a feature-owned lazy-loaded integration component;
- load pinned atlas `1.4.0` full-body profile;
- render viewer in a development-only path/component or directly behind a local feature seam;
- verify:
  - model loads;
  - rotate/pan/zoom;
  - select;
  - focus;
  - isolate;
  - reset;
  - display/layer controls available through package APIs;
  - loading/error/WebGL callbacks;
- record actual package TypeScript API signatures rather than relying on planning pseudocode;
- record bundle/asset measurements.

### Deliverables

- `features/body-explorer` scaffold;
- working real Vanatome viewer spike;
- integration notes added to this plan;
- exact resolved versions in lockfile.

### Acceptance

- existing Web typecheck/lint/tests/build stay green;
- viewer renders in Chromium in staging/dev;
- no BodyState mutation path is added;
- Three/Vanatome are lazy/code-split.

---

## BODY3D-1102 — Vanatome adapter boundary

### Objective

Prevent third-party viewer APIs from leaking into product/domain components.

### Tasks

- define `AnatomyViewerPort` using actual supported capability surface;
- implement `VanatomeAdapter`;
- normalize:
  - selected ID;
  - hovered ID if supported;
  - isolation;
  - focus;
  - reset;
  - system/layer visibility;
  - display mode;
  - load/error/WebGL state;
- define branded `AnatomyStructureId` type;
- add adapter contract tests with mocked controller/viewer boundaries.

### Acceptance

- `WorkspaceViewport`, BodyState components and Chat do not import Vanatome directly;
- package-specific fields remain inside `features/body-explorer/adapters` and narrow viewer components.

---

## BODY3D-1103 — Atlas registry inventory

### Objective

Create a verified local representation of the exact pinned atlas structure registry for mapping work.

### Tasks

- load atlas release 1.4.0 metadata/catalog;
- generate an inventory artifact containing available:
  - anatomy IDs;
  - names;
  - system;
  - layer;
  - parent ID;
  - focus position;
  - geometry mapping evidence where available;
- do not copy unnecessary binary assets into Git during this ticket;
- add a validation command/test that can compare mapping IDs with the pinned registry.

### Suggested artifact

```text
apps/web/src/features/body-explorer/data/vanatome-1.4.0-registry.generated.json
```

If the registry is too large/noisy for source control, generate it during a validation script from a checked-in normalized subset and immutable catalog fixture. Document the chosen boundary.

### Acceptance

- no anatomy ID in later mappings can be invented;
- CI can detect unknown mapping IDs.

---

## BODY3D-1104 — BodyRegionOntology v1

### Objective

Implement the canonical product-owned body region vocabulary from `body-region-ontology.md`.

### Tasks

- create `body-regions.v1.json` or typed equivalent;
- include:
  - stable ID;
  - zh-CN label;
  - English label;
  - parent/group;
  - side;
  - aliases;
- add parsers/validators;
- add deterministic alias normalization;
- preserve unresolved values rather than fuzzy-guessing;
- add fixtures for:
  - left/right shoulder;
  - left/right knee;
  - neck;
  - upper/lower back;
  - pelvis;
  - hip/gluteal;
  - ambiguous `肩颈`, `肩膀`, etc.

### Acceptance

- canonical IDs are independent of Vanatome;
- bilateral regions cannot lose laterality;
- ambiguous free text does not silently become a durable ID.

---

## BODY3D-1105 — Curated Vanatome ↔ BodyRegion mapping

### Objective

Connect the BodySense ontology to verified atlas structures.

### Tasks

- build `vanatome-region-map.v1.json` for atlas release 1.4.0;
- for each BodyRegionId define:
  - preferred focus anatomy ID;
  - anatomy IDs associated with the region;
- generate reverse mapping;
- resolve reverse ownership deterministically;
- use atlas hierarchy/laterality rather than string heuristics at runtime;
- document structures that intentionally remain anatomy-only;
- review region boundaries manually with the real 3D model.

### Acceptance

- every mapped anatomy ID exists in the pinned registry;
- every canonical region has either a mapping or an explicitly documented reason for no mapping;
- reverse mapping has no unreviewed ambiguity;
- mapping declares ontology version and atlas release.

---

## BODY3D-1106 — Body Explorer presentation store and selectors

### Objective

Own 3D presentation state without creating a new health truth store.

### Tasks

- create `BodyExplorerStore`;
- state includes:
  - mode `region | anatomy`;
  - selected/hovered region;
  - selected/hovered anatomy structure;
  - isolated structure;
  - visible systems;
  - display mode;
  - camera/reset request state as needed;
- create selectors deriving region visual state from BodyState;
- implement precedence:
  - safety;
  - worsening;
  - improving;
  - stable;
  - observed;
  - none;
- keep hover non-persistent;
- do not persist health facts locally.

### Acceptance

- no server truth is duplicated into persisted Zustand state;
- region coloration is entirely derived from durable projections.

---

## BODY3D-1107 — Replace primary SVG with BodyExplorer3D

### Objective

Make Vanatome the primary body surface in `状态`.

### Tasks

- replace current `BodyOverview` primary rendering with lazy `BodyExplorer3D`;
- keep current SVG as `BodyExplorerFallback2D`;
- canvas sits directly on workbench surface;
- preserve current low-density UI language;
- implement:
  - full-body framing;
  - click/tap selection;
  - hover label where available;
  - persistent selected state;
  - reset;
  - region status marking;
- make 3D height/width responsive to current workbench split;
- prevent body from stretching/centering incorrectly as the old SVG did.

### Acceptance

- State remains usable with chat expanded/collapsed;
- body is visually balanced at common desktop sizes;
- no new nested dashboard card chrome;
- 2D fallback works.

---

## BODY3D-1108 — Region inspector and BodyState bidirectional linking

### Objective

Make 3D selection and durable BodyState feel like one object.

### Tasks

- selecting mapped anatomy resolves BodyRegionId;
- select/focus region when a BodyState row is activated;
- filter or group BodyState records by selected region;
- preserve unfiltered overview when no region selected;
- add `返回全身` / reset behavior;
- avoid duplicating BodyState components where existing workbench UI can be refactored.

### Acceptance

- 3D → record list works;
- record list → 3D works;
- no click creates or edits health truth.

---

## BODY3D-1109 — Anatomy drill-down

### Objective

Ship the full advanced anatomy capability in the same implementation while hiding it until requested.

### Tasks

- add `深入查看` from selected region;
- enter `anatomy` mode without leaving State tab;
- expose compact relevant systems, initially prioritize:
  - regional anatomy;
  - muscular;
  - skeletal;
  - nervous;
- remaining atlas systems live under `更多` or contextual control;
- support:
  - structure selection;
  - focus;
  - isolate;
  - normal/x-ray/ghost where useful;
  - hierarchy/breadcrumb;
  - reset/back to region mode;
- anatomy-only structures may remain selectable without forcing a BodyRegion mapping.

### Acceptance

- all advanced capabilities work in the same State workspace;
- default user is not confronted by all anatomy systems;
- selecting an anatomy structure is never described as a diagnosis.

---

## BODY3D-1110 — Additive durable `body_region_id` contract

### Objective

Move BodyState from free-text-only region identity toward canonical region keys without breaking current data.

### Tasks

- inspect Go contracts/domain/schema ownership before modifying;
- add optional canonical `body_region_id` to relevant durable BodyState structures;
- keep existing `body_region` display/raw text;
- update API contracts/codegen as needed;
- new explicit 3D selection writes canonical ID;
- legacy records:
  - deterministic aliases may map;
  - ambiguous values remain null;
- correction/temporal semantics from ADR 0004 remain unchanged;
- migration must be additive and reversible during rollout.

### Acceptance

- existing records still load;
- canonical ID round-trips through Go durable state and Web projection;
- no migration guesses ambiguous left/right;
- all contract tests green.

---

## BODY3D-1111 — Chat context bridge

### Objective

Allow the single AI conversation to understand the user's selected spatial context.

### Tasks

- implement removable composer context chip;
- region context:
  - BodyRegionId;
  - display label;
  - related durable IDs only where appropriate;
- anatomy context additionally includes selected anatomy ID/name;
- ensure authoritative BodyState is still loaded server-side rather than trusting client context alone;
- selecting a structure never sends private health data to Vanatome/upstream static host.

### Acceptance

- `询问 BodySense` focuses composer and displays context chip;
- user can remove context;
- no raw internal JSON is shown;
- runtime receives enough structured context to ground the question without replacing server authority.

---

## BODY3D-1112 — Accessibility and fallback completion

### Objective

Ensure the product is usable without precise pointer interaction or WebGL.

### Tasks

- keyboard-accessible body region list/tree;
- accessible labels/counts/status;
- canvas selection has adjacent semantic equivalent;
- force/test WebGL failure path;
- handle atlas load error and retry;
- handle WebGL context loss via upstream lifecycle and fallback;
- preserve selected BodyRegionId when falling back;
- reduced-motion behavior for camera transitions where feasible.

### Acceptance

- essential State functions remain available without canvas;
- no essential hover-only action;
- screen-reader user can select regions from semantic controls.

---

## BODY3D-1113 — Atlas self-hosting and license provenance

### Objective

Remove production dependence on upstream static hosting while respecting licenses.

### Tasks

- obtain exact pinned release assets/catalog/provenance;
- verify hashes/contents against expected upstream release;
- create immutable self-host path;
- preserve attribution and source notices;
- add repository documentation, for example:

```text
THIRD_PARTY_NOTICES.md or docs/... attribution section
public/static/anatomy/vanatome/1.4.0/ATTRIBUTION.txt
```

- record:
  - Vanatome software MIT boundary;
  - Z-Anatomy-derived atlas CC BY-SA 4.0 boundary;
  - modification statement if BodySense modifies atlas material;
- configure same-origin serving/CDN headers;
- switch catalog URL by environment/config, not source edits.

### Acceptance

- staging can load self-hosted immutable atlas;
- production config points to BodySense-controlled versioned path;
- required attribution/provenance is present;
- user health data never appears in atlas requests.

---

## BODY3D-1114 — Performance, browser, and visual acceptance

### Objective

Measure the real implementation and prevent a visually impressive but operationally expensive regression.

### Required measurements

Record:

- lazy JS chunk gzip size;
- atlas bytes for full-body and selected system flows;
- cold-cache time to first usable body;
- warm-cache time to first usable body;
- selection/focus response;
- idle CPU/GPU behavior;
- memory after repeated State ↔ Analysis transitions;
- chat collapse/expand with viewer mounted;
- WebGL context loss/recovery/fallback.

### Browser matrix

At minimum:

- Chrome/Chromium desktop;
- Edge desktop;
- Safari/WebKit if supported by current project testing environment;
- mobile/touch smoke path.

### Visual acceptance views

Capture stable screenshots for:

- full body front;
- full body back;
- right shoulder selected;
- lower back selected;
- anatomy muscular drill-down;
- anatomy skeletal drill-down;
- WebGL/atlas fallback.

### Acceptance

- no unrelated route must synchronously load Three/Vanatome;
- workbench remains responsive during loading;
- no continuous decorative motion;
- no severe memory accumulation after repeated tab switching;
- visual design remains consistent with current graphite Workbench.

---

## BODY3D-1115 — Old SVG retirement boundary

### Objective

Finish cutover without deleting the reliability fallback prematurely.

### Tasks

- remove old SVG from primary normal path;
- retain minimal fallback component;
- remove old body-zone coordinate logic if no longer needed by fallback;
- update tests/docs that still treat the SVG as primary;
- do not delete fallback until WebGL/accessibility requirements are met by another equivalent path.

### Acceptance

- normal supported browser always gets 3D viewer;
- fallback remains deterministic and tested.

## 5. Suggested parallelization

After BODY3D-1101 proves integration:

```text
Lane A — Viewer
1102 -> 1107 -> 1109 -> 1112

Lane B — Semantics
1103 -> 1104 -> 1105 -> 1106

Lane C — Durable contract
1110

Lane D — Distribution
1113
```

Integration gates:

```text
1105 + 1106 + 1107
        -> 1108

1108 + 1110
        -> 1111

1107 + 1109 + 1112 + 1113
        -> 1114
        -> 1115
```

Do not parallelize mapping against an unpinned/different atlas release.

## 6. Test strategy

### Unit

- ontology parser;
- aliases/laterality;
- mapping validation;
- reverse lookup;
- visual-state aggregation;
- presentation store;
- chat context.

### Component

Use mock `AnatomyViewerPort` for most deterministic tests.

### Integration

Use real pinned atlas loader in a narrow integration suite.

### E2E

Use a WebGL-capable browser for selection/focus/layer smoke tests.

### Failure tests

- atlas 404;
- invalid catalog;
- unknown mapped anatomy ID;
- WebGL unavailable;
- WebGL context loss where testable;
- missing canonical BodyRegionId on legacy facts.

## 7. Data migration rules

- migration is additive;
- never destroy existing `body_region` text;
- explicit UI region selection is highest-confidence canonical identity;
- deterministic alias mapping may populate canonical ID;
- uncertain/ambiguous legacy values remain null;
- no LLM-only bulk backfill without review/evidence rules;
- all migrated durable changes retain existing provenance semantics.

## 8. Security/privacy rules

- Vanatome/atlas receives no BodyState or user data;
- atlas requests contain only static release paths;
- no health content in query strings;
- no user ID in anatomy asset URL;
- self-host before production acceptance;
- third-party package dependency goes through normal supply-chain/lockfile review.

## 9. Rollback

Rollback must be possible without mutating durable health history.

### UI rollback

Feature-level fallback:

```text
BodyExplorer3D -> BodyExplorerFallback2D
```

### Contract rollback

`body_region_id` is additive/optional, so older UI can continue using `body_region`.

### Atlas rollback

Switch config to previous immutable atlas release only if its corresponding mapping version is also restored.

Never run atlas N with mapping built for atlas M.

## 10. Definition of done

The plan can be archived only when:

1. ADR 0006 is implemented.
2. Vanatome is the primary State body viewer.
3. Region mode and anatomy mode are both complete.
4. BodyRegionOntology v1 is implemented and validated.
5. Pinned atlas mapping is complete and CI validated.
6. BodyState has additive canonical region identity.
7. 3D ↔ BodyState linking works in both directions.
8. selected region/structure can be attached to Chat.
9. accessibility and WebGL fallback are complete.
10. atlas is self-host-ready with correct license/provenance.
11. performance measurements and visual acceptance are recorded.
12. typecheck/lint/unit/integration/E2E/build suites pass.
13. docs are updated to `Implemented` state.

## 11. Implementation checkpoint template

Append each completed ticket here in this format:

```text
### BODY3D-xxxx — COMPLETE

Commit:
Files:
Behavior:
Tests:
Measurements:
Risks/follow-up:
```

## 12. Upstream references

- Vanatome: https://github.com/vixotic/Vanatome
- Atlas contract: https://github.com/vixotic/Vanatome/blob/main/docs/atlas-contract.md
- Anatomy conversion/release pipeline: https://github.com/vixotic/Vanatome/blob/main/docs/anatomy-pipeline.md
- npm viewer package: https://www.npmjs.com/package/@vixotic/vanatome-react
- npm atlas package: https://www.npmjs.com/package/@vixotic/vanatome-atlas


## 13. Runtime loading / staging performance checkpoint — 2026-08-27

### BODY3D-RUNTIME-LOAD — COMPLETE

**Observed incident**

- consultation shell could remain in skeleton state for roughly 20 seconds on the remote staging path;
- business workspace was visually blocked even though its read-model request was already fast;
- the 3D body fell back to 2D while Chrome reported `ERR_HTTP2_PING_FAILED 200 (OK)` for the self-hosted atlas model.

**Root cause**

1. API/DB latency was not the bottleneck. Staging logs measured `/health-workspace` at roughly 9–30 ms and `/consultations/:id/thread` at roughly 19–41 ms.
2. Web bootstrap paid a serial waterfall: auth refresh -> `/me` -> profile -> lazy consultation route -> page queries -> lazy chat panel. Remote tailnet RTT/chunk retries amplified every serial boundary.
3. `ConsultationPage` incorrectly coupled the independent workspace read model to thread pending/error state, so a slow chat thread kept already-available business data behind `InfoPanelSkeleton`.
4. The initial Vanatome path loaded the monolithic `full-body` GLB (31,849,556 bytes). Nginx had the file and returned HTTP 200 locally, but the remote HTTP/2 stream ended after only part of the body was transferred. This was a transport interruption, not an asset 404.

**Implemented correction**

- preload the consultation route during auth/profile bootstrap;
- treat successful refresh as the auth boundary and hydrate display-only `/me` data concurrently with protected-route/profile loading;
- preload `AssistantChatPanel` immediately when the workbench route mounts;
- decouple State/Analysis/Plan/Progress workspace rendering from consultation thread loading;
- redesign app/business/3D skeletons to match final workbench geometry and allow body records to appear while 3D continues loading;
- use Vanatome native atlas composition: initial `regional-anatomy` body shell, then skeletal/muscular/nervous systems on demand;
- enable nginx gzip for `model/gltf-binary`;
- expose a stable `data-viewer-state=ready` signal from the real Vanatome `onReady` event so E2E cannot confuse metadata readiness with rendered-model readiness.

**Measured result on canonical staging**

```text
real viewer cold ready     2062 ms
real viewer warm ready     1477 ms
E2E                        1 passed (92.0 s)
initial model              regional-anatomy, ~6.3 MB raw / ~3.8 MB gzip
monolithic full-body GLB   no longer requested on initial path
full web tests             40 files / 197 tests passed
typecheck / lint / build   passed
```

The E2E run also exercised all 35 canonical BodyRegion mappings and progressively fetched only systems reached by the test. Business BodyState records remained visible independently while the 3D viewer hydrated.

## 13. Runtime transport + browser diagnostics checkpoint — 2026-08-27

A real staging incident exposed two browser-only failure modes that unit/API health checks did not cover.

### Consultation SSE transport incident

Observed user run:

- AI service returned successfully and Go persisted the full assistant answer.
- durable events reached `run.completed`, `message.completed`, and `stream.done`.
- the browser-facing SSE socket disconnected mid-run (`broken pipe`).
- the same `request_id` was subsequently replayed while the durable run was still `running` and the old behavior returned `409 RUN_IN_PROGRESS`.
- no durable `/runs/:runId/events` recovery request was observed from that browser session, leaving the UI in `processing` despite durable completion.

Corrections:

1. Same-`request_id` retries against a running/waiting run are now treated as **transport reattachment**, not a second business command.
2. The reattached response replays durable runtime events from seq 1 and tails the event log until stored `stream.done`.
3. The web runtime keeps its eager durable watcher, but now has a guaranteed post-disconnect fallback recovery path if that watcher never attached.
4. SSE write errors now include `run_id`, `conversation_id`, event type, and seq.
5. Real staging validation sent two concurrent POSTs with one request ID: both returned 200, the second replayed `run.started`, and both reached one `stream.done`.

### Body Explorer browser failure incident

The affected browser loaded `catalog.json` and `regional-anatomy.metadata.json` successfully but never requested the GLB. A prior reproduction on the same client showed `THREE.WebGLRenderer: Context Lost`, so this failure is before model transfer and belongs to browser/Viewer/WebGL initialization, not atlas distribution.

Corrections:

1. WebGL capability detection now explicitly releases its temporary context using `WEBGL_lose_context` instead of consuming a browser context until GC.
2. A transient WebGL failure automatically recreates the Viewer once before falling back to 2D.
3. Added authenticated, privacy-safe `/api/v1/client-diagnostics` telemetry. It records only operational fields (category/event/code/phase/run/request/resource/error message) and never consultation/body-state text.
4. Body3D emits diagnostic milestones/errors for atlas metadata, model load start, viewer ready, Vanatome errors, render-boundary errors, WebGL unavailable/retry/restored.
5. Chat transport emits diagnostics for SSE read failure, durable watcher start/terminal/failure, and guaranteed fallback recovery.
6. Staging/production GORM logging now defaults to Warn rather than Info so interpolated SQL does not routinely place consultation content in shared logs; local development remains Info and `DB_LOG_LEVEL` can override.

Validation:

- Go consultation/handler/database tests pass.
- Web typecheck + lint pass.
- Web suite: 40 files / 197 tests pass.
- Web production build passes.
- Real HTTPS Body Explorer E2E passes after the change.
- Client diagnostic ingestion observed `atlas_metadata_ready -> model_load_started -> viewer_ready` from real Chromium.
- Active-run transport reattach staging probe: first POST 200, same-request retry 200, both streams reached one `stream.done`.
