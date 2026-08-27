# BodySense 3D Body Explorer Architecture

> Status: Target architecture / ready for implementation
> Date: 2026-08-27
> Decision source: ADR 0006
> Business source of truth: ADR 0004, Longitudinal BodyState Domain Model
> UI source of truth: Desktop Workbench UI/UX

## 1. Purpose

Body Explorer turns the State workspace body map from a coarse SVG indicator into a spatial interface over the user's Longitudinal BodyState.

The product goal is not to build an anatomy teaching application. The goal is to let a user answer, with minimal cognitive load:

- **Where** on my body are there current records?
- What has been observed or confirmed for this area?
- Is it improving, stable, worsening, or still uncertain?
- What analysis or plan items relate to this area?
- If I want more detail, what real anatomical structures are present here?
- Can I ask BodySense about the selected area without manually restating the context?

The 3D model therefore acts as a **spatial index** and **inspection surface**. Durable health meaning remains in BodyState / Diagnosis / Treatment.

## 2. Upstream foundation

BodySense adopts Vanatome rather than implementing a custom anatomy viewer.

Current upstream capabilities verified during architecture research include:

- controlled React viewer based on React Three Fiber / Three.js;
- stable anatomy structure IDs;
- full-body profile loading;
- system-specific loading;
- recursive hierarchy;
- hover / picking;
- orbit / pan / zoom;
- bounds-based focus;
- isolate / reset;
- normal / x-ray / ghost display modes;
- system/layer visibility;
- loading, progress, ready, error, retry, and WebGL-loss states;
- versioned atlas catalog + metadata + GLB contract;
- anatomy identity embedded in glTF `extras`.

The software package and atlas are separate licensing surfaces. See ADR 0006 and section 15.

## 3. Product interaction model

The system exposes one implementation with two levels of complexity.

### 3.1 Region mode — default

Region mode is the ordinary BodySense State experience.

The user sees:

- a neutral full-body 3D model;
- subtle highlights only for BodySense regions that have active state;
- click / hover affordance;
- drag to rotate, wheel/gesture to zoom;
- a small reset/front/back control set if needed;
- BodyState details beside the model.

The user does **not** see all anatomical systems by default.

Example:

```text
3D body                         身体记录

  [right shoulder highlighted]  右肩
                                - 抬高手臂时疼痛
                                - 斜方肌紧张
                                - 活动范围下降
```

### 3.2 Anatomy mode — deliberate drill-down

After selecting a BodySense region, the user may choose `深入查看`.

Anatomy mode can then expose relevant Vanatome-supported structures and systems, for example:

- regional anatomy;
- muscular;
- skeletal;
- nervous;
- other available systems when contextually useful.

The user may:

- select a structure;
- focus the camera;
- isolate the structure;
- use x-ray / ghost modes;
- navigate the parent/child hierarchy;
- return to the owning BodySense region.

This is progressive disclosure, not a separate product generation.

## 4. Architecture overview

```text
Consultation / BodyState / Diagnosis / Treatment projections
                         |
                         | BodyRegionId
                         v
                 BodyRegionOntology
                    /          \
                   /            \
        region->anatomy      anatomy->region
                 |                 |
                 v                 v
             BodyExplorerStore / selectors
                         |
                         v
                 AnatomyViewerPort
                         |
                         v
                   VanatomeAdapter
                         |
          +--------------+--------------+
          |                             |
 @vixotic/vanatome-react       @vixotic/vanatome-atlas
          |                             |
          +--------------+--------------+
                         |
                 versioned atlas release
                         |
                    GLB + registry
```

## 5. Proposed Web feature boundary

```text
apps/web/src/features/body-explorer/
├── components/
│   ├── BodyExplorer3D.tsx
│   ├── BodyExplorerFallback2D.tsx
│   ├── BodyRegionInspector.tsx
│   ├── AnatomyInspector.tsx
│   ├── AnatomyLayerControls.tsx
│   ├── BodyViewControls.tsx
│   └── BodyExplorerLoadingState.tsx
├── adapters/
│   ├── anatomyViewerPort.ts
│   └── vanatomeAdapter.ts
├── hooks/
│   ├── useBodyExplorerAtlas.ts
│   ├── useBodyRegionState.ts
│   └── useBodyExplorerChatContext.ts
├── model/
│   ├── bodyExplorerStore.ts
│   ├── bodyExplorerSelectors.ts
│   ├── bodyRegionOntology.ts
│   ├── anatomyMapping.ts
│   └── anatomyTypes.ts
├── data/
│   ├── body-regions.v1.json
│   └── vanatome-region-map.v1.json
└── index.ts
```

