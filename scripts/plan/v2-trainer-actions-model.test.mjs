// DS-TP-01 executable *reference* model, not a deployed Trainer or medical gate.
// The inputs are already owner-admitted user actions and Go-verified current gate
// decisions. Source of truth: docs/product/domain/training.md TP-D6..D10 and
// docs/product/domain/safety.md SAFE-D2/D6. No network/database state is altered.
import assert from "node:assert/strict";
import { test } from "node:test";

const clone = (value) => structuredClone(value);
const safeId = (value) => typeof value === "string" && value.trim().length > 0;

function localTrainingDate(startedAt, timeZoneSnapshot) {
  if (!Number.isFinite(Date.parse(startedAt)) || !safeId(timeZoneSnapshot))
    return null;
  try {
    const parts = Object.fromEntries(
      new Intl.DateTimeFormat("en-US", {
        timeZone: timeZoneSnapshot,
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
      })
        .formatToParts(new Date(startedAt))
        .filter((part) => part.type !== "literal")
        .map((part) => [part.type, part.value]),
    );
    return `${parts.year}-${parts.month}-${parts.day}`;
  } catch {
    return null;
  }
}

// The caller supplies an accepted plan/template and a *fresh*, verified Go gate.
// In particular, a draft proposal, cached UI allow or raw user text is not one.
function startReferenceSession({
  existingSessions,
  userId,
  plan,
  templateId,
  sessionLogId,
  startedAt,
  timeZoneSnapshot,
  verifiedGate,
}) {
  const date = localTrainingDate(startedAt, timeZoneSnapshot);
  if (
    !safeId(userId) ||
    !safeId(sessionLogId) ||
    !safeId(templateId) ||
    !date ||
    plan?.userId !== userId ||
    plan?.accepted !== true ||
    !safeId(plan.planId) ||
    !safeId(plan.acceptedRevisionId) ||
    !plan.templateIds?.includes(templateId) ||
    existingSessions.some((s) => s.sessionLogId === sessionLogId) ||
    existingSessions.some(
      (s) => s.userId === userId && s.state === "in_progress",
    ) ||
    verifiedGate?.available !== true ||
    !["allow", "warn"].includes(verifiedGate.result)
  )
    return null;
  return {
    sessionLogId,
    userId,
    planId: plan.planId,
    planRevisionId: plan.acceptedRevisionId,
    templateId,
    startedAt,
    trainingDate: date,
    timeZoneSnapshot,
    state: "in_progress",
    actions: [],
    revision: 0,
  };
}

// A deliberately bounded append-only reference. The real Go owner MUST also
// validate tenant, source provenance, accepted revision and action/gate admission
// within a transaction, not rely on this JavaScript fixture or a UI flag.
function appendReferenceAction(session, actorId, command, verifiedGate) {
  const reject = { kind: "rejected", session };
  if (
    !session ||
    !command ||
    session.userId !== actorId ||
    session.state !== "in_progress" ||
    verifiedGate?.available !== true ||
    !["allow", "warn"].includes(verifiedGate.result) ||
    !safeId(command.idempotencyKey) ||
    !safeId(command.sourceId) ||
    !safeId(command.planExerciseId)
  )
    return reject;
  const signature = JSON.stringify({
    kind: command.kind,
    scope: command.scope,
    setNumber: command.setNumber ?? null,
    planExerciseId: command.planExerciseId,
    correctsActionId: command.correctsActionId ?? null,
    sourceId: command.sourceId,
  });
  const replay = session.actions.find(
    (a) => a.idempotencyKey === command.idempotencyKey,
  );
  if (replay)
    return replay.signature === signature
      ? { kind: "replayed", session, actionId: replay.actionId }
      : reject;
  if (
    command.expectedRevision !== session.revision ||
    !["completed", "skipped", "corrected"].includes(command.kind) ||
    !["set", "exercise"].includes(command.scope) ||
    (command.scope === "set" &&
      (!Number.isSafeInteger(command.setNumber) || command.setNumber < 1)) ||
    (command.scope === "exercise" && command.setNumber != null)
  )
    return reject;
  if (command.kind === "corrected") {
    const prior = session.actions.find(
      (a) => a.actionId === command.correctsActionId,
    );
    if (
      !prior ||
      prior.kind === "corrected" ||
      prior.planExerciseId !== command.planExerciseId ||
      prior.scope !== command.scope ||
      (prior.setNumber ?? null) !== (command.setNumber ?? null) ||
      session.actions.some((a) => a.correctsActionId === prior.actionId)
    )
      return reject;
  } else if (command.correctsActionId != null) {
    return reject;
  }
  const next = clone(session);
  const actionId = `action-${next.actions.length + 1}`;
  next.actions.push({
    ...command,
    actionId,
    signature,
  });
  next.revision += 1;
  return { kind: "appended", session: next, actionId };
}

