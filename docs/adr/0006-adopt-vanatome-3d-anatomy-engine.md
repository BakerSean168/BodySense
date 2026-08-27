# ADR 0006: Adopt Vanatome as the 3D Anatomy Visualization Engine

## Status

Accepted — implementation pending

## Date

2026-08-27

## Context

BodySense now exposes one long-lived health workbench whose primary durable health object is the user-scoped Longitudinal `BodyState` defined by ADR 0004.

The current State workspace uses a hand-written SVG body silhouette. That implementation is intentionally simple, but it has reached the limit of what it can support well:

- body areas are visually coarse and cannot represent fine laterality or deeper anatomy;
- the body is not rotatable, zoomable, focusable, or layer-aware;
- selection is limited to a small fixed set of 2D marker coordinates;
- muscle, skeletal, nervous, and other anatomical structures cannot be explored in spatial context;
- the SVG has no stable anatomy registry that can support hierarchy, saved selection, focus, isolation, or future cross-linking to Diagnosis / Treatment;
- improving the SVG further would create a bespoke anatomy-viewer implementation that is not a BodySense differentiator.

BodySense is open source. Reuse of a mature open-source anatomy viewer and open anatomy data is therefore preferred over building and maintaining a custom 3D anatomy engine.

Research on 2026-08-27 identified **Vanatome** as the strongest fit for this boundary. Vanatome provides an embeddable React anatomy viewer and a separate versioned atlas loader. Its viewer is implemented with React Three Fiber / Three.js, while its anatomy atlas is derived from Z-Anatomy. The upstream project exposes stable anatomy identifiers, hierarchy, selection, focus, isolation, system/layer visibility, normal/x-ray/ghost display modes, loading/error/WebGL-loss states, and immutable versioned atlas releases.

At the time of this decision, upstream documents report:

- `@vixotic/vanatome-react` as the React viewer package;
- `@vixotic/vanatome-atlas` as the catalog / atlas loader package;
- a full-body atlas profile exposed through a versioned catalog;
- 807 stable anatomy entries, 749 geometry-mapped anatomy IDs, 984 mapped GLB nodes, and 11 curated systems in the current published atlas description;
- MIT licensing for the viewer/software packages;
- CC BY-SA 4.0 obligations for the Z-Anatomy-derived atlas material.

The upstream atlas contract also makes stable anatomy IDs part of the public compatibility surface and stores anatomy identity in glTF `extras`, which is materially better than binding product logic to arbitrary GLB object names.

## Decision

### 1. Vanatome is the default 3D anatomy engine

BodySense will adopt:

- `@vixotic/vanatome-react` for the controlled React 3D anatomy viewer;
- `@vixotic/vanatome-atlas` for loading a versioned human anatomy catalog;
- the Vanatome/Z-Anatomy-derived atlas as the initial anatomy dataset.

BodySense will not build a parallel general-purpose Three.js anatomy engine unless a concrete Vanatome limitation is demonstrated and cannot be solved through a narrow adapter or upstream contribution.

### 2. Full capability is implemented as one product, with progressive disclosure

This decision does **not** define a sequence of user-facing V1/V2/V3 anatomy products.

The target product supports, from the first complete implementation:

- full-body 3D navigation;
- rotate / pan / zoom;
- hover and click selection;
- camera focus and reset;
- BodySense region selection;
- anatomy-structure drill-down;
- muscle / skeletal / nervous and other supported system layers;
- isolate / translucent-parent exploration where useful;
- BodyState ↔ 3D bidirectional linking;
- selected structure / region → AI conversation context;
- accessible non-3D equivalents and graceful failure states.

Complexity is hidden through progressive disclosure rather than by shipping separate generations of the product.

Default users see a simple region-oriented body experience. Detailed anatomy controls appear only after a user deliberately drills into a body region or chooses an advanced anatomy mode.

### 3. BodySense owns a canonical BodyRegionOntology

Vanatome anatomy identifiers are **not** BodySense domain identifiers.

BodySense will introduce a stable product-owned `BodyRegionId` taxonomy for durable health state and interaction semantics.

Examples include:

- `shoulder.right`
- `scapular.left`
- `lower_back`
- `hip.right`
- `knee.left`

The ontology maps one BodySense region to zero or more Vanatome anatomy structures.

