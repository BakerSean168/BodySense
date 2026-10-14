import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { validatePenpotHandoff } from "./validate-v2-penpot-handoff.mjs";

const manifest = JSON.parse(
  readFileSync(
    new URL("../../docs/design/v2/penpot-scene-manifest.json", import.meta.url),
    "utf8",
  ),
);
const spec = JSON.parse(
  readFileSync(
    new URL("../../docs/design/v2/penpot-flow-contract.json", import.meta.url),
    "utf8",
  ),
);
const clone = (x) => JSON.parse(JSON.stringify(x));

test("design contract aligns six named flows with 42 persisted editable frames", () => {
  const r = validatePenpotHandoff(manifest, spec);
  assert.equal(r.ok, true, r.errors.join("; "));
  assert.equal(r.scenes, 42);
  assert.equal(r.components, 14);
  assert.equal(r.flows, 6);
  assert.equal(r.transitions, 37);
});

test("a transition cannot target an invented Penpot frame", () => {
  const invalid = clone(spec);
  invalid.flows[0].steps[0].to = "Desktop / Diagnosis / Made Up";
  assert.equal(validatePenpotHandoff(manifest, invalid).ok, false);
});

test("a medical receipt cannot be reached by an unguarded local click", () => {
  const invalid = clone(spec);
  const step = invalid.flows
    .find((f) => f.id === "PD-DX-01")
    .steps.find((s) => s.to.endsWith("Receipt"));
  step.kind = "local";
  delete step.requires;
  assert.equal(validatePenpotHandoff(manifest, invalid).ok, false);
});

test("erasure completion must require backend verification evidence", () => {
  const invalid = clone(spec);
  const step = invalid.flows
    .find((f) => f.id === "PD-ASSET-01")
    .steps.find((s) => s.to.endsWith("Erasure completed"));
  delete step.requires;
  assert.equal(validatePenpotHandoff(manifest, invalid).ok, false);
});

test("writer cannot pretend review has been signed off", () => {
  const invalid = clone(spec);
  invalid.status = "READY";
  assert.equal(validatePenpotHandoff(manifest, invalid).ok, false);
});

test("duplicate scene identifiers cannot masquerade as distinct states", () => {
  const invalid = clone(manifest);
  invalid.scenes[1].id = invalid.scenes[0].id;
  assert.equal(validatePenpotHandoff(invalid, spec).ok, false);
});

test("overlay test evidence is a prerequisite, not complete business-flow evidence", () => {
  const invalid = clone(manifest);
  invalid.overlayQa.nextActions = ["navigate-to"];
  assert.equal(validatePenpotHandoff(invalid, spec).ok, false);
});
