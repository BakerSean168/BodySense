import test from "node:test";
import assert from "node:assert/strict";
import {
  assertCursorState,
  advanceRetentionCursor,
} from "./v2-chat-retention-model.mjs";

const initial = () => ({
  processedThroughSeq: 0,
  contextStartSeq: 1,
  prunedThroughSeq: 0,
  nextSeq: 7,
});
const process = (to = 6) => ({
  kind: "process",
  to,
  completeTurnEnds: [2, 4, 6],
  batches: [
    {
      endSeq: 2,
      disposition: "durable_source",
      domainReceipt: "body-state-revision-1",
    },
    { endSeq: 4, disposition: "no_long_term_value" },
    {
      endSeq: 6,
      disposition: "durable_source",
      domainReceipt: "body-state-revision-2",
    },
  ].filter((batch) => batch.endSeq <= to),
});
const cleanupProof = (to = 4) => ({
  inventoryComplete: true,
  writesFenced: true,
  sourceRefsRelinked: true,
  durableEvidenceVerified: true,
  clearedThroughSeq: to,
  requiredSurfaces: ["messages", "checkpoints", "cache", "search_index"],
  verifiedSurfaces: ["messages", "checkpoints", "cache", "search_index"].map(
    (surface) => ({
      surface,
      verified: true,
      clearedThroughSeq: to,
      receipt: `synthetic-${surface}-receipt`,
    }),
  ),
});
const prune = (to = 4) => ({
  kind: "prune",
  to,
  completeTurnEnds: [2, 4, 6],
  activeRun: false,
  pendingInteraction: false,
  unsettledProposal: false,
  liveCheckpoint: false,
  cleanupProof: cleanupProof(to),
});
const prepared = () => {
  const processed = advanceRetentionCursor(initial(), process());
  return advanceRetentionCursor(processed, {
    kind: "context",
    to: 5,
    completeTurnEnds: [4],
  });
};
const fails = (fn, code) => assert.throws(fn, (error) => error?.code === code);

test("empty/new conversation has stable initial cursor invariants", () => {
  assert.equal(assertCursorState(initial()), true);
  fails(
    () => assertCursorState({ ...initial(), contextStartSeq: 0 }),
    "cursor_shape",
  );
  fails(
    () => assertCursorState({ ...initial(), prunedThroughSeq: 1 }),
    "cursor_order",
  );
  fails(
    () => assertCursorState({ ...initial(), processedThroughSeq: 7 }),
    "cursor_order",
  );
  fails(
    () => assertCursorState({ ...initial(), contextStartSeq: 8 }),
    "cursor_order",
  );
});

test("process completed interactions only after durable source receipt or explicit no-value outcome", () => {
  assert.deepEqual(advanceRetentionCursor(initial(), process(4)), {
    ...initial(),
    processedThroughSeq: 4,
  });
  assert.equal(prepared().processedThroughSeq, 6);
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        ...process(4),
        completeTurnEnds: [2],
      }),
    "incomplete_interaction",
  );
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        ...process(4),
        batches: [{ endSeq: 2, disposition: "no_long_term_value" }],
      }),
    "processing_gap",
  );
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        ...process(2),
        batches: [{ endSeq: 2, disposition: "durable_source" }],
      }),
    "missing_domain_receipt",
  );
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        ...process(2),
        batches: [{ endSeq: 2, disposition: "pending" }],
      }),
    "processing_disposition",
  );
  fails(
    () =>
      advanceRetentionCursor(initial(), { ...process(2), protectedFromSeq: 2 }),
    "protected_interaction",
  );
});

test("context movement is independent of processing and never splits an interaction", () => {
  const next = advanceRetentionCursor(initial(), {
    kind: "context",
    to: 3,
    completeTurnEnds: [2],
  });
  assert.equal(next.contextStartSeq, 3);
  assert.equal(next.processedThroughSeq, 0);
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        kind: "context",
        to: 4,
        completeTurnEnds: [2, 4],
      }),
    "incomplete_interaction",
  );
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        kind: "context",
        to: 9,
        completeTurnEnds: [8],
      }),
    "uncommitted_message",
  );
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        kind: "context",
        to: 5,
        completeTurnEnds: [4],
        protectedFromSeq: 4,
      }),
    "protected_interaction",
  );
});