Vanatome APIs must not be imported outside this feature boundary except for narrowly defined shared types if unavoidable.

## 6. Core types

### 6.1 Product region identity

```ts
export type BodyRegionId = string & { readonly __brand: "BodyRegionId" };
```

Concrete IDs are defined by the versioned BodyRegionOntology rather than inferred ad hoc from user strings.

### 6.2 Anatomy identity

```ts
export type AnatomyStructureId = string & {
  readonly __brand: "AnatomyStructureId";
};
```

Anatomy IDs are upstream atlas identities and remain presentation/visualization identities.

### 6.3 Explorer mode

```ts
export type BodyExplorerMode = "region" | "anatomy";
```

### 6.4 Region presentation

```ts
export type BodyRegionVisualState =
  "none" | "observed" | "stable" | "improving" | "worsening" | "safety_review";
```

These states are derived from BodySense data.

## 7. BodyExplorerStore

Only presentation state belongs in the client store.

Proposed state:

```ts
interface BodyExplorerState {
  mode: "region" | "anatomy";
  hoveredRegionId: BodyRegionId | null;
  selectedRegionId: BodyRegionId | null;
  hoveredAnatomyId: AnatomyStructureId | null;
  selectedAnatomyId: AnatomyStructureId | null;
  isolatedAnatomyId: AnatomyStructureId | null;
  visibleSystems: string[];
  displayMode: "normal" | "xray" | "ghost";
  cameraPreset: "free" | "front" | "back" | "left" | "right";
}
```

Do not persist ephemeral hover state.

Persisting the selected region across reload is optional and must not be confused with durable health state.

## 8. AnatomyViewerPort

The BodySense UI talks to a stable application-owned port.

Suggested interface:

```ts
export interface AnatomyViewerPort {
  select(id: AnatomyStructureId | null): void;
  focus(id: AnatomyStructureId): void;
  isolate(id: AnatomyStructureId | null): void;
  setVisibleSystems(systemIds: string[]): void;
  setDisplayMode(mode: "normal" | "xray" | "ghost"): void;
  resetView(): void;
  setViewPreset(preset: "front" | "back" | "left" | "right"): void;
}
```

`setViewPreset` may be implemented outside Vanatome if the package does not expose it directly; it still belongs behind the adapter boundary.

## 9. Vanatome adapter responsibilities

`VanatomeAdapter` owns all third-party-specific translation:

- atlas loading;
- upstream controller wiring;
- selected / isolated IDs;
- focus request keys;
- visible layers/systems;
- display modes;
- loading / ready / error callbacks;
- WebGL-loss callbacks;
- upstream hierarchy access;
- structure selection callbacks;
- conversion from upstream IDs into branded `AnatomyStructureId` values.

It does **not** own:

- BodyState queries;
- Diagnosis state;
- Treatment state;
- status color decisions;
- region mapping semantics;
- chat prompt/context construction.

## 10. BodyRegionOntology integration

The ontology is described separately in `body-region-ontology.md`.

Every region record contains at least:

```ts
interface BodyRegionDefinition {
  id: BodyRegionId;
  labelZhCN: string;
  labelEn: string;
  parentId: BodyRegionId | null;
  side: "left" | "right" | "midline" | null;
  aliases: string[];
  anatomyIds: AnatomyStructureId[];
}
```

The anatomy mapping must be generated/validated against the exact pinned atlas registry.

Unknown or removed anatomy IDs fail validation.

## 11. BodyState → 3D projection

### 11.1 Source data

The 3D model uses existing durable projection inputs:

- active Facts;
- confirmed Observations;
- pending observations when explicitly represented as pending;
- active safety state;
- trend/lifecycle information.

Hypotheses may be represented in adjacent UI but should not paint a region as if they were a confirmed body fact.

### 11.2 Region aggregation

A selector groups relevant records by canonical `BodyRegionId`.

Example:

```text
shoulder.right
  facts: 2
  observations: 1
  trend: worsening
  safety: false
```

### 11.3 Visual precedence

Recommended precedence:

```text
safety_review
  > worsening
  > improving
  > stable
  > observed
  > none
```

Color is never the only status indicator. Text/list equivalents remain adjacent.

### 11.4 No anatomy-derived diagnosis

No atlas structure automatically implies a concern, condition, diagnosis, or treatment.

## 12. 3D → BodyState interaction

When the user clicks an anatomy structure:

