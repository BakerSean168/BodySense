# BodySense BodyRegionOntology

> Status: Target canonical ontology v1
> Date: 2026-08-27
> Decision source: ADR 0006
> Scope: durable BodySense body-region identity and mapping to anatomy visualization

## 1. Why this exists

BodySense needs a stable product vocabulary for body regions.

The current system still carries free-text region values such as `右肩`, `腰部`, or `颈肩`. Free text is useful for display and natural-language capture, but it is not a sufficient canonical key for:

- left/right distinction;
- filtering and grouping BodyState;
- deterministic 3D highlighting;
- mapping to anatomy structures;
- cross-linking Diagnosis / Treatment / Progress;
- long-term data migration;
- localization.

Vanatome also exposes stable anatomy identifiers, but those IDs describe atlas structures and must not become BodySense business identities.

BodyRegionOntology is therefore a BodySense-owned semantic boundary between health records and anatomy visualization.

## 2. Principles

1. `BodyRegionId` is owned by BodySense.
2. IDs are stable API/domain identifiers, not translated labels.
3. Bilateral regions use explicit `.left` / `.right` suffixes.
4. Midline/general regions do not invent left/right.
5. Region identity is coarser than anatomy structure identity.
6. One BodyRegionId may map to many anatomy IDs.
7. One anatomy ID should map to one nearest primary BodyRegionId for reverse lookup; secondary associations may be explicit metadata rather than ambiguous reverse ownership.
8. Legacy free text is mapped conservatively; ambiguous values remain unresolved.
9. Changing/removing an existing canonical ID is a migration-level breaking change.
10. Region ontology changes are separate from atlas upgrades.

## 3. Canonical type

```ts
export type BodyRegionId =
  | "head"
  | "neck"
  | "chest"
  | "abdomen"
  | "upper_back"
  | "lower_back"
  | "pelvis"
  | "shoulder.left"
  | "shoulder.right"
  | "scapular.left"
  | "scapular.right"
  | "upper_arm.left"
  | "upper_arm.right"
  | "elbow.left"
  | "elbow.right"
  | "forearm.left"
  | "forearm.right"
  | "wrist.left"
  | "wrist.right"
  | "hand.left"
  | "hand.right"
  | "hip.left"
  | "hip.right"
  | "gluteal.left"
  | "gluteal.right"
  | "thigh.left"
  | "thigh.right"
  | "knee.left"
  | "knee.right"
  | "calf.left"
  | "calf.right"
  | "ankle.left"
  | "ankle.right"
  | "foot.left"
  | "foot.right";
```

This v1 set is intentionally product-oriented and musculoskeletal/posture-friendly. It does not attempt to enumerate every organ or anatomical structure.

## 4. Hierarchy

```text
body
├── head
├── neck
├── torso
│   ├── chest
│   ├── abdomen
│   ├── upper_back
│   ├── lower_back
│   └── pelvis
├── upper_limb.left
│   ├── shoulder.left
│   ├── scapular.left
│   ├── upper_arm.left
│   ├── elbow.left
│   ├── forearm.left
│   ├── wrist.left
│   └── hand.left
├── upper_limb.right
│   ├── shoulder.right
│   ├── scapular.right
│   ├── upper_arm.right
│   ├── elbow.right
│   ├── forearm.right
│   ├── wrist.right
│   └── hand.right
├── lower_limb.left
│   ├── hip.left
│   ├── gluteal.left
│   ├── thigh.left
│   ├── knee.left
│   ├── calf.left
│   ├── ankle.left
│   └── foot.left
└── lower_limb.right
    ├── hip.right
    ├── gluteal.right
    ├── thigh.right
    ├── knee.right
    ├── calf.right
    ├── ankle.right
    └── foot.right
```

`body`, `torso`, `upper_limb.*`, and `lower_limb.*` are grouping nodes and do not need to be persisted as ordinary leaf-level health regions unless the user input is genuinely nonspecific.

If a durable record cannot be resolved more precisely, the system may preserve its original text and leave `body_region_id` unset rather than forcing it into a broad grouping node.

## 5. Definition schema

```ts
interface BodyRegionDefinition {
  id: BodyRegionId;
  labelZhCN: string;
  labelEn: string;
  parentId: string | null;
  side: "left" | "right" | "midline" | null;
  aliasesZhCN: string[];
  aliasesEn: string[];
  preferredAnatomyId?: string;
  anatomyIds: string[];
  version: 1;
}
```

