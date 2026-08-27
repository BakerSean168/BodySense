# Vanatome adapter contract

This directory is the BodySense boundary around Vanatome. Product, durable
BodyState, chat, and Go contracts must depend on `AnatomyViewerPort` rather
than Vanatome package types.

## Pinned runtime verified for BODY3D-1101/1102

- `@vixotic/vanatome-react`: `0.1.6`
- `@vixotic/vanatome-atlas`: `0.1.4`
- official human atlas release: `1.4.0`
- atlas build id: `994e6cc8ffbb212e`
- `three`: `0.180.0` (the Vanatome peer range is `^0.180.0`)
- `@react-three/fiber`: `9.6.1`
- `@react-three/drei`: `10.7.7`

## Actual 0.1.6 React API used by BodySense

`VanatomeViewer` is controlled by `selectedId`, `hoveredId`, `isolation`,
`visibleLayers`, and `displayMode`. Focus and reset are request counters:
`focusRequestKey` and `resetViewKey`. `useVanatomeController()` owns those
counters plus selection, isolation, and visible layers; display mode remains
host-owned state.

Supported display modes in the installed types are `normal`, `xray`, and
`ghost`. Isolation modes are `selected`, `parent`, and `parent-context`.
Viewer errors are currently `model-load-failed` and `webgl-context-lost`.

The atlas package exposes `createOfficialHumanAtlas()` and `loadProfile()`.
BodySense first loads and verifies the catalog version/build, then loads the
`full-body` profile. The catalog's system ids match the structure `layer`
values used by Vanatome visibility filtering.

### 0.1.6 camera quirk

`initialCameraTarget` is used by Vanatome's reset/focus camera helper but is
not copied into the initial `OrbitControls.target` on first mount. The pinned
full-body GLB was therefore measured and translated by its vertical center so
that first mount and reset both frame the full body around the viewer origin.
This is viewer-only geometry and is not a BodyRegion mapping.