The mapping direction is:

```text
BodyState / Diagnosis / Treatment
        |
        | BodyRegionId
        v
BodyRegionOntology
        |
        | anatomyIds[]
        v
Vanatome adapter
        |
        v
Vanatome atlas / GLB
```

The reverse mapping from a selected Vanatome anatomy structure to its nearest BodySense region is derived by the same mapping layer.

### 4. Durable health truth never depends on a third-party anatomy ID

ADR 0004 remains authoritative:

- Go owns durable BodyState business truth;
- Python owns Agent runtime reasoning;
- Web renders projections and submits user intent.

A Vanatome `anatomyId` may be persisted only as optional presentation/provenance metadata when useful. It must never be the sole canonical body-region identity for BodyState facts, observations, hypotheses, Diagnosis, or Treatment.

### 5. Introduce an explicit viewer adapter boundary

The Web application will wrap Vanatome behind a BodySense-owned adapter / port rather than allowing upstream APIs to spread through product components.

Conceptually:

```ts
interface AnatomyViewerPort {
  selectStructure(id: AnatomyStructureId | null): void;
  focusStructure(id: AnatomyStructureId): void;
  isolateStructure(id: AnatomyStructureId | null): void;
  setVisibleSystems(systems: AnatomySystemId[]): void;
  setDisplayMode(mode: "normal" | "xray" | "ghost"): void;
  resetView(): void;
}
```

The concrete initial implementation is `VanatomeAdapter`.

### 6. BodyState remains the source of status coloration and health meaning

Vanatome supplies anatomy geometry and interaction primitives. It does not decide the health state of a body region.

BodySense derives region presentation from durable product data, for example:

- confirmed / normal context;
- active observation;
- improving;
- stable;
- worsening;
- safety review required.

The 3D model is a spatial index and exploration surface over BodySense health state, not a second health-state model.

### 7. Region mode is default; anatomy mode is a drill-down

The default State experience stays low-cognitive-load:

```text
Region mode
  -> select right shoulder
  -> show BodyState records for right shoulder
  -> optionally choose "深入查看"
  -> anatomy mode
  -> inspect muscles / bones / nerves / supported systems
```

The user is not presented with all 11 anatomy systems as primary navigation on first load.

### 8. The atlas is version-pinned and eventually self-hosted

Implementation may initially load an immutable official Vanatome atlas release during integration and evaluation.

Production must use an explicitly pinned catalog release and should self-host the corresponding catalog, metadata, GLB assets, attribution, and license notices under BodySense-controlled static hosting or CDN infrastructure.

No runtime dependency on an unversioned upstream atlas URL is allowed.

Atlas upgrades are reviewed compatibility changes because anatomy IDs, model geometry, focus targets, and mapping coverage are product-facing dependencies.

### 9. The current SVG becomes a fallback, not the primary implementation

The existing SVG body map will remain temporarily as:

- a WebGL-unavailable fallback;
- a loading / catastrophic-viewer failure fallback where appropriate;
- a migration safety path until the 3D acceptance suite is complete.

It is no longer the target primary body visualization after Vanatome cutover.

### 10. Licensing is treated as a first-class asset boundary

BodySense will preserve a clear license boundary:

- BodySense application code: governed by the BodySense repository license;
- Vanatome viewer/software packages: MIT, subject to upstream terms;
- Vanatome/Z-Anatomy-derived atlas material: CC BY-SA 4.0, with attribution, modification notices, and ShareAlike obligations applied to adapted atlas material.

The repository must include or link the required notices before self-hosting or redistributing atlas assets.

### 11. No anatomy-generated medical truth

Selecting or viewing an anatomical structure must never itself create a health Fact, Observation, Hypothesis, Diagnosis candidate, or Treatment action.

A user click changes presentation context. Durable business changes still pass through existing BodyState / Diagnosis / Treatment commands and their review boundaries.

## Alternatives considered

### A. Build a custom React Three Fiber viewer on a MakeHuman surface model

Pros:

- complete control over visual style;
- CC0 source model can simplify asset licensing;
- region meshes could be designed exactly for BodySense.

Rejected as the primary path because BodySense would still need to build/maintain picking, hierarchy, layer visibility, focus/isolation, anatomy metadata, deeper structure models, atlas versioning, and conversion tooling. That work is not a product differentiator and delays BodyState/anatomy integration.

