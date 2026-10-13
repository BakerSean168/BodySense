// Red tests: these safety gates must pass before adopting the specification oracle.
// Kept separate from the initial green cases so the incomplete implementation is not misreported.
import test from "node:test";
import assert from "node:assert/strict";
import { advanceRetentionCursor } from "./v2-chat-retention-model.mjs";

const state = {
  processedThroughSeq: 4,
  contextStartSeq: 5,
  prunedThroughSeq: 0,
  nextSeq: 7,
};
const cleanupProof = {
  inventoryComplete: true,
  writesFenced: true,
  sourceRefsRelinked: true,
  durableEvidenceVerified: true,
  clearedThroughSeq: 4,
  requiredSurfaces: ["messages"],
  verifiedSurfaces: [
    {
      surface: "messages",
      verified: true,
      clearedThroughSeq: 4,
      receipt: "synthetic",
    },
  ],
};

test("physical pruning fails closed if any active-dependency flag is UNKNOWN", () => {
  const request = { kind: "prune", to: 4, completeTurnEnds: [4], cleanupProof };
  for (const missing of [
    "activeRun",
    "pendingInteraction",
    "unsettledProposal",
    "liveCheckpoint",
  ]) {
    const explicit = {
      ...request,
      activeRun: false,
      pendingInteraction: false,
      unsettledProposal: false,
      liveCheckpoint: false,
    };
    delete explicit[missing];
    assert.throws(
      () => advanceRetentionCursor(state, explicit),
      (error) => error?.code === "active_dependency",
      `missing ${missing} must NOT silently mean false`,
    );
  }
});

test("every completed turn in a processing range requires its own attested disposition", () => {
  const request = {
    kind: "process",
    to: 4,
    completeTurnEnds: [2, 4],
    batches: [{ endSeq: 4, disposition: "no_long_term_value" }],
  };
  assert.throws(
    () => advanceRetentionCursor({ ...state, processedThroughSeq: 0 }, request),
    (error) => error?.code === "processing_gap",
    "the first completed turn ending at 2 has no disposition",
  );
});

test("prune requires every dependency to be literally closed, never coercible false", () => {
  const closed = {
    kind: "prune",
    to: 4,
    completeTurnEnds: [4],
    cleanupProof,
    activeRun: false,
    pendingInteraction: false,
    unsettledProposal: false,
    liveCheckpoint: false,
  };
  for (const flag of [
    "activeRun",
    "pendingInteraction",
    "unsettledProposal",
    "liveCheckpoint",
  ]) {
    for (const value of [undefined, null, 0, "false", {}, true]) {
      assert.throws(
        () => advanceRetentionCursor(state, { ...closed, [flag]: value }),
        (error) => error?.code === "active_dependency",
        `${flag}=${String(value)} must not authorize physical cleanup`,
      );
    }
  }
  assert.equal(advanceRetentionCursor(state, closed).prunedThroughSeq, 4);
});

test("processing from a prior cursor cannot skip, duplicate or reorder complete turns", () => {
  const current = { ...state, processedThroughSeq: 2 };
  const valid = {
    kind: "process",
    to: 6,
    completeTurnEnds: [2, 4, 6],
    batches: [
      { endSeq: 4, disposition: "no_long_term_value" },
      { endSeq: 6, disposition: "no_long_term_value" },
    ],
  };
  assert.equal(advanceRetentionCursor(current, valid).processedThroughSeq, 6);
  for (const batches of [
    [valid.batches[1]],
    [valid.batches[0], valid.batches[0]],
    [valid.batches[1], valid.batches[0]],
  ]) {
    assert.throws(
      () => advanceRetentionCursor(current, { ...valid, batches }),
      (error) => error?.code === "processing_gap",
    );
  }
});
