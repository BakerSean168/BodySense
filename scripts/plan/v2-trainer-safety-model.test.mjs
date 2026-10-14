// Executable DS-TP-01 reference contract. This is NOT the Go SafetyState implementation
// or a verified clinical policy. Evidence and classification enter as already-authorized data.
// Source of truth: docs/product/domain/safety.md (SAFE-D1..D8).
import assert from "node:assert/strict";
import { test } from "node:test";

const LEVEL = { advisory: 1, restricted: 2, urgent: 3 };

function reviewGate({ snapshot, planId, exerciseId, now }) {
  if (
    !snapshot ||
    snapshot.available !== true ||
    !Array.isArray(snapshot.restrictions) ||
    typeof now !== "string" ||
    Number.isNaN(Date.parse(now)) ||
    typeof planId !== "string" ||
    planId.length === 0
  ) {
    return { result: "block", matchedRestrictionIds: [], requiresReview: true };
  }
  let highest = 0;
  const matched = [];
  for (const restriction of snapshot.restrictions) {
    if (
      !restriction ||
      !restriction.restrictionId ||
      !LEVEL[restriction.riskLevel] ||
      !restriction.scope ||
      !restriction.validFrom ||
      Number.isNaN(Date.parse(restriction.validFrom)) ||
      !restriction.liftPolicy
    ) {
      return {
        result: "block",
        matchedRestrictionIds: matched,
        requiresReview: true,
      };
    }
    if (new Date(restriction.validFrom) > new Date(now)) continue;
    if (restriction.liftedAt && restriction.liftBasis) continue; // Go must verify receipt.
    // No implicit auto-lift from validUntil. On-expiry is an audited command/event.
    const scope = restriction.scope;
    let applies;
    if (restriction.riskLevel === "urgent" || scope.kind === "all_training")
      applies = true;
    else if (scope.kind === "plan") applies = scope.planId === planId;
    else if (scope.kind === "exercise") {
      if (scope.planId && scope.planId !== planId) applies = false;
      else if (
        !Array.isArray(scope.exerciseIds) ||
        !scope.exerciseIds.length ||
        !exerciseId
      )
        return {
          result: "block",
          matchedRestrictionIds: matched,
          requiresReview: true,
        };
      else applies = scope.exerciseIds.includes(exerciseId);
    } else
      return {
        result: "block",
        matchedRestrictionIds: matched,
        requiresReview: true,
      };
    if (applies) {
      matched.push(restriction.restrictionId);
      highest = Math.max(highest, LEVEL[restriction.riskLevel]);
    }
  }
  return {
    result: highest >= 2 ? "block" : highest === 1 ? "warn" : "allow",
    matchedRestrictionIds: matched,
    requiresReview: false,
  };
}

// Pure predicate for a proposed exact-ID lift. Real evidence admission, policy
// provenance, tenant authorization, CAS and commit receipt are owned by Go.
function authorizeLift({
  snapshot,
  restrictionId,
  expectedRevision,
  basis,
  now,
  verifiedRule,
}) {
  if (
    !snapshot?.available ||
    !Number.isSafeInteger(expectedRevision) ||
    snapshot.revision !== expectedRevision ||
    !restrictionId ||
    !basis ||
    !Number.isFinite(Date.parse(now))
  )
    return false;
  const target = snapshot.restrictions?.find(
    (r) => r.restrictionId === restrictionId,
  );
  if (
    !target ||
    target.liftedAt ||
    target.liftBasis ||
    basis.kind !== target.liftPolicy.replace("on_expiry", "expiry")
  )
    return false;
  if (basis.kind === "expiry") {
    return (
      !!verifiedRule &&
      !!target.validUntil &&
      Number.isFinite(Date.parse(target.validUntil)) &&
      Date.parse(now) >= Date.parse(target.validUntil)
    );
  }
  if (basis.kind === "user_reverification") {
    return (
      typeof basis.checkInteractionId === "string" &&
      !!basis.checkInteractionId &&
      basis.conditionsVerified === true
    );
  }
  if (basis.kind === "professional_clearance") {
    return (
      !!basis.sourceId &&
      ["user_message", "clinical_document"].includes(basis.sourceKind) &&
      basis.admitted === true &&
      basis.appliesToRestriction === true
    );
  }
  return false;
}

const at = "2026-10-14T00:00:00Z";
const earlier = "2026-10-13T00:00:00Z";
const restriction = (
  id,
  riskLevel,
  scope = { kind: "all_training" },
  more = {},
) => ({
  restrictionId: id,
  riskLevel,
  scope,
  validFrom: earlier,
  liftPolicy: "user_reverification",
  ...more,
});
const state = (...restrictions) => ({
  available: true,
  revision: 3,
  restrictions,
});
const gate = (snapshot, extra = {}) =>
  reviewGate({
    snapshot,
    planId: "plan-A",
    exerciseId: "standard-squat",
    now: at,
    ...extra,
  });

