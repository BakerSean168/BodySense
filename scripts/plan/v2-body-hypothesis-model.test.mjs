// DS-BS-02 executable reference for user-owned BodyHypothesis and read-only
// ConcernGroup projections. Not Go/SQL/clinical evidence or deployment proof.
// Authorities: docs/product/domain/bodystate.md BS-D9 and acceptance BS-04,
// BS-04b, BS-14, BS-15, BS-28, and INV-02/25/33.
import assert from "node:assert/strict";
import { test } from "node:test";

const clone = (value) => structuredClone(value);
const nonEmpty = (value) =>
  typeof value === "string" && value.trim().length > 0;
const allowedSourceKinds = new Set([
  "user_message",
  "direct_edit",
  "structured_answer",
]);
const fixture = () => ({
  revision: 1,
  hypotheses: [],
  facts: [
    {
      id: "fact-1",
      userId: "owner",
      statement: "head hurts",
      assertionStatus: "asserted",
    },
  ],
  observations: [
    {
      id: "obs-1",
      userId: "owner",
      rawValue: "67",
      assertionStatus: "asserted",
    },
  ],
  diagnosis: [],
  safetyRestrictions: [{ restrictionId: "risk-1", riskLevel: "restricted" }],
  sources: {
    "msg-1": {
      userId: "owner",
      actor: "user",
      kind: "user_message",
      available: true,
      admitted: true,
    },
    "msg-2": {
      userId: "owner",
      actor: "user",
      kind: "user_message",
      available: true,
      admitted: true,
    },
    "direct-1": {
      userId: "owner",
      actor: "user",
      kind: "direct_edit",
      available: true,
      admitted: true,
    },
    foreign: {
      userId: "other",
      actor: "user",
      kind: "user_message",
      available: true,
      admitted: true,
    },
    "agent-1": {
      userId: "owner",
      actor: "agent",
      kind: "user_message",
      available: true,
      admitted: true,
    },
    unavailable: {
      userId: "owner",
      actor: "user",
      kind: "user_message",
      available: false,
      admitted: true,
    },
  },
  replay: {},
});

function ownsReferences(state, userId, factIds, observationIds) {
  return (
    Array.isArray(factIds) &&
    Array.isArray(observationIds) &&
    factIds.every((id) =>
      state.facts.some((item) => item.id === id && item.userId === userId),
    ) &&
    observationIds.every((id) =>
      state.observations.some(
        (item) => item.id === id && item.userId === userId,
      ),
    )
  );
}

function sourceIsAdmissible(state, userId, command) {
  const source = state.sources[command.sourceId];
  return (
    command.actor === "user" &&
    source?.userId === userId &&
    source.actor === "user" &&
    source.kind === command.sourceKind &&
    allowedSourceKinds.has(command.sourceKind) &&
    source.available === true &&
    source.admitted === true
  );
}

function issue(state, command, body) {
  const key = `${command.userId}/${command.idempotencyKey}`;
  const signature = JSON.stringify(body);
  const previous = state.replay[key];
  if (previous) {
    return previous.signature === signature
      ? { outcome: "replayed", state, receipt: previous.receipt }
      : { outcome: "rejected", state };
  }
  return { signature, key };
}

function executeRecordHypothesis(state, command, authenticatedUserId) {
  const body = {
    operation: "record",
    userId: command.userId,
    actor: command.actor,
    sourceId: command.sourceId,
    sourceKind: command.sourceKind,
    statement: command.statement,
    relatedFactIds: command.relatedFactIds,
    relatedObservationIds: command.relatedObservationIds,
    expectedRevision: command.expectedRevision,
  };
  // Server-sourced identity must match command target before checking replay receipts.
  if (
    !nonEmpty(authenticatedUserId) ||
    authenticatedUserId !== command.userId ||
    !nonEmpty(command.idempotencyKey) ||
    !nonEmpty(command.userId)
  )
    return { outcome: "rejected", state };
  const replay = issue(state, command, body);
  if (replay.outcome) return replay;
  if (
    !sourceIsAdmissible(state, command.userId, command) ||
    !nonEmpty(command.statement) ||
    state.revision !== command.expectedRevision ||
    !ownsReferences(
      state,
      command.userId,
      command.relatedFactIds,
      command.relatedObservationIds,
    )
  )
    return { outcome: "rejected", state };
  const next = clone(state);
  const id = `hyp-${next.revision + 1}`;
  next.hypotheses.push({
    id,
    userId: command.userId,
    statement: command.statement,
    source: {
      actor: "user",
      reference: { kind: command.sourceKind, id: command.sourceId },
    },
    relatedFactIds: [...command.relatedFactIds],
    relatedObservationIds: [...command.relatedObservationIds],
    assertionStatus: "asserted",
  });
  next.revision += 1;
  const receipt = { id, revision: next.revision };
  next.replay[replay.key] = { signature: replay.signature, receipt };
  return { outcome: "recorded", state: next, receipt };
}