### `preferredAnatomyId`

Used as the default focus target for camera movement when the user selects a BodySense region.

It does not define the region by itself.

### `anatomyIds`

The set of atlas structures considered spatially associated with that region in the currently pinned mapping release.

## 6. Region labels and aliases

Initial examples:

| BodyRegionId     | zh-CN       | Example aliases              |
| ---------------- | ----------- | ---------------------------- |
| `neck`           | 颈部        | 颈椎区域、脖子、颈肩中的颈部 |
| `shoulder.right` | 右肩        | 右侧肩部、右肩膀             |
| `scapular.right` | 右肩胛区    | 右肩胛、右肩胛骨周围         |
| `upper_back`     | 上背部      | 背上部、胸椎区域             |
| `lower_back`     | 下背 / 腰部 | 腰、腰背、下背               |
| `pelvis`         | 骨盆        | 骨盆区域、骨盆中央           |
| `gluteal.left`   | 左臀        | 左侧臀部、左臀肌区域         |
| `hip.right`      | 右髋        | 右髋部、右侧髋关节区域       |
| `knee.left`      | 左膝        | 左膝盖、左膝关节区域         |
| `ankle.right`    | 右踝        | 右脚踝、右踝关节区域         |

Aliases are input-normalization aids, not additional canonical IDs.

## 7. Laterality rules

Laterality is clinically/product-wise important and must never be silently discarded.

Examples:

```text
"右肩" -> shoulder.right
"左膝" -> knee.left
"肩膀" -> unresolved unless context establishes side
"腰" -> lower_back
"左右肩都疼" -> two region associations, shoulder.left + shoulder.right
```

If the user's statement is ambiguous, BodySense may ask a clarifying question rather than guess.

## 8. Free-text normalization

### 8.1 Additive target contract

```json
{
  "body_region_id": "shoulder.right",
  "body_region": "右肩"
}
```

`body_region` remains human-readable. `body_region_id` is canonical when available.

### 8.2 Normalization order

1. explicit canonical ID from a UI selection;
2. exact alias match;
3. deterministic normalized alias match;
4. structured AI extraction with side and region confidence;
5. unresolved.

Do not use fuzzy matching alone to create durable canonical identity.

### 8.3 Unresolved record

```json
{
  "body_region_id": null,
  "body_region": "肩颈一带"
}
```

The raw text is preserved until later clarification.

## 9. Mapping to Vanatome

The mapping is versioned separately from the ontology:

```json
{
  "schema_version": 1,
  "ontology_version": 1,
  "atlas_provider": "vanatome",
  "atlas_release": "1.4.0",
  "regions": {
    "shoulder.right": {
      "preferred_anatomy_id": "...verified upstream id...",
      "anatomy_ids": ["...verified ids..."]
    }
  }
}
```

Actual anatomy IDs must be generated/curated from the pinned atlas registry during implementation. This design document deliberately does not invent identifiers.

## 10. Mapping construction process

1. Load the pinned Vanatome catalog.
2. Extract its full structure registry.
3. Generate a machine-readable inventory containing:
   - ID;
   - name;
   - parent ID;
   - system;
   - layer;
   - focus position;
   - mapped-node count where available.
4. Curate region mappings using anatomy names, hierarchy, laterality, and spatial relevance.
5. Validate every mapped ID against the registry.
6. Validate reverse ownership uniqueness.
7. Add fixture tests for all canonical BodyRegionIds.
8. Review mapping changes as data/architecture changes, not casual UI edits.

## 11. Reverse mapping

A selected anatomy structure should resolve to the nearest BodySense region.

Suggested algorithm:

```text
selected anatomy ID
  -> exact reverse mapping exists? use it
  -> else walk parentId chain
  -> first mapped ancestor determines BodyRegionId
  -> else anatomy-only context, no BodyRegionId
```

Do not guess from English labels at runtime once a versioned mapping exists.

## 12. Region aggregation from BodyState

A BodyRegionId may own several health records.

Example:

```text
shoulder.right
  confirmed facts: 2
  pending observations: 1
  trend: worsening
  safety: false
```

The visual selector derives one summary state for the 3D region.

Recommended precedence:

