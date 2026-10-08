# Body Canvas V3 — design / implementation handoff

Status: implementation candidate, browser-verified, production promotion pending.
Penpot file: `24d9d841-759d-81bc-8008-b6423250c2b8`
Product page: `536d2a99-67da-80a5-8008-c113cf18702a`
Foundation page: `536d2a99-67da-80a5-8008-c112dce0b60c`
Component page: `536d2a99-67da-80a5-8008-c1132f6f6c7c`

## Product thesis

Body Canvas V3 makes the body itself the persistent home surface. Chat, BodyState, assessment, accepted Treatment, equipment, history and fine anatomy are task overlays. The game/inventory metaphor is used only for progressive disclosure and fast navigation; health truth is never represented as a fabricated level, XP, score or simulated certainty.

The model is explicitly a **reference body**, not a personal scan or digital twin. Reference motions are teaching/context animations, not posture measurements, tissue-load estimates or diagnostic evidence.

## Penpot coverage

The exact board manifest is versioned in `penpot-manifest.json`.

- Desktop D01–D22: body home, region state, multi-region brush, motion frame, evidence/assessment, accepted plan, focused training, BodyState, equipment, outcome, contextual assistant, keyboard access, history, proposal acceptance, empty/loading/error/safety/insufficient-evidence/correction states, dark canvas and text/anatomy navigation.
- Mobile M01–M08: body home, region sheet, assistant, training inventory, evidence, BodyState, focused motion and touch-accessible tools.
- 495 navigation hotspots connect the prototype journeys.
- The design uses a render from the pinned Vanatome/Z-Anatomy reference asset. It is not a screenshot of the implemented application.

## Implementation mapping

| Penpot concept | Implementation |
| --- | --- |
| D01 / M01 body-first home | `BodyCanvasWorkspace.tsx` |
| D02 region state | canonical BodyRegion selection + real BodyState facts |
| D03 brush / box | depth-tested `SurfacePicker`, multi-region selection reducer |
| D04 / D07 motion | procedural `ReferenceRig`; authored reference motions |
| D05 assessment | existing Diagnosis workspace component inside inventory dialog |
| D06 treatment | existing Treatment component + conservative exercise-to-demo mapping |
| D08 body state | existing BodyState workspace component |
| D09 equipment | `equipment.available` confirmed BodyState facts |
| D10 outcome/history | existing longitudinal workspace flow |
| D11 assistant | existing AssistantChatPanel with explicit spatial context |
| D12/D22 accessibility | visible region selector + keyboard commands + 3D text fallback |
| D18 safety | existing safety capability gates stop motion playback |
| fine anatomy | existing Vanatome BodyExplorer preserved as an on-demand inventory panel |

## Spatial-context boundary

The runtime contract now carries:

- `body_region_id`: primary canonical region.
- `body_region_ids[]`: optional multi-region selection.
- `reference_motion`: only when the user has actually entered Motion Studio.

`reference_motion.source` must equal `reference_animation`. Phase is bounded to 0…1 and motion IDs are server-whitelisted. Go normalizes the context before persistence/forwarding. The Python runtime prompt explicitly states that these fields are navigation/description context and must not be promoted to symptoms, posture measurements, biomechanical evidence or diagnosis.

## 3D architecture

The home canvas intentionally does **not** instantiate the complete Vanatome viewer stack. `vanatomePin.ts` resolves and validates the pinned release/build and regional bundle, while `referenceAtlasLoader.ts` extracts the body-shell geometry needed for the home scene. Fine Anatomy continues to use the official/full Vanatome adapter.

The pinned contract is:

- Vanatome release: 1.4.0
- build: `994e6cc8ffbb212e`
- initial system: `regional-anatomy`
- attribution: Vanatome / Z-Anatomy, CC BY-SA 4.0

The procedural reference rig is deliberately labeled as an educational visualization. It is not a validated anatomical skeleton or personalized biomechanics engine.

## Responsive implementation notes

The implementation follows the prototype semantics rather than pixel-locking every draft coordinate. After browser review, the desktop motion switch was moved to the upper-right so it no longer competes with the model head. At <=760px, region details and inventory behave as bottom sheets / full-height task surfaces; the main dock collapses to the highest-frequency actions. Text region selection remains available independent of WebGL.

## Verification evidence

The branch includes focused unit tests, Go spatial-context tests, Python prompt-contract tests and Playwright flows. The browser acceptance flow verifies:

- body-first initial state;
- real BodyState region data;
- region-only context does not silently attach a default standing motion;
- Motion Studio adds explicit reference-motion context;
- keyboard tool changes and Escape unwind;
- inventory dialog navigation;
- desktop and 390×844 mobile layouts without horizontal overflow.

The generated screenshot `penpot-home.png` is the Penpot reference artifact. Browser screenshots are test artifacts and are intentionally not committed as product assets.