function effectiveActions(session) {
  const correctedIds = new Set(
    session.actions
      .filter((a) => a.kind === "corrected")
      .map((a) => a.correctsActionId),
  );
  return session.actions.filter(
    (a) => a.kind !== "corrected" && !correctedIds.has(a.actionId),
  );
}

function applyVerifiedSafetyTransition(session, result) {
  const next = clone(session);
  // Unavailable verification must block new actions but must NOT invent a
  // safety event, finished session or implicit skipped sets.
  if (result?.available !== true) return next;
  if (result.result === "urgent") {
    next.state = "ended";
    next.endedReason = "safety_stopped";
  } else if (result.result === "restricted" && next.state === "in_progress") {
    next.state = "paused";
    next.pauseReason = "safety_hold";
  }
  return next;
}

const fresh = { available: true, result: "allow" };
const plan = {
  userId: "owner",
  planId: "plan-A",
  accepted: true,
  acceptedRevisionId: "rev-8",
  templateIds: ["template-1"],
};
const start = (overrides = {}) =>
  startReferenceSession({
    existingSessions: [],
    userId: "owner",
    plan,
    templateId: "template-1",
    sessionLogId: "session-1",
    startedAt: "2026-10-13T16:30:00Z",
    timeZoneSnapshot: "Asia/Shanghai",
    verifiedGate: fresh,
    ...overrides,
  });
const action = (overrides = {}) => ({
  idempotencyKey: "cmd-1",
  sourceId: "user-confirmation-1",
  planExerciseId: "plan-exercise-1",
  scope: "set",
  setNumber: 1,
  kind: "completed",
  expectedRevision: 0,
  ...overrides,
});