```text
safety_review
> worsening
> improving
> stable
> observed
> none
```

The raw records remain visible in the adjacent list/inspector.

## 13. Relationship to Concern

`Concern` in ADR 0004 is an internal health grouping and is not identical to BodyRegionId.

Examples:

- one concern may span multiple regions (`neck` + `shoulder.right`);
- multiple concerns may exist for the same region over time;
- a region is spatial identity, not a clinical/problem identity.

Therefore:

```text
Concern != BodyRegion
```

A Concern may reference one or more BodyRegionIds.

## 14. Relationship to anatomy structures

```text
BodyRegion
  = product-level spatial location

AnatomyStructure
  = atlas-level anatomical entity
```

Examples conceptually:

```text
shoulder.right
  -> multiple muscles
  -> multiple bones / joints
  -> nerves / regional structures
```

A user's Fact such as `右肩抬高时疼痛` remains a Fact about `shoulder.right`, not automatically a Fact about any particular muscle.

## 15. Relationship to Diagnosis

Diagnosis may consume BodyRegionIds as structured context.

A Diagnosis candidate may optionally reference relevant BodyRegionIds, but must not infer that a selected anatomy structure is confirmed pathology.

Possible future shape:

```json
{
  "candidate_id": "...",
  "related_body_region_ids": ["shoulder.right", "scapular.right"]
}
```

This is a later additive contract, not required for initial viewer rendering.

## 16. Relationship to Treatment and Progress

Treatment interventions and outcomes may reference BodyRegionIds for filtering/visualization.

Examples:

- exercise targets `shoulder.right` and `scapular.right`;
- outcome records improvement in `lower_back`;
- Progress view highlights changed regions on the 3D body.

These features should share the same ontology rather than creating module-specific region strings.

## 17. Versioning

The ontology version changes when:

- a canonical ID is added;
- a canonical ID changes meaning;
- hierarchy changes materially;
- an ID is deprecated or split.

Adding aliases does not necessarily require a major ontology version if canonical meaning is unchanged.

Atlas mapping versioning is independent:

```text
BodyRegionOntology v1
Vanatome mapping v1 for atlas 1.4.0
Vanatome mapping v2 for atlas 1.5.x
```

## 18. Deprecation rules

Never silently recycle an ID for a new meaning.

If a region needs to be split later:

```text
old: shoulder.right
new: shoulder_joint.right + shoulder_superficial.right
```

then old durable data must retain meaning through migration/compatibility mapping. A split must be justified by real product need; anatomy drill-down should usually absorb finer detail without exploding the durable region ontology.

## 19. Validation rules

CI validation must reject:

- duplicate BodyRegionIds;
- missing parent nodes;
- bilateral region without side;
- anatomy IDs absent from the declared atlas release;
- preferred anatomy ID not contained in that region's anatomy IDs unless explicitly documented;
- one anatomy ID mapped as primary reverse owner to multiple regions;
- mapping file atlas version mismatch.

## 20. Initial user-facing selection behavior

### Region selection

Hover or click a structure that maps to a BodyRegionId:

```text
3D structure -> region -> select region -> filter BodyState
```

### Region list selection

```text
BodyState row -> region -> preferred anatomy target -> focus viewer
```

### Anatomy-only structure

If no region mapping exists:

```text
select anatomy structure
-> show anatomy context
-> no BodyState filter unless user explicitly navigates to an owning/mapped region
```

## 21. Implementation artifacts

The implementation should produce:

- `body-regions.v1.json` — canonical region definitions and aliases;
- `vanatome-region-map.v1.json` — exact mapping for pinned atlas release;
- generated TypeScript types/lookup tables where useful;
- validation script/tests;
- migration/normalization helper for legacy free text;
- fixtures covering left/right and ambiguous inputs.

## 22. Definition of done

1. Every canonical BodyRegionId has a documented label and hierarchy.
2. Every bilateral region has explicit left/right IDs.
3. Mapping contains only verified IDs from the pinned Vanatome atlas.
4. Reverse mapping is deterministic.
5. Legacy ambiguous strings are not guessed into durable IDs.
6. BodyState can store canonical ID additively.
7. 3D selection and BodyState filtering share the same ontology.
8. Diagnosis/Treatment remain free to adopt the ontology additively without coupling to Vanatome.