function executeAmendHypothesis(state, command, authenticatedUserId) {
  const body = {
    operation: command.operation,
    userId: command.userId,
    actor: command.actor,
    sourceId: command.sourceId,
    sourceKind: command.sourceKind,
    targetId: command.targetId,
    statement: command.statement ?? null,
    expectedRevision: command.expectedRevision,
  };
  // A forged tenant cannot replay another tenant's previous amendment receipt.
  if (
    !nonEmpty(authenticatedUserId) ||
    authenticatedUserId !== command.userId ||
    !nonEmpty(command.idempotencyKey) ||
    !nonEmpty(command.userId)
  )
    return { outcome: "rejected", state };
  const replay = issue(state, command, body);
  if (replay.outcome) return replay;
  const target = state.hypotheses.find((item) => item.id === command.targetId);
  if (
    !target ||
    target.userId !== command.userId ||
    target.assertionStatus !== "asserted" ||
    state.revision !== command.expectedRevision ||
    !sourceIsAdmissible(state, command.userId, command) ||
    !["correct", "withdraw"].includes(command.operation) ||
    (command.operation === "correct" && !nonEmpty(command.statement)) ||
    (command.operation === "withdraw" && command.statement != null)
  )
    return { outcome: "rejected", state };
  const next = clone(state);
  const prior = next.hypotheses.find((item) => item.id === target.id);
  prior.assertionStatus =
    command.operation === "correct" ? "superseded" : "withdrawn";
  let successorId;
  if (command.operation === "correct") {
    successorId = `hyp-${next.revision + 1}`;
    next.hypotheses.push({
      ...clone(prior),
      id: successorId,
      statement: command.statement,
      source: {
        actor: "user",
        reference: { kind: command.sourceKind, id: command.sourceId },
      },
      assertionStatus: "asserted",
      supersedesHypothesisId: prior.id,
    });
  }
  next.revision += 1;
  const receipt = { targetId: prior.id, successorId, revision: next.revision };
  next.replay[replay.key] = { signature: replay.signature, receipt };
  return {
    outcome: command.operation === "correct" ? "corrected" : "withdrawn",
    state: next,
    receipt,
  };
}

function queryCurrentHypotheses(state, userId, authenticatedUserId) {
  if (!nonEmpty(authenticatedUserId) || authenticatedUserId !== userId)
    return [];
  return state.hypotheses
    .filter(
      (item) => item.userId === userId && item.assertionStatus === "asserted",
    )
    .map((item) => ({
      id: item.id,
      statement: item.statement,
      assertionStatus: item.assertionStatus,
      sourceAvailability:
        state.sources[item.source.reference.id]?.available === true
          ? "available"
          : "unavailable",
      clinicalConclusion: false,
    }));
}

