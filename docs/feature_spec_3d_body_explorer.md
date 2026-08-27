# Feature Spec: 3D Body Explorer

> Status: Ready for implementation
> Date: 2026-08-27
> Architecture: `architecture/body-explorer-3d-anatomy.md`
> Domain mapping: `architecture/body-region-ontology.md`
> Decision: ADR 0006

## 1. Product statement

3D Body Explorer is the spatial interaction layer of the BodySense State workspace.

It replaces the coarse primary SVG silhouette with a precise, interactive 3D anatomy viewer while keeping the existing product principle intact:

> The user should understand their current body state without learning an anatomy application.

The 3D body is therefore simple by default and detailed on demand.

## 2. Primary user jobs

A user should be able to:

1. glance at the body and see which areas currently contain meaningful records;
2. click a body area and immediately view the corresponding BodyState records;
3. rotate the body to inspect front/back/side regions;
4. focus a selected body area without manually navigating the camera;
5. drill into real anatomical structures when they want more detail;
6. switch relevant anatomy layers such as muscle, skeleton, and nerves;
7. ask BodySense about the currently selected region or anatomy structure;
8. move from a BodyState record back to the correct location on the 3D body;
9. retain the same usable State workspace when WebGL or the atlas fails.

## 3. Non-goals

- teaching complete medical anatomy as a primary product flow;
- using 3D selection as a medical diagnosis;
- exposing all 807+ anatomy entries as a default sidebar;
- allowing the user to manually edit durable health truth by painting the 3D model;
- turning the State tab into a separate full-screen anatomy product;
- requiring a new backend service for 3D rendering;
- storing Vanatome IDs as the only durable body region key.

## 4. Default State experience

### 4.1 Layout

Desktop keeps the current two-surface application shell:

```text
AI conversation | Workbench
```

Within `状态`:

```text
┌──────────────────────┬──────────────────────────────────────┐
│                      │ 身体记录 / selected region           │
│       3D body        │                                      │
│                      │ current facts                        │
│ rotate / click       │ observations                         │
│                      │ hypotheses when relevant              │
│ region summary       │ actions                              │
└──────────────────────┴──────────────────────────────────────┘
```

The 3D body remains visually quiet and subordinate to actual BodyState information.

### 4.2 Initial camera

Default camera:

- full-body framing;
- near-front three-quarter or straight front view chosen after visual evaluation;
- no auto-rotation;
- no continuous animation.

### 4.3 Region visualization

Regions with no current records remain neutral.

Regions with current state receive restrained visual emphasis derived from BodySense status.

Recommended semantic treatment:

- `observed`: subtle accent;
- `stable`: neutral marked state;
- `improving`: green/success;
- `worsening`: amber/red attention state;
- `safety_review`: strongest safety state.

Color must be accompanied by text status in the region/list inspector.

## 5. Region interaction

### 5.1 Hover

Desktop hover may show a lightweight label:

```text
右肩 · 3 条记录
```

Hover must not be the only way to access information.

### 5.2 Click / tap

Clicking a mapped structure selects its BodyRegionId.

Immediate effects:

1. selected region receives persistent visual selection;
2. adjacent BodyState content filters or scrolls to the region;
3. region label becomes visible;
4. selected region remains selected until another selection/reset;
5. camera may make a subtle focus adjustment if this improves spatial understanding.

The click does not mutate durable health data.

### 5.3 Clicking the selected region again

Do not unexpectedly deselect if that would make the inspector disappear.

Preferred behavior:

- selection remains stable;
- explicit `返回全身` / reset handles navigation out.

## 6. Camera controls

Required:

- pointer drag rotate;
- scroll/pinch zoom;
- supported pan behavior;
- smooth focus to selected structure/region;
- reset to whole body.

Optional compact presets if they improve UX after browser review:

- front;
- back;
- left;
- right.

Controls must remain visually secondary.

## 7. Region inspector

When a region is selected, the primary business content should show:

```text
右肩

3 条相关记录

抬高手臂时疼痛             待观察
右侧斜方肌持续紧张         已确认
活动范围下降               待确认

[询问 BodySense] [深入查看]
```

Do not add a large explanatory card around this content.

The inspector may reuse/refactor the existing BodyStateWorkbench rather than creating duplicate state rendering.

## 8. Anatomy drill-down

### 8.1 Entry

Anatomy mode is entered deliberately through a control such as:

```text
深入查看
```

It is not automatically shown just because a user clicks a region.

### 8.2 Anatomy mode UI

The body view remains in place.

A compact contextual control row may appear:

```text
区域 | 肌肉 | 骨骼 | 神经 | 更多
```

Only systems relevant to the selected region and supported by the pinned atlas should be promoted.

`更多` may expose the remaining supported systems without cluttering the primary UI.

### 8.3 Selecting an anatomy structure

When a structure is selected:

- keep it highlighted;
- show structure name;
- show parent/hierarchy context;
- show its owning BodySense region if mapped;
- allow focus/isolate;
- provide `询问 BodySense`;
- do not claim the structure is the cause of any symptom merely because it is selected.

### 8.4 Isolate

Isolation is an advanced inspection action.

Use Vanatome-supported isolation/translucent-parent behavior where it improves context.

There must always be an obvious reset/back action.

## 9. BodyState → 3D interaction

Clicking a BodyState row that has `body_region_id` should:

1. select that region in the Body Explorer;
2. focus the region's preferred anatomy target;
3. preserve the current record's scroll/focus context where practical.

If the record lacks canonical region identity:

- do not guess;
- keep the record usable;
- no 3D focus is required.

## 10. Chat integration

### 10.1 Ask about region

Clicking `询问 BodySense` attaches lightweight structured context to the existing single conversation.

Composer visual:

```text
[右肩 ×]
和 BodySense 说说你的身体感受…
```

The user can remove the context chip before sending.

### 10.2 Ask about anatomy structure

In anatomy mode:

```text
[右肩 · Deltoid ×]
```

The exact structure label comes from the pinned atlas registry.

### 10.3 Server authority

Client context is a navigation hint, not the sole health context.

The AI runtime must continue to retrieve authoritative BodyState/Diagnosis data through existing server/runtime rules.

## 11. Empty state

When there are no body records:

- 3D body still renders as the discoverable input surface;
- no permanent explanatory header is needed;
- a compact empty hint may appear near the body/record area:

```text
还没有身体记录
和 BodySense 说说最近哪里不舒服。
```

The model should not show fake colored regions.

## 12. Loading state

Loading should preserve workbench stability.

Expected sequence:

```text
State workspace shell immediately
-> neutral lightweight body placeholder / skeleton
-> atlas loading progress if needed
-> interactive 3D body
```

Do not block the entire application shell while the atlas loads.

## 13. Error / WebGL fallback

If Vanatome fails to load or WebGL is unavailable:

- State workspace remains functional;
- current 2D body representation or a simplified accessible region list is shown;
- BodyState records remain available;
- retry is offered when meaningful;
- selected BodyRegionId is retained across fallback if possible.

Example:

```text
3D 身体视图暂时不可用   [重试]

[2D body fallback]
```

Avoid frightening technical error text such as `WebGL context lost` as the primary user message.

## 14. Safety state

Safety status remains a BodySense domain state.

If a selected region has an active safety review:

- region has strong safety emphasis;
- textual safety message remains visible in the BodyState area;
- anatomy exploration never hides the safety message;
- entering anatomy mode does not downgrade or reinterpret the safety state.

## 15. Mobile / tablet

### Desktop

- 3D body and record inspector may remain side-by-side;
- drag/scroll interactions enabled;
- hover enhances but is optional.

### Tablet

- keep 3D body large enough for touch selection;
- inspector may stack below or beside depending on width;
- touch targets >= 40–44px for explicit controls.

### Mobile

- 3D body can occupy the top portion of the Workbench view;
- record inspector follows below;
- no hover-only UI;
- layer controls collapse into a compact sheet/popover;
- model must not consume an entire long page height before useful records.