test("prune uses complete boundaries and must be below processed and context", () => {
  const current = prepared();
  assert.deepEqual(advanceRetentionCursor(current, prune()), {
    ...current,
    prunedThroughSeq: 4,
  });
  fails(
    () => advanceRetentionCursor(current, prune(6)),
    "unsafe_prune_boundary",
  );
  fails(
    () =>
      advanceRetentionCursor({ ...current, processedThroughSeq: 2 }, prune(4)),
    "unsafe_prune_boundary",
  );
  fails(
    () =>
      advanceRetentionCursor(current, { ...prune(4), completeTurnEnds: [2] }),
    "incomplete_interaction",
  );
  fails(
    () => advanceRetentionCursor(current, { ...prune(4), protectedFromSeq: 3 }),
    "protected_interaction",
  );
});

test("prune fails closed for all four kinds of active runtime dependency", () => {
  for (const type of [
    "activeRun",
    "pendingInteraction",
    "unsettledProposal",
    "liveCheckpoint",
  ]) {
    fails(
      () => advanceRetentionCursor(prepared(), { ...prune(), [type]: true }),
      "active_dependency",
    );
  }
});

test("cleanup cannot pass with partial cleanup or incomplete controlled-copy enumeration", () => {
  const current = prepared();
  for (const missing of [
    "inventoryComplete",
    "writesFenced",
    "sourceRefsRelinked",
    "durableEvidenceVerified",
    "clearedThroughSeq",
  ]) {
    const proof = cleanupProof();
    delete proof[missing];
    fails(
      () =>
        advanceRetentionCursor(current, { ...prune(), cleanupProof: proof }),
      "cleanup_unproven",
    );
  }
  const missingReceipt = cleanupProof();
  missingReceipt.verifiedSurfaces[1].receipt = "";
  fails(
    () =>
      advanceRetentionCursor(current, {
        ...prune(),
        cleanupProof: missingReceipt,
      }),
    "copy_not_verified",
  );

  const incomplete = cleanupProof();
  incomplete.verifiedSurfaces.pop();
  fails(
    () =>
      advanceRetentionCursor(current, { ...prune(), cleanupProof: incomplete }),
    "copy_not_verified",
  );

  const duplicate = cleanupProof();
  duplicate.requiredSurfaces.push("messages");
  fails(
    () =>
      advanceRetentionCursor(current, { ...prune(), cleanupProof: duplicate }),
    "copy_inventory",
  );

  const noMessages = cleanupProof();
  noMessages.requiredSurfaces = ["cache"];
  noMessages.verifiedSurfaces = noMessages.verifiedSurfaces.filter(
    (v) => v.surface === "cache",
  );
  fails(
    () =>
      advanceRetentionCursor(current, { ...prune(), cleanupProof: noMessages }),
    "copy_inventory",
  );

  const incompleteCoverage = cleanupProof();
  incompleteCoverage.verifiedSurfaces[2].clearedThroughSeq = 2;
  fails(
    () =>
      advanceRetentionCursor(current, {
        ...prune(),
        cleanupProof: incompleteCoverage,
      }),
    "copy_not_verified",
  );
});

test("stale/replayed cursor changes do not mint a new transition", () => {
  const state = advanceRetentionCursor(prepared(), prune());
  fails(() => advanceRetentionCursor(state, prune(4)), "cursor_noop");
  fails(() => advanceRetentionCursor(state, prune(2)), "cursor_regression");
  fails(
    () =>
      advanceRetentionCursor(state, {
        kind: "context",
        to: 4,
        completeTurnEnds: [3],
      }),
    "cursor_regression",
  );
  fails(
    () => advanceRetentionCursor(state, { ...process(6), to: 6 }),
    "cursor_noop",
  );
  fails(
    () => advanceRetentionCursor(state, { kind: "delete" }),
    "transition_shape",
  );
  fails(
    () => advanceRetentionCursor(state, { kind: "erase", to: 1 }),
    "unknown_transition",
  );
});

test("original cursor object and message sequence are never mutated or renumbered", () => {
  const state = Object.freeze(initial());
  const next = advanceRetentionCursor(state, process(2));
  assert.equal(state.processedThroughSeq, 0);
  assert.equal(next.nextSeq, 7);
  assert.equal(next.prunedThroughSeq, 0);
  assert.notEqual(next, state);
});

test("unsafe malformed boundaries, fences and unsafely large integers are rejected", () => {
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        ...process(2),
        completeTurnEnds: [2, 2],
      }),
    "incomplete_interaction",
  );
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        ...process(2),
        protectedFromSeq: -1,
      }),
    "fence_shape",
  );
  fails(
    () =>
      advanceRetentionCursor(initial(), {
        ...process(2),
        to: Number.MAX_SAFE_INTEGER + 1,
      }),
    "transition_shape",
  );
});