// Candidates are already selected by an external, not-yet-approved grouping
// policy (DC-032). The projection MUST NOT invent a grouping algorithm, stable
// group key, medical diagnosis, or persistent duplicate health-data owner.
function queryConcernGroups(state, userId, candidates, authenticatedUserId) {
  if (!nonEmpty(authenticatedUserId) || authenticatedUserId !== userId)
    return [];
  const before = JSON.stringify(state);
  const result = candidates.map((candidate) => {
    const fields = {
      relatedFactIds: "facts",
      relatedObservationIds: "observations",
      relatedHypothesisIds: "hypotheses",
      relatedEpisodeIds: "episodes",
    };
    const projection = {};
    let needsReview = false;
    for (const [field, collection] of Object.entries(fields)) {
      const ids = candidate[field] ?? [];
      if (!Array.isArray(ids)) {
        needsReview = true;
        projection[field] = [];
        continue;
      }
      const ownerItems = state[collection] ?? [];
      projection[field] = ids.filter((id) => {
        const found = ownerItems.find(
          (item) => item.id === id && item.userId === userId,
        );
        if (!found) needsReview = true;
        return !!found;
      });
    }
    return {
      ...projection,
      freshness: needsReview ? "needs_review" : "current",
    };
  });
  assert.equal(
    JSON.stringify(state),
    before,
    "read-only regrouping must not mutate authority",
  );
  return result;
}

// Fixtures supply independent authenticated identity; real commands cannot set it.
const recordHypothesis = (state, command) =>
  executeRecordHypothesis(state, command, "owner");
const amendHypothesis = (state, command) =>
  executeAmendHypothesis(state, command, "owner");
const currentHypotheses = (state, userId) =>
  queryCurrentHypotheses(state, userId, "owner");
const projectConcernGroups = (state, userId, candidates) =>
  queryConcernGroups(state, userId, candidates, "owner");

const input = (changes = {}) => ({
  userId: "owner",
  actor: "user",
  sourceId: "msg-1",
  sourceKind: "user_message",
  idempotencyKey: "key-1",
  expectedRevision: 1,
  statement: "I suspect migraine",
  relatedFactIds: ["fact-1"],
  relatedObservationIds: [],
  ...changes,
});

test("SEC-01: record authenticates tenant before write or receipt replay", () => {
  const state = fixture();
  for (const actorId of [undefined, "", "other"]) {
    const denied = executeRecordHypothesis(state, input(), actorId);
    assert.equal(denied.outcome, "rejected");
    assert.strictEqual(denied.state, state);
  }
  assert.equal(
    executeRecordHypothesis(state, input({ userId: "other" }), "owner").outcome,
    "rejected",
  );
  const first = executeRecordHypothesis(state, input(), "owner");
  assert.equal(first.outcome, "recorded");
  const forgedReplay = executeRecordHypothesis(first.state, input(), "other");
  assert.equal(forgedReplay.outcome, "rejected");
  assert.strictEqual(forgedReplay.state, first.state);
  assert.deepEqual(
    first.state.hypotheses.map(({ id }) => id),
    [first.receipt.id],
  );
});

test("SEC-01: amendments authenticate tenant before exact replay and mutation", () => {
  const first = executeRecordHypothesis(fixture(), input(), "owner");
  const command = {
    ...input({ sourceId: "msg-2", idempotencyKey: "fix", expectedRevision: 2 }),
    operation: "correct",
    targetId: first.receipt.id,
    statement: "corrected by me",
  };
  assert.equal(
    executeAmendHypothesis(first.state, command, "other").outcome,
    "rejected",
  );
  assert.equal(
    executeAmendHypothesis(first.state, command, undefined).outcome,
    "rejected",
  );
  const corrected = executeAmendHypothesis(first.state, command, "owner");
  assert.equal(corrected.outcome, "corrected");
  assert.equal(
    executeAmendHypothesis(corrected.state, command, "other").outcome,
    "rejected",
  );
  assert.equal(
    executeAmendHypothesis(
      corrected.state,
      { ...command, userId: "other" },
      "owner",
    ).outcome,
    "rejected",
  );
  assert.equal(corrected.state.hypotheses.length, 2);
});

test("SEC-01: read models reject forged authenticated tenant contexts", () => {
  const first = recordHypothesis(fixture(), input());
  assert.deepEqual(queryCurrentHypotheses(first.state, "owner", "other"), []);
  assert.deepEqual(queryCurrentHypotheses(first.state, "owner", undefined), []);
  assert.deepEqual(
    queryConcernGroups(
      first.state,
      "owner",
      [
        {
          relatedFactIds: ["fact-1"],
          relatedHypothesisIds: [first.receipt.id],
        },
      ],
      "other",
    ),
    [],
  );
  assert.equal(queryCurrentHypotheses(first.state, "owner", "owner").length, 1);
});

