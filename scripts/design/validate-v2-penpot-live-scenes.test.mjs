import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { validateLivePenpotScenes } from "./validate-v2-penpot-live-scenes.mjs";

const readJson = (file) =>
  JSON.parse(
    readFileSync(
      new URL("../../docs/design/v2/" + file, import.meta.url),
      "utf8",
    ),
  );
const live = readJson("penpot-live-scenes-manifest.json");
const original = readJson("penpot-scene-manifest.json");
const clone = (value) => JSON.parse(JSON.stringify(value));

test("real Penpot composition manifest cross-references same file and home starts", () => {
  const r = validateLivePenpotScenes(live, original);
  assert.equal(r.ok, true, r.errors.join("; "));
  assert.ok(r.scenes >= 11);
  assert.ok(r.localInteractions >= 8);
  assert.ok(r.viewerPassed >= 2);
  assert.ok(r.flowStarts >= 2);
});

test("a supposedly saved navigation cannot reference an invented board", () => {
  const broken = clone(live);
  broken.localInteractions[0].targetId = "00000000-0000-0000-0000-000000000000";
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("a canvas stage must have its own real shape ID", () => {
  const broken = clone(live);
  broken.scenes[0].canvasId = null;
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("a saved hotspot cannot have two contradictory navigation targets", () => {
  const broken = clone(live);
  broken.localInteractions.push({
    ...broken.localInteractions[0],
    to: broken.localInteractions[1].to,
    targetId: broken.localInteractions[1].targetId,
  });
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("medical commit and erasure cannot be a local design click", () => {
  const broken = clone(live);
  const badKey = "Desktop / Diagnosis / Receipt";
  broken.scenes.push({
    id: "12345678-1234-1234-1234-123456789012",
    canvasId: "12345678-1234-1234-1234-123456789013",
    windowId: "12345678-1234-1234-1234-123456789014",
    kind: "desktop",
    key: badKey,
  });
  broken.localInteractions[0].to = badKey;
  broken.localInteractions[0].targetId = broken.scenes.at(-1).id;
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("a Viewer PASS needs a timestamp and corresponding real navigation", () => {
  const broken = clone(live);
  const passed = broken.viewerQa.find((x) => x.status === "passed");
  assert.ok(passed);
  passed.evidence = "self-reported, without browser";
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("a flow cannot be registered without an actual root scene", () => {
  const broken = clone(live);
  broken.flowStarts[0].startId = "12345678-1234-1234-1234-123456789015";
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("no incomplete design stage can silently claim domain release readiness", () => {
  const broken = clone(live);
  broken.phase = "SIGNED_OFF";
  broken.outstanding = [];
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("identity profile uses one real overlay shared by all five Person facets", () => {
  const r = validateLivePenpotScenes(live, original);
  assert.equal(r.ok, true, r.errors.join("; "));
  assert.equal(live.identityOverlay.openers.length, 5);
  assert.equal(r.overlayQa, true);
});

test("an identity popover cannot silently masquerade as a navigation tab", () => {
  const broken = clone(live);
  broken.identityOverlay.openers[0].type = "navigate-to";
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("a shared identity overlay must be reached from every Person facet", () => {
  const broken = clone(live);
  broken.identityOverlay.openers.pop();
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("a privileged server command cannot be linked as ordinary Penpot navigation", () => {
  const broken = clone(live);
  broken.guardedCommands[0].interactions = 1;
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("multi-segment command names are allowed but empty guards are rejected", () => {
  assert.equal(validateLivePenpotScenes(live, original).ok, true);
  const broken = clone(live);
  const command = broken.guardedCommands.find((g) =>
    g.command.startsWith("person.asset."),
  );
  assert.ok(command);
  command.guard = "";
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("a guarded business action must not reuse a saved navigation hotspot", () => {
  const broken = clone(live);
  broken.guardedCommands[0].id = broken.localInteractions[0].sourceId;
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("all 36 canvas scenes have unique bounded Penpot coordinate evidence", () => {
  const result = validateLivePenpotScenes(live, original);
  assert.equal(result.ok, true, result.errors.join("; "));
  assert.equal(result.coordinateAudited, result.scenes);
  assert.ok(result.coordinateAudited >= 36);
});

test("a clipped 1280px desktop floating window is not accepted", () => {
  const broken = clone(live);
  const frame = broken.coordinateAudit.scenes.find(
    (s) => s.kind === "desktop" && s.windowId,
  );
  assert.ok(frame);
  frame.x = 840;
  frame.right = frame.x + frame.windowWidth;
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("a mobile floating window must retain its 16px minimum horizontal margin", () => {
  const broken = clone(live);
  const frame = broken.coordinateAudit.scenes.find(
    (s) => s.kind === "mobile" && s.windowId,
  );
  assert.ok(frame);
  frame.x = 20;
  frame.right = frame.x + frame.windowWidth;
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("viewport evidence cannot invent an unknown Penpot scene", () => {
  const broken = clone(live);
  broken.coordinateAudit.scenes[0].id = "00000000-0000-0000-0000-000000000001";
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("Viewer click trace must cite the exact Penpot hotspot shape ID", () => {
  const broken = clone(live);
  broken.viewerQa[0].sourceId = "00000000-0000-0000-0000-000000000002";
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("Viewer click sequence may not claim a skipped transition", () => {
  const broken = clone(live);
  const index = broken.viewerQa.findIndex((step) => step.step === 2);
  assert.ok(index > 0);
  broken.viewerQa[index].step = 3;
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("Viewer test-case summary must agree with the exact observed step count", () => {
  const broken = clone(live);
  broken.viewerEvidence.testCases[0].clickedSteps += 1;
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});

test("Flow starting points must match the actual saved Penpot startingBoard names", () => {
  const broken = clone(live);
  broken.flowStarts[0].startName = "V2 / Scene / Invented scene";
  assert.equal(validateLivePenpotScenes(broken, original).ok, false);
});