test("SAFE-04: overlapping IDs remain visible; strongest restriction blocks", () => {
  const result = gate(
    state(restriction("a", "advisory"), restriction("r", "restricted")),
  );
  assert.deepEqual(result, {
    result: "block",
    matchedRestrictionIds: ["a", "r"],
    requiresReview: false,
  });
});
test("SAFE-05: urgent applies globally regardless of a narrower scope", () => {
  const result = gate(
    state(restriction("u", "urgent", { kind: "plan", planId: "different" })),
  );
  assert.equal(result.result, "block");
  assert.deepEqual(result.matchedRestrictionIds, ["u"]);
});
test("SAFE-04: advisory alone warns without claiming block", () => {
  assert.equal(gate(state(restriction("a", "advisory"))).result, "warn");
});
test("SAFE-D2: exercise concept is not the prescription line or display name", () => {
  const s = state(
    restriction("r", "restricted", {
      kind: "exercise",
      exerciseIds: ["standard-squat"],
    }),
  );
  assert.equal(gate(s, { exerciseId: "other-exercise" }).result, "allow");
  assert.equal(gate(s, { exerciseId: "standard-squat" }).result, "block");
});
test("SAFE-10: unavailable SafetyState or ambiguous exercise match fails closed", () => {
  assert.equal(gate({ available: false, restrictions: [] }).result, "block");
  assert.equal(
    gate(
      state(
        restriction("r", "restricted", {
          kind: "exercise",
          exerciseIds: ["standard-squat"],
        }),
      ),
      { exerciseId: undefined },
    ).result,
    "block",
  );
});
test("SAFE-02: elapsed review point never auto-lifts non-expiry restriction", () => {
  const s = state(
    restriction("r", "restricted", undefined, { validUntil: earlier }),
  );
  assert.equal(gate(s).result, "block");
});
test("SAFE-08: only an exact expected revision can authorize a lift", () => {
  const s = state(restriction("r", "restricted"));
  assert.equal(
    authorizeLift({
      snapshot: s,
      restrictionId: "r",
      expectedRevision: 2,
      basis: {
        kind: "user_reverification",
        checkInteractionId: "i",
        conditionsVerified: true,
      },
      now: at,
    }),
    false,
  );
});
test("SAFE-D5: user re-verification must have an interaction and verified conditions", () => {
  const s = state(restriction("r", "restricted"));
  const input = {
    snapshot: s,
    restrictionId: "r",
    expectedRevision: 3,
    now: at,
  };
  assert.equal(
    authorizeLift({
      ...input,
      basis: { kind: "user_reverification", checkInteractionId: "i" },
    }),
    false,
  );
  assert.equal(
    authorizeLift({
      ...input,
      basis: {
        kind: "user_reverification",
        checkInteractionId: "i",
        conditionsVerified: true,
      },
    }),
    true,
  );
});
test("SAFE-01/09: OCR candidate and unsourced professional clearance cannot lift", () => {
  const s = state(
    restriction("r", "restricted", undefined, {
      liftPolicy: "professional_clearance",
    }),
  );
  const input = {
    snapshot: s,
    restrictionId: "r",
    expectedRevision: 3,
    now: at,
  };
  assert.equal(
    authorizeLift({
      ...input,
      basis: {
        kind: "professional_clearance",
        sourceId: "ocr",
        sourceKind: "clinical_document",
      },
    }),
    false,
  );
  assert.equal(
    authorizeLift({
      ...input,
      basis: {
        kind: "professional_clearance",
        sourceId: "src",
        sourceKind: "user_message",
        admitted: true,
        appliesToRestriction: true,
      },
    }),
    true,
  );
});
test("TP-06c/SAFE-D5: audited expiry requires verified rule, exact deadline and restriction", () => {
  const s = state(
    restriction("r", "restricted", undefined, {
      liftPolicy: "on_expiry",
      validUntil: earlier,
    }),
  );
  const input = {
    snapshot: s,
    restrictionId: "r",
    expectedRevision: 3,
    basis: { kind: "expiry" },
    now: at,
  };
  assert.equal(authorizeLift(input), false);
  assert.equal(authorizeLift({ ...input, verifiedRule: true }), true);
  assert.equal(
    authorizeLift({ ...input, verifiedRule: true, restrictionId: "other" }),
    false,
  );
  assert.equal(gate(s).result, "block"); // until Go has durably committed the lift event.
});
test("SAFE-05: lifting one restriction never removes another active blocker", () => {
  const s = state(
    restriction("a", "advisory", undefined, {
      liftedAt: at,
      liftBasis: { kind: "user_reverification", checkInteractionId: "i" },
    }),
    restriction("r", "restricted"),
  );
  assert.deepEqual(gate(s).matchedRestrictionIds, ["r"]);
});
test("SAFE-D5: inconsistent, absent or invalid snapshots never declare allow", () => {
  assert.equal(gate(undefined).result, "block");
  assert.equal(
    gate(
      state({
        restrictionId: "x",
        riskLevel: "unknown",
        scope: { kind: "all_training" },
        validFrom: earlier,
        liftPolicy: "user_reverification",
      }),
    ).result,
    "block",
  );
});
