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
