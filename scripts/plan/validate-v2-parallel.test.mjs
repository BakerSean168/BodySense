import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import {
  REPO_ROOT,
  validateHandoff,
  validateRepository,
} from "./validate-v2-parallel.mjs";

const state = JSON.parse(
  fs.readFileSync(
    path.join(REPO_ROOT, "docs/plan/active/v2-handoff-state.json"),
    "utf8",
  ),
);
const fresh = () => structuredClone(state);

test("V2 foundation plans and five domain gates are connected to actual files", () => {
  assert.deepEqual(validateRepository(), []);
});

test("a domain cannot be integrated before all prerequisite gates", () => {
  const mutation = fresh();
  // Independent of the live state: force one prerequisite gate open.
  mutation.domains[0].gates.design = {
    state: "in_progress",
    next: "PD-TEST-01",
    details: "Design acceptance still open.",
  };
  mutation.domains[0].gates.integration = {
    state: "ready",
    proof: ["docs/product/acceptance.md"],
  };
  assert.ok(
    validateHandoff(mutation).some((error) =>
      error.includes("integration signed off before"),
    ),
  );
});

test("a domain cannot sign off engineering on only an additive migration", () => {
  const mutation = fresh();
  // Independent of the live state: the database gate is not signed off yet.
  mutation.domains[0].gates.database = {
    state: "in_progress",
    next: "DB-TEST-01",
    details: "Only an additive migration exists.",
  };
  mutation.domains[0].gates.engineering = {
    state: "ready",
    proof: ["apps/api/migrations/000002_body_fact_ledger.up.sql"],
  };
  assert.ok(
    validateHandoff(mutation).some((error) =>
      error.includes("engineering signed off before"),
    ),
  );
});

test("missing evidence, incorrect paths and invalid gate status fail closed", () => {
  const mutation = fresh();
  mutation.domains[0].gates.business = { state: "ready", proof: [] };
  mutation.domains[1].product = ["../outside.md"];
  mutation.domains[2].gates.view_model = { state: "deployed" };
  const errors = validateHandoff(mutation);
  assert.ok(
    errors.some((error) => error.includes("ready without artifact proof")),
  );
  assert.ok(
    errors.some((error) => error.includes("invalid relative repository path")),
  );
  assert.ok(
    errors.some((error) => error.includes("invalid/missing gate state")),
  );
});

test("duplicate domains and incomplete workstream maps fail closed", () => {
  const mutation = fresh();
  mutation.domains[1].id = mutation.domains[0].id;
  delete mutation.tracks.product_design;
  const errors = validateHandoff(mutation);
  assert.ok(errors.some((error) => error.includes("duplicate/missing ID")));
  assert.ok(errors.some((error) => error.includes("exactly four workstreams")));
});
