import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import {
  evaluateV2StagingReadiness,
  REQUIRED_DOMAINS,
  REQUIRED_FOUNDATIONS,
  REQUIRED_GATES,
} from "./v2-staging-readiness.mjs";

const current = JSON.parse(
  fs.readFileSync("docs/plan/active/v2-handoff-state.json", "utf8"),
);
function accepted() {
  const v2 = structuredClone(current);
  v2.domains = REQUIRED_DOMAINS.map((id) => ({
    id,
    gates: Object.fromEntries(
      REQUIRED_GATES.map((gate) => [
        gate,
        { state: "ready", proof: [`evidence/${id}/${gate}`] },
      ]),
    ),
  }));
  v2.foundations = REQUIRED_FOUNDATIONS.map((id) => ({
    id,
    state: "ready",
    proof: [`evidence/${id}`],
  }));
  return v2;
}
const evaluate = (enabled, handoff) =>
  evaluateV2StagingReadiness({ enabled, handoff });

test("current unapproved V2 must never promote staging", () => {
  assert.equal(evaluate("true", current).eligible, false);
  assert.equal(evaluate(undefined, current).eligible, false);
});
test("operator variable is false by default even if all gates are approved", () => {
  assert.equal(evaluate(undefined, accepted()).eligible, false);
  assert.equal(evaluate("false", accepted()).eligible, false);
  assert.equal(evaluate("TRUE", accepted()).eligible, false);
});
test("full formal V2 gates plus explicit operator opt-in allow staging promotion", () => {
  assert.deepEqual(evaluate("true", accepted()), {
    eligible: true,
    blockers: [],
  });
});
test("one pending clinical integration gate blocks staging", () => {
  const v2 = accepted();
  v2.domains[3].gates.integration.state = "in_progress";
  assert.equal(evaluate("true", v2).eligible, false);
});
test("a ready gate without evidence blocks staging", () => {
  const v2 = accepted();
  v2.domains[2].gates.database.proof = [];
  assert.equal(evaluate("true", v2).eligible, false);
});
test("missing domains, duplicate domains, and unapproved foundations block staging", () => {
  const missing = accepted();
  missing.domains.pop();
  assert.equal(evaluate("true", missing).eligible, false);
  const duplicate = accepted();
  duplicate.domains[3] = duplicate.domains[2];
  assert.equal(evaluate("true", duplicate).eligible, false);
  const foundation = accepted();
  foundation.foundations[2].state = "in_progress";
  assert.equal(evaluate("true", foundation).eligible, false);
});
function penpotPending() {
  const v2 = accepted();
  const penpot = v2.foundations.find((entry) => entry.id === "F-PENPOT");
  penpot.state = "blocked";
  penpot.proof = [];
  return v2;
}
test("visual sign-off on staging waives only a blocked F-PENPOT", () => {
  assert.equal(evaluate("true", penpotPending()).eligible, false);
  const waived = evaluateV2StagingReadiness({
    enabled: "true",
    handoff: penpotPending(),
    visualSignoffOnStaging: "true",
  });
  assert.equal(waived.eligible, true);
  assert.deepEqual(waived.blockers, []);
  assert.equal(waived.waived.length, 1);
  assert.match(waived.waived[0], /^F-PENPOT:/);
});
test("visual sign-off waiver never replaces the operator opt-in", () => {
  const result = evaluateV2StagingReadiness({
    enabled: undefined,
    handoff: penpotPending(),
    visualSignoffOnStaging: "true",
  });
  assert.equal(result.eligible, false);
});
test("visual sign-off waiver does not cover other foundations or gates", () => {
  const foundation = penpotPending();
  foundation.foundations.find((e) => e.id === "F-CONTRACT").state = "blocked";
  const gate = penpotPending();
  gate.domains[0].gates.design.state = "blocked";
  const missing = penpotPending();
  missing.foundations = missing.foundations.filter((e) => e.id !== "F-PENPOT");
  const queued = penpotPending();
  queued.foundations.find((e) => e.id === "F-PENPOT").state = "queued";
  for (const handoff of [foundation, gate, missing, queued]) {
    const result = evaluateV2StagingReadiness({
      enabled: "true",
      handoff,
      visualSignoffOnStaging: "true",
    });
    assert.equal(result.eligible, false);
  }
});
test("invalid handoff fails closed", () => {
  assert.equal(evaluate("true", null).eligible, false);
  assert.equal(evaluate("true", { version: 2 }).eligible, false);
});
test("candidate workflow gates only mutable staging promotion", () => {
  const workflow = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  assert.match(workflow, /staging-readiness:\n/);
  assert.match(
    workflow,
    /node scripts\/delivery\/v2-staging-readiness\.mjs --github-output/,
  );
  assert.match(
    workflow,
    /promote-staging:[\s\S]*?needs: \[resolve, candidate-manifest, staging-readiness\]/,
  );
  assert.match(
    workflow,
    /needs\.staging-readiness\.outputs\.eligible == 'true'/,
  );
  assert.match(workflow, /needs\.resolve\.outputs\.is_current_main == 'true'/);
  assert.match(workflow, /name: Build canonical candidate manifest/);
  assert.match(
    workflow,
    /BODYSENSE_V2_STAGING_VISUAL_SIGNOFF_ON_STAGING: \$\{\{ vars\.BODYSENSE_V2_STAGING_VISUAL_SIGNOFF_ON_STAGING \}\}/,
  );
});