test("BS-04: explicit self hypothesis records user source, not Diagnosis", () => {
  const state = fixture();
  const result = recordHypothesis(state, input());
  assert.equal(result.outcome, "recorded");
  assert.equal(result.state.hypotheses[0].source.actor, "user");
  assert.equal(result.state.hypotheses[0].assertionStatus, "asserted");
  assert.deepEqual(result.state.diagnosis, []);
  assert.deepEqual(state.hypotheses, []);
});
test("BS-04b/BS-14: agent inference cannot be user BodyHypothesis", () => {
  const state = fixture();
  assert.equal(
    recordHypothesis(state, input({ actor: "agent" })).outcome,
    "rejected",
  );
  assert.equal(
    recordHypothesis(state, input({ sourceId: "agent-1" })).outcome,
    "rejected",
  );
  assert.equal(
    recordHypothesis(state, input({ sourceKind: "agent_analysis" })).outcome,
    "rejected",
  );
  assert.deepEqual(state.hypotheses, []);
});
test("BS-02/SEC-01: foreign and missing source or missing statement reject", () => {
  const state = fixture();
  for (const patch of [
    { sourceId: "foreign" },
    { sourceId: "unavailable" },
    { statement: " " },
  ]) {
    assert.equal(recordHypothesis(state, input(patch)).outcome, "rejected");
  }
  assert.deepEqual(state.hypotheses, []);
});
test("BS-D9: links must resolve within tenant, no implicit evidence copies", () => {
  const state = fixture();
  for (const patch of [
    { relatedFactIds: ["unknown-fact"] },
    { relatedObservationIds: ["unknown-observation"] },
    { relatedFactIds: "fact-1" },
  ])
    assert.equal(recordHypothesis(state, input(patch)).outcome, "rejected");
  const result = recordHypothesis(
    state,
    input({ relatedObservationIds: ["obs-1"] }),
  );
  assert.equal(result.state.hypotheses[0].relatedObservationIds[0], "obs-1");
  assert.deepEqual(result.state.facts, state.facts);
  assert.deepEqual(result.state.observations, state.observations);
});
test("INV-09: exact replay returns stable receipt, changed key payload rejects", () => {
  const first = recordHypothesis(fixture(), input());
  const repeated = recordHypothesis(first.state, input());
  assert.equal(repeated.outcome, "replayed");
  assert.deepEqual(repeated.receipt, first.receipt);
  assert.strictEqual(repeated.state, first.state);
  assert.equal(
    recordHypothesis(first.state, input({ statement: "different" })).outcome,
    "rejected",
  );
});
test("INV-09: stale revision cannot append hypothesis", () => {
  const first = recordHypothesis(fixture(), input());
  assert.equal(
    recordHypothesis(first.state, input({ idempotencyKey: "key-2" })).outcome,
    "rejected",
  );
  assert.equal(first.state.hypotheses.length, 1);
});
test("BS-28: explicit correction appends successor and preserves Fact/Observation", () => {
  const first = recordHypothesis(fixture(), input());
  const result = amendHypothesis(first.state, {
    ...input({ sourceId: "msg-2", idempotencyKey: "fix", expectedRevision: 2 }),
    operation: "correct",
    targetId: first.receipt.id,
    statement: "I no longer suspect migraine",
  });
  assert.equal(result.outcome, "corrected");
  assert.equal(result.state.hypotheses.length, 2);
  assert.equal(result.state.hypotheses[0].assertionStatus, "superseded");
  assert.equal(
    result.state.hypotheses[1].supersedesHypothesisId,
    first.receipt.id,
  );
  assert.deepEqual(result.state.facts, first.state.facts);
  assert.deepEqual(result.state.observations, first.state.observations);
  assert.deepEqual(
    result.state.safetyRestrictions,
    first.state.safetyRestrictions,
  );
});
test("BS-28: withdrawal is not correction, diagnosis change or privacy erasure", () => {
  const first = recordHypothesis(fixture(), input());
  const result = amendHypothesis(first.state, {
    ...input({
      sourceId: "direct-1",
      sourceKind: "direct_edit",
      idempotencyKey: "withdraw",
      expectedRevision: 2,
    }),
    operation: "withdraw",
    targetId: first.receipt.id,
    statement: undefined,
  });
  assert.equal(result.outcome, "withdrawn");
  assert.equal(result.state.hypotheses.length, 1);
  assert.equal(result.state.hypotheses[0].assertionStatus, "withdrawn");
  assert.equal(result.state.sources["msg-1"].available, true);
  assert.deepEqual(result.state.diagnosis, []);
  assert.deepEqual(result.state.facts, first.state.facts);
});
test("BS-28: foreign/duplicate/agent/stale corrections reject without mutation", () => {
  const first = recordHypothesis(fixture(), input());
  const correct = {
    ...input({ sourceId: "msg-2", idempotencyKey: "fix", expectedRevision: 2 }),
    operation: "correct",
    targetId: first.receipt.id,
    statement: "I meant something else",
  };
  for (const patch of [
    { userId: "other" },
    { actor: "agent" },
    { expectedRevision: 1 },
    { targetId: "missing" },
    { sourceId: "unavailable" },
  ])
    assert.equal(
      amendHypothesis(first.state, { ...correct, ...patch }).outcome,
      "rejected",
    );
  const amended = amendHypothesis(first.state, correct);
  assert.equal(amended.outcome, "corrected");
  assert.equal(
    amendHypothesis(amended.state, {
      ...correct,
      idempotencyKey: "again",
      expectedRevision: 3,
    }).outcome,
    "rejected",
  );
});
test("BS-28: exact retry does not duplicate amendment", () => {
  const first = recordHypothesis(fixture(), input());
  const amendment = {
    ...input({ sourceId: "msg-2", idempotencyKey: "fix", expectedRevision: 2 }),
    operation: "correct",
    targetId: first.receipt.id,
    statement: "corrected text",
  };
  const once = amendHypothesis(first.state, amendment);
  const replay = amendHypothesis(once.state, amendment);
  assert.equal(replay.outcome, "replayed");
  assert.deepEqual(replay.receipt, once.receipt);
  assert.equal(replay.state.hypotheses.length, 2);
});
test("INV-02: source unavailability changes disclosure, not clinical certainty", () => {
  const first = recordHypothesis(fixture(), input());
  const next = clone(first.state);
  next.sources["msg-1"].available = false;
  assert.deepEqual(currentHypotheses(next, "owner"), [
    {
      id: first.receipt.id,
      statement: "I suspect migraine",
      assertionStatus: "asserted",
      sourceAvailability: "unavailable",
      clinicalConclusion: false,
    },
  ]);
  assert.deepEqual(currentHypotheses(next, "other"), []);
  assert.deepEqual(next.diagnosis, []);
});
test("BS-15/INV-25: ConcernGroup regrouping is read-only, no second authority", () => {
  const first = recordHypothesis(fixture(), input());
  const snapshot = JSON.stringify(first.state);
  const groupsA = projectConcernGroups(first.state, "owner", [
    {
      relatedFactIds: ["fact-1"],
      relatedHypothesisIds: [first.receipt.id],
    },
  ]);
  const groupsB = projectConcernGroups(first.state, "owner", [
    {
      relatedObservationIds: ["obs-1"],
      relatedHypothesisIds: [first.receipt.id],
    },
  ]);
  assert.equal(groupsA[0].freshness, "current");
  assert.equal(groupsB[0].freshness, "current");
  assert.equal(JSON.stringify(first.state), snapshot);
  assert.equal(first.state.hypotheses.length, 1);
});
test("BS-15/SEC-01: invalid and foreign concern links never leak facts", () => {
  const state = fixture();
  state.facts.push({
    id: "private-fact",
    userId: "other",
    statement: "private",
  });
  const result = projectConcernGroups(state, "owner", [
    {
      relatedFactIds: ["fact-1", "private-fact", "unknown"],
    },
  ]);
  assert.deepEqual(result[0].relatedFactIds, ["fact-1"]);
  assert.equal(result[0].freshness, "needs_review");
});