## 16. Visual style

The model should fit the current native-desktop-inspired Workbench:

- neutral graphite / muted anatomy colors by default;
- BodySense green reserved for meaningful selection/improvement;
- no neon sci-fi glow as the default product style;
- no decorative auto-rotation;
- subtle highlight transitions;
- no thick card around the model;
- 3D canvas sits directly on the Workbench surface.

Vanatome's holographic visual capability may be toned down to match BodySense rather than copied verbatim.

## 17. Motion

Motion should feel deliberate and spatial:

- region focus: smooth, short camera transition;
- reset: smooth return to full-body framing;
- layer transitions: restrained fade/visibility change;
- no continuous floating/pulsing body animation;
- respect `prefers-reduced-motion` where the viewer/adapter permits.

## 18. Accessibility

Required:

- keyboard-accessible region list as an equivalent to canvas selection;
- status text independent of color;
- clear focus indicators on controls;
- screen-reader region labels/counts;
- semantic loading/error announcements;
- 2D/list fallback when 3D cannot be used;
- no essential feature hidden behind hover.

## 19. Analytics / telemetry

No user health data is sent to Vanatome.

Internal product telemetry, if later desired, should use coarse interaction events without health content, e.g.:

- body_explorer_loaded;
- region_selected;
- anatomy_mode_opened;
- anatomy_structure_selected;
- viewer_fallback_used.

This is optional and is not required for initial implementation.

## 20. Performance acceptance

Before final cutover, record measured values for:

- lazy JS chunk size;
- full-body atlas transfer size;
- first interactive body time on cold/warm cache;
- focus interaction responsiveness;
- memory after repeated State/Analysis tab switching;
- WebGL context recovery/fallback behavior.

The 3D stack must be code-split so non-State routes and first application shell render do not synchronously depend on Three.js/Vanatome.

## 21. Licensing UX

Attribution should be accessible without becoming permanent visual noise.

Preferred placement:

```text
... menu / About BodySense / Anatomy attribution
```

or a small `资料来源 / Attribution` entry in the advanced anatomy surface.

Required attribution must still be present in redistributed static assets and repository notices.

## 22. Acceptance criteria

### Primary flow

- [ ] State tab loads interactive full-body Vanatome viewer.
- [ ] User can rotate, zoom, select, focus, isolate and reset.
- [ ] BodyState regions with data are visually marked.
- [ ] Clicking a region filters/highlights related BodyState data.
- [ ] Clicking a BodyState record focuses the corresponding body region.
- [ ] User can deliberately enter anatomy drill-down.
- [ ] Relevant anatomy systems can be switched.
- [ ] Anatomy structures can be selected and focused.
- [ ] Selected structure resolves to BodySense region when mapped.
- [ ] `询问 BodySense` passes region/structure context to the single chat composer.

### Domain safety

- [ ] Viewer selection never mutates BodyState by itself.
- [ ] BodySense uses canonical BodyRegionId instead of Vanatome IDs for durable region identity.
- [ ] Hypothesis/anatomy selection is not presented as confirmed diagnosis.

### Reliability

- [ ] Atlas load failure leaves State usable.
- [ ] WebGL unsupported/lost path has a 2D/list fallback.
- [ ] Mapping validation rejects unknown anatomy IDs.
- [ ] Pinned atlas release is explicit.

### Accessibility

- [ ] Every region is selectable through non-canvas controls.
- [ ] Status is understandable without color.
- [ ] Essential interactions work without hover.

### Licensing

- [ ] Vanatome software notices are recorded.
- [ ] Z-Anatomy/CC BY-SA atlas attribution is present.
- [ ] Self-hosted atlas keeps provenance/attribution metadata.

## 23. References

- ADR: `adr/0006-adopt-vanatome-3d-anatomy-engine.md`
- Architecture: `architecture/body-explorer-3d-anatomy.md`
- Ontology: `architecture/body-region-ontology.md`
- Plan: `plan/active/2026-08-27-vanatome-3d-body-explorer.md`