test("TP-D6: IANA local date is captured at start, not server/UTC date", () => {
  const session = start();
  assert.equal(session.trainingDate, "2026-10-14");
  assert.equal(session.startedAt, "2026-10-13T16:30:00Z");
  assert.equal(session.timeZoneSnapshot, "Asia/Shanghai");
  assert.equal(
    localTrainingDate("2026-10-14T06:00:00Z", "Asia/Shanghai"),
    "2026-10-14",
  );
  assert.equal(localTrainingDate("bad", "Asia/Shanghai"), null);
  assert.equal(localTrainingDate("2026-10-14T00:00:00Z", "not-a-zone"), null);
});
test("TP-D6: two same-day sessions preserve independent IDs, completed history and timezone", () => {
  const first = { ...start(), state: "ended", endedReason: "finished" };
  const second = start({
    existingSessions: [first],
    sessionLogId: "session-2",
  });
  assert.equal(second.trainingDate, first.trainingDate);
  assert.notEqual(second.sessionLogId, first.sessionLogId);
  assert.equal(second.planRevisionId, first.planRevisionId);
  assert.equal(
    start({ existingSessions: [start()], sessionLogId: "session-3" }),
    null,
  );
});
test("TP-D1/D7: draft, foreign plan, bad template, absent gate cannot start", () => {
  assert.equal(start({ plan: { ...plan, accepted: false } }), null);
  assert.equal(start({ plan: { ...plan, userId: "other" } }), null);
  assert.equal(start({ templateId: "not-in-plan" }), null);
  assert.equal(
    start({ verifiedGate: { available: false, result: "allow" } }),
    null,
  );
  assert.equal(
    start({ verifiedGate: { available: true, result: "block" } }),
    null,
  );
});
test("TP-D7: navigation/demo/close/disconnect do not append execution facts", () => {
  const session = start();
  for (const event of [
    "navigate",
    "video_play",
    "close_window",
    "disconnect",
  ]) {
    assert.equal(
      appendReferenceAction(session, "owner", action({ kind: event }), fresh)
        .kind,
      "rejected",
    );
    assert.deepEqual(session.actions, []);
  }
});
test("TP-D7/SEC-01: source, scope, actor and revision are mandatory", () => {
  const session = start();
  for (const cmd of [
    action({ sourceId: "" }),
    action({ setNumber: undefined }),
    action({ setNumber: 0 }),
    action({ scope: "exercise", setNumber: 1 }),
    action({ expectedRevision: 1 }),
    action({ planExerciseId: "" }),
  ])
    assert.equal(
      appendReferenceAction(session, "owner", cmd, fresh).kind,
      "rejected",
    );
  assert.equal(
    appendReferenceAction(session, "other", action(), fresh).kind,
    "rejected",
  );
  assert.deepEqual(session.actions, []);
});
test("TP-D8: exact idempotency replay yields one action; changed payload or stale CAS rejects", () => {
  const session = start();
  const first = appendReferenceAction(session, "owner", action(), fresh);
  assert.equal(first.kind, "appended");
  assert.equal(first.session.revision, 1);
  const replay = appendReferenceAction(first.session, "owner", action(), fresh);
  assert.equal(replay.kind, "replayed");
  assert.equal(replay.actionId, first.actionId);
  assert.equal(replay.session.actions.length, 1);
  assert.equal(
    appendReferenceAction(
      first.session,
      "owner",
      action({ kind: "skipped" }),
      fresh,
    ).kind,
    "rejected",
  );
  assert.equal(
    appendReferenceAction(
      first.session,
      "owner",
      action({ idempotencyKey: "cmd-2" }),
      fresh,
    ).kind,
    "rejected",
  );
});
test("TP-D8: exact user correction is an append, not deletion or silent rewrite", () => {
  const first = appendReferenceAction(start(), "owner", action(), fresh);
  const correction = appendReferenceAction(
    first.session,
    "owner",
    action({
      idempotencyKey: "correct-1",
      kind: "corrected",
      expectedRevision: 1,
      correctsActionId: first.actionId,
    }),
    fresh,
  );
  assert.equal(correction.kind, "appended");
  assert.equal(correction.session.actions.length, 2);
  assert.equal(effectiveActions(correction.session).length, 0);
  assert.equal(first.session.actions[0].kind, "completed");
  assert.equal(
    appendReferenceAction(
      correction.session,
      "owner",
      action({
        idempotencyKey: "correct-2",
        kind: "corrected",
        expectedRevision: 2,
        correctsActionId: first.actionId,
      }),
      fresh,
    ).kind,
    "rejected",
  );
});
test("TP-D8: correction cannot target unknown/foreign action or another set", () => {
  const first = appendReferenceAction(start(), "owner", action(), fresh);
  for (const cmd of [
    action({
      kind: "corrected",
      correctsActionId: "foreign",
      expectedRevision: 1,
    }),
    action({
      kind: "corrected",
      correctsActionId: first.actionId,
      setNumber: 2,
      expectedRevision: 1,
    }),
    action({
      kind: "corrected",
      correctsActionId: first.actionId,
      planExerciseId: "other",
      expectedRevision: 1,
    }),
  ])
    assert.equal(
      appendReferenceAction(first.session, "owner", cmd, fresh).kind,
      "rejected",
    );
});
test("TP-D7: skip exercise records one explicit action, never fake skipped sets", () => {
  const result = appendReferenceAction(
    start(),
    "owner",
    action({ kind: "skipped", scope: "exercise", setNumber: undefined }),
    fresh,
  );
  assert.equal(result.kind, "appended");
  assert.equal(result.session.actions.length, 1);
  assert.equal(result.session.actions[0].scope, "exercise");
});
test("SAFE-D6: matching restricted holds; urgent stops; neither invents skipped/complete", () => {
  const first = appendReferenceAction(
    start(),
    "owner",
    action(),
    fresh,
  ).session;
  const hold = applyVerifiedSafetyTransition(first, {
    available: true,
    result: "restricted",
  });
  assert.equal(hold.state, "paused");
  assert.equal(hold.pauseReason, "safety_hold");
  assert.deepEqual(hold.actions, first.actions);
  assert.equal(
    appendReferenceAction(
      hold,
      "owner",
      action({ idempotencyKey: "cmd-2", expectedRevision: 1 }),
      fresh,
    ).kind,
    "rejected",
  );
  const stopped = applyVerifiedSafetyTransition(hold, {
    available: true,
    result: "urgent",
  });
  assert.equal(stopped.state, "ended");
  assert.equal(stopped.endedReason, "safety_stopped");
  assert.deepEqual(stopped.actions, first.actions);
});
test("SAFE-D6: missing gate must not mutate history or claim safe continuation", () => {
  const first = appendReferenceAction(
    start(),
    "owner",
    action(),
    fresh,
  ).session;
  assert.equal(
    appendReferenceAction(
      first,
      "owner",
      action({ idempotencyKey: "cmd-2", expectedRevision: 1 }),
      { available: false, result: "allow" },
    ).kind,
    "rejected",
  );
  assert.deepEqual(
    applyVerifiedSafetyTransition(first, {
      available: false,
      result: "urgent",
    }),
    first,
  );
});