// BH-03: existing owner-bound linked records cannot be replaced by another
// owner's valid-looking references, even when that owner supplied real IDs.
test("BH-03: cross-owner Fact and Observation references reject atomically", () => {
  const state = fixture();
  state.facts.push({
    id: "foreign-fact",
    userId: "other",
    statement: "private",
    assertionStatus: "asserted",
  });
  state.observations.push({
    id: "foreign-observation",
    userId: "other",
    rawValue: "private",
    assertionStatus: "asserted",
  });
  const snapshot = JSON.stringify(state);
  for (const patch of [
    { relatedFactIds: ["fact-1", "foreign-fact"] },
    { relatedObservationIds: ["obs-1", "foreign-observation"] },
  ]) {
    const result = executeRecordHypothesis(state, input(patch), "owner");
    assert.equal(result.outcome, "rejected");
    assert.strictEqual(result.state, state);
  }
  assert.equal(JSON.stringify(state), snapshot);
});

// BH-04/BH-05: replay is bound to original target, action, source and
// payload. A new key cannot bypass the non-current assertion guard.
test("BH-04/BH-05: corrected hypothesis refuses altered retry and second amendment", () => {
  const first = recordHypothesis(fixture(), input());
  const command = {
    ...input({
      sourceId: "msg-2",
      idempotencyKey: "correct-once",
      expectedRevision: 2,
    }),
    operation: "correct",
    targetId: first.receipt.id,
    statement: "my corrected suspicion",
  };
  const updated = amendHypothesis(first.state, command);
  assert.equal(updated.outcome, "corrected");
  for (const patch of [
    { operation: "withdraw", statement: undefined },
    { targetId: updated.receipt.successorId },
    { statement: "different replacement" },
    { sourceId: "direct-1", sourceKind: "direct_edit" },
  ]) {
    const denied = amendHypothesis(updated.state, { ...command, ...patch });
    assert.equal(denied.outcome, "rejected");
    assert.strictEqual(denied.state, updated.state);
  }
  const second = amendHypothesis(updated.state, {
    ...command,
    idempotencyKey: "second-write",
    expectedRevision: 3,
  });
  assert.equal(second.outcome, "rejected");
  assert.equal(updated.state.hypotheses.length, 2);
  assert.equal(updated.state.hypotheses[0].assertionStatus, "superseded");
});