### B. Consume Z-Anatomy/BodyParts3D directly and build our own atlas pipeline

Pros:

- direct control over source anatomy data;
- no intermediate viewer package.

Rejected as the initial path because Vanatome already provides a curated browser-ready atlas contract, stable anatomy IDs, hierarchy, glTF metadata, conversion/release pipeline, and controlled React viewer. BodySense should reuse that work and only own the semantic/product layer that is specific to BodySense.

### C. Use a generic Three.js/model-viewer component

Pros:

- mature generic 3D tooling;
- low coupling to anatomy-specific upstream code.

Rejected because a generic viewer does not provide an anatomy registry, stable anatomy identities, system hierarchy, focus targets, or anatomy-aware selection/isolation. BodySense would still need to build the missing anatomy product layer.

### D. Keep improving the 2D SVG body map

Pros:

- smallest bundle and implementation complexity;
- excellent fallback/accessibility characteristics.

Rejected as the primary target because it cannot satisfy fine anatomical drill-down or spatial exploration. It remains valuable as a tested fallback.

## Consequences

### Positive

- BodySense obtains a mature 3D anatomy interaction surface without building a bespoke viewer stack.
- Fine anatomy, hierarchy, focus, isolate, layers, and WebGL lifecycle handling are reusable upstream capabilities.
- Stable upstream anatomy IDs are contained behind a product-owned mapping layer.
- Product-level body-region identity remains stable even if the atlas changes.
- The State workspace can evolve from a decorative silhouette into a true spatial interface for Longitudinal BodyState.
- Open-source licensing aligns with the BodySense project model.
- Future upstream improvements to Vanatome can be adopted through a controlled adapter and version upgrade.

### Cost / Risk

- The Web client gains a significant 3D dependency and asset-loading path.
- Full-body GLB assets require loading, caching, versioning, CDN/CORS, and performance management.
- CC BY-SA atlas obligations must be handled correctly.
- BodyRegionOntology and anatomy mappings require deliberate curation and tests.
- WebGL capability, context loss, low-end devices, reduced-motion preferences, and accessibility require explicit fallback behavior.
- Vanatome is a young dependency; package and atlas upgrades must be pinned and reviewed rather than floated automatically.

## Invariants

1. BodyState remains the authoritative durable health-state model.
2. `BodyRegionId` is BodySense-owned and is not a Vanatome anatomy ID.
3. Vanatome is accessed through a BodySense adapter boundary.
4. Anatomy selection is presentation state unless the user performs an explicit domain action.
5. Region health color/status is derived from BodySense data, not from the atlas.
6. Region mode is the default low-complexity experience; anatomy mode is progressive disclosure.
7. Atlas versions are explicit and immutable in production.
8. Atlas licensing and provenance are preserved when redistributed or adapted.
9. The SVG fallback remains until 3D compatibility and accessibility acceptance is complete.
10. Go/Python/Web ownership from ADR 0002 and ADR 0004 is unchanged.

## Initial implementation pins

Research snapshot on 2026-08-27:

- `@vixotic/vanatome-react`: current npm release observed as `0.1.6`;
- `@vixotic/vanatome-atlas`: current npm release observed as `0.1.4`;
- upstream README example atlas: `https://atlas.vanatome.vixotic.in/releases/1.4.0/catalog.json`.

The implementation branch must pin exact resolved versions in the lockfile. These values are a planning baseline, not a permanent requirement; upgrades require explicit review.

## Follow-up

- Implement the architecture in `docs/architecture/body-explorer-3d-anatomy.md`.
- Define the canonical region model in `docs/architecture/body-region-ontology.md`.
- Implement the product behavior in `docs/feature_spec_3d_body_explorer.md`.
- Execute `docs/plan/active/2026-08-27-vanatome-3d-body-explorer.md`.
- Add atlas attribution and self-hosting artifacts before production cutover.

## Upstream references

- https://github.com/vixotic/Vanatome
- https://github.com/vixotic/Vanatome/blob/main/docs/atlas-contract.md
- https://github.com/vixotic/Vanatome/blob/main/docs/anatomy-pipeline.md
- https://www.npmjs.com/package/@vixotic/vanatome-react
- https://www.npmjs.com/package/@vixotic/vanatome-atlas