1. Vanatome emits/selects `anatomyId`.
2. `anatomyMapping` resolves the nearest owning BodyRegionId.
3. BodyExplorerStore updates `selectedAnatomyId` and `selectedRegionId`.
4. BodyState inspector filters to that region.
5. If anatomy mode is active, AnatomyInspector displays the structure and hierarchy.
6. No durable mutation occurs.

When the user clicks a BodyState record:

1. resolve its canonical BodyRegionId;
2. select that region;
3. select/focus the region's preferred anatomy target;
4. update the 3D highlight.

## 13. Chat context bridge

The selected region/structure can be attached to the existing single conversation without rendering raw JSON to the user.

Conceptual context payload:

```ts
interface BodyExplorerChatContext {
  bodyRegionId: BodyRegionId;
  bodyRegionLabel: string;
  anatomyStructureId?: AnatomyStructureId;
  anatomyStructureLabel?: string;
  relatedFactIds: string[];
  relatedObservationIds: string[];
}
```

The composer may render a lightweight context chip such as:

```text
关于：右肩 ×
```

This context is presentation/input context only. The AI still receives authoritative server-side BodyState data according to existing runtime rules.

## 14. Contract migration

Current body region values are largely free-text (`body_region`).

The target additive contract is:

```json
{
  "body_region_id": "shoulder.right",
  "body_region": "右肩"
}
```

Migration rules:

1. `body_region_id` is additive first.
2. Existing text remains for compatibility and user display.
3. New user-selected 3D regions write canonical IDs.
4. Existing free-text records are mapped conservatively through aliases/normalization.
5. Ambiguous legacy text remains unmapped rather than guessed.
6. Diagnosis / Treatment can adopt canonical IDs after BodyState migration proves stable.

This is a business-contract change and must be implemented through the Go-owned durable boundary rather than by Web-only local rewriting.

## 15. Atlas loading and hosting

### 15.1 Integration stage

The implementation spike may use the upstream immutable release catalog observed in the Vanatome README:

```text
https://atlas.vanatome.vixotic.in/releases/1.4.0/catalog.json
```

The exact version must be explicit.

### 15.2 Production stage

Self-host the pinned release under an immutable path, for example:

```text
/static/anatomy/vanatome/1.4.0/
  catalog.json
  ... metadata
  ... GLB files
  ATTRIBUTION.txt
  LICENSES/
```

The release directory must never be modified in place.

A new atlas version is a new directory.

### 15.3 CORS and cache

Prefer same-origin delivery through the BodySense Web static/CDN boundary.

Recommended cache semantics for versioned assets:

```text
Cache-Control: public, max-age=31536000, immutable
```

The catalog URL itself is also versioned, so it may use immutable caching.

## 16. Licensing and provenance

Before redistributing atlas assets, BodySense must preserve:

- Vanatome software license notice where required;
- Z-Anatomy/atlas attribution;
- CC BY-SA 4.0 reference;
- modification notice if BodySense changes the atlas;
- upstream provenance fields/manifests where provided.

Do not strip provenance from metadata or generated manifests.

If BodySense modifies atlas material, the adapted atlas material remains subject to the applicable ShareAlike requirement.

## 17. Performance strategy

### 17.1 Code splitting

Three.js / R3F / Vanatome must be dynamically imported with the Body Explorer feature and must not inflate unrelated routes' initial JS bundle.

The Status workspace may show a lightweight skeleton/fallback immediately while the lazy chunk and atlas load.

### 17.2 Asset strategy

Initial implementation should benchmark:

- upstream `full-body` profile;
- system-specific loading;
- cold cache vs warm cache;
- staging over Tailnet and normal public network;
- desktop integrated GPU and a low-end device profile.

Do not invent a custom GLB conversion pipeline until actual measurements show a requirement that upstream assets cannot satisfy.

### 17.3 Rendering strategy

Requirements:

- no continuous decorative animation;
- viewer should be idle when user is not interacting, as far as supported by the upstream package;
- do not enable expensive post-processing just for selection highlighting;
- prefer material/selection states already supported by Vanatome;
- respect reduced-motion preference for camera transitions when possible.

### 17.4 Performance acceptance

The implementation plan must record real measured values before final cutover:

- JS lazy chunk size;
- atlas bytes transferred;
- time to first usable body;
- time to selected-structure focus;
- idle CPU/GPU behavior;
- memory after repeated tab changes;
- WebGL context recovery behavior.

## 18. Accessibility

The 3D canvas is not the only way to access body data.

Required equivalents:

- a keyboard-accessible BodyRegion list/tree;
- selected region reflected in adjacent text UI;
- region status labels beyond color;
- buttons for reset/focus and layer changes;
- no essential interaction requiring hover;
- clear loading/error text;
- SVG/list fallback when WebGL is unavailable;
- focus management after selecting from a list or entering anatomy mode.

The 3D canvas can be marked as supplementary when equivalent semantic controls exist.

## 19. Error and fallback states

### 19.1 Atlas loading error

Show compact retry UI and render the existing 2D body fallback.

### 19.2 WebGL unavailable

Do not block the State workspace. Render the 2D fallback and the same region list/BodyState inspector.

### 19.3 WebGL context loss

Use upstream lifecycle callback/state. Attempt supported recovery; if recovery fails, fall back to 2D without losing selected BodyRegionId.

### 19.4 Missing mapping

If an anatomy structure has no BodySense region mapping:

- anatomy exploration may still display it;
- the UI labels it as anatomy-only context;
- do not associate BodyState records by guess.

### 19.5 Atlas version mismatch

Fail mapping validation in CI. Do not silently run a mapping generated for a different atlas version.

## 20. Testing architecture

### 20.1 Unit

- BodyRegionId parsing/validation;
- alias normalization;
- region→anatomy mapping;
- anatomy→region reverse mapping;
- region visual-state precedence;
- selection store transitions;
- chat context construction.

### 20.2 Contract

- every mapped anatomy ID exists in pinned registry;
- no duplicate canonical BodyRegionId;
- laterality is explicit for bilateral regions;
- mapping file declares atlas release version;
- no unknown parent region.

### 20.3 Component

Mock `AnatomyViewerPort` for most UI tests.

Test:

- clicking region list selects viewer region;
- viewer selection filters BodyState;
- anatomy drill-down opens only after deliberate action;
- errors fall back to 2D;
- chat context chip uses selected region.

### 20.4 Browser/E2E

Real WebGL smoke tests should cover:

- viewer loads;
- rotate/select works;
- selection updates inspector;
- focus/reset works;
- layer switching works;
- browser reload preserves business state;
- unavailable/forced WebGL failure keeps workspace usable.

### 20.5 Visual regression

Capture stable camera presets rather than arbitrary user angles.

At minimum:

- front full body;
- back full body;
- right shoulder selected;
- anatomy drill-down;
- 2D fallback.

## 21. Security and privacy

The atlas is public static content and contains no user data.

User health data must never be encoded into asset URLs, atlas metadata, or third-party requests.

If the official Vanatome endpoint is temporarily used during integration, only static atlas requests go upstream. BodyState, user IDs, selected Fact IDs, Diagnosis data, and conversation content stay inside BodySense.

Production self-hosting removes this external static-asset dependency.

## 22. Migration from current BodyOverview

Current:

```text
WorkspaceViewport
  -> BodyOverview (SVG)
  -> BodyStateWorkbench
```

Target:

```text
WorkspaceViewport
  -> BodyExplorer3D
       -> BodyExplorerFallback2D (conditional)
       -> VanatomeAdapter
  -> BodyStateWorkbench / RegionInspector
```

The old `selectBodyZoneSummaries` model can be kept temporarily as a fallback adapter, then replaced by canonical BodyRegion selectors once the durable contract carries `body_region_id`.

## 23. Definition of done

The architecture is complete when:

1. Vanatome loads as the primary State body surface.
2. Region and anatomy modes both function within the existing single-page workbench.
3. BodySense owns canonical BodyRegionId and mappings.
4. BodyState ↔ 3D selection is bidirectional.
5. Chat can receive selected region/structure context.
6. Production atlas version is pinned and self-host-ready.
7. License/attribution material is present.
8. 2D fallback keeps State usable without WebGL.
9. Mapping, unit, browser, performance and accessibility acceptance suites pass.
10. Existing BodyState / Diagnosis / Treatment domain invariants remain unchanged.

## References

- ADR 0006: `../adr/0006-adopt-vanatome-3d-anatomy-engine.md`
- BodyRegion ontology: `./body-region-ontology.md`
- Feature spec: `../feature_spec_3d_body_explorer.md`
- Active implementation plan: `../plan/active/2026-08-27-vanatome-3d-body-explorer.md`
- Vanatome repository: https://github.com/vixotic/Vanatome
- Atlas contract: https://github.com/vixotic/Vanatome/blob/main/docs/atlas-contract.md
- Anatomy pipeline: https://github.com/vixotic/Vanatome/blob/main/docs/anatomy-pipeline.md