// BH-07/BH-08: derived grouping consumes untrusted candidate references,
// reveals only owner-valid IDs and cannot create independent health state.
test("BH-07/BH-08: mixed foreign, malformed and missing group references are read-only", () => {
  const state = fixture();
  const first = recordHypothesis(state, input());
  const model = first.state;
  model.hypotheses.push({
    id: "foreign-hypothesis",
    userId: "other",
    statement: "private",
    assertionStatus: "asserted",
  });
  model.episodes = [
    { id: "own-episode", userId: "owner" },
    { id: "foreign-episode", userId: "other" },
  ];
  const snapshot = JSON.stringify(model);
  const results = queryConcernGroups(
    model,
    "owner",
    [
      {
        relatedFactIds: ["fact-1", "no-such-fact"],
        relatedObservationIds: "not-a-list",
        relatedHypothesisIds: [first.receipt.id, "foreign-hypothesis"],
        relatedEpisodeIds: ["own-episode", "foreign-episode"],
        guessDiagnosis: "must not leak",
      },
      {
        relatedFactIds: ["fact-1"],
        relatedHypothesisIds: [first.receipt.id],
        relatedEpisodeIds: ["own-episode"],
      },
    ],
    "owner",
  );
  assert.deepEqual(results[0], {
    relatedFactIds: ["fact-1"],
    relatedObservationIds: [],
    relatedHypothesisIds: [first.receipt.id],
    relatedEpisodeIds: ["own-episode"],
    freshness: "needs_review",
  });
  assert.deepEqual(results[1], {
    relatedFactIds: ["fact-1"],
    relatedObservationIds: [],
    relatedHypothesisIds: [first.receipt.id],
    relatedEpisodeIds: ["own-episode"],
    freshness: "current",
  });
  assert.equal(JSON.stringify(model), snapshot);
  assert.equal(model.revision, first.receipt.revision);
});
