import { describe, expect, it } from "vitest";
import { ApiRequestError } from "@/lib/api-client";
import {
  EMPTY_DECISION_SESSION,
  decisionAttemptContent,
  diagnosisHeaderMode,
  projectDiagnosisStepper,
  projectNativeDiagnosisWindow,
  settlementAttemptContent,
  withCommandError,
  withDecisionReceipt,
  withProposalReviewed,
  withSettlementReceipt,
} from "./diagnosisDecisionViewModel";
import {
  CONDITION_ID,
  FINGERPRINT_A,
  PROPOSAL_A,
  PROPOSAL_B,
  RUN_ID,
  makeCondition,
  makeDecisionReceipt,
  makeDiagnosis,
  makeProposal,
  makeRun,
  makeSettlementReceipt,
} from "./__fixtures__/nativeDiagnosisFixtures";

const conflict = new ApiRequestError("changed", 409, "REVISION_CONFLICT");
const safety = new ApiRequestError("urgent", 409, "SAFETY_BLOCKED");

function rows(view: ReturnType<typeof projectNativeDiagnosisWindow>) {
  return Object.fromEntries(
    (view.run?.proposals ?? []).map((row) => [row.proposalId, row]),
  );
}

describe("projectNativeDiagnosisWindow", () => {
  it("lists only adopted ConditionEntries and never turns proposals into conditions", () => {
    const view = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis({
        conditionEntries: [
          makeCondition(),
          makeCondition({
            conditionId: "00000000-0000-4000-8000-0000000000c2",
            medicalName: "已停用",
            disposition: "inactive",
          }),
        ],
      }),
      run: makeRun(),
    });
    expect(view.confirmed.map((item) => item.name)).toEqual(["肩峰下撞击"]);
    expect(view.confirmed[0]).toMatchObject({
      conditionId: CONDITION_ID,
      certainty: "user_confirmed_agent_analysis",
      cause: { summary: "肩胛控制不足", evidenceStrength: "较可能" },
    });
    expect(view.run?.proposals.map((row) => row.state)).toEqual([
      "undecided",
      "undecided",
    ]);
    expect(view.run?.proposals[0]).toMatchObject({
      title: "上交叉综合征",
      levelLabel: "疑似",
      selectable: true,
    });
  });

  it("labels certainty from the source claim kind, not from AI output", () => {
    const [clinical] = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis({
        conditionEntries: [
          makeCondition({
            sourceClaimRefs: [
              {
                claimId: "claim-1",
                sourceKind: "clinical_document",
                sourceId: "doc-1",
                sourceVersion: "1",
                statement: "MRI 报告",
                recordedAt: "2026-10-18T03:00:00Z",
                validityAtAssessment: "current",
              },
            ],
          }),
        ],
      }),
      run: null,
    }).confirmed;
    expect(clinical?.certainty).toBe("clinically_documented");
  });

  describe("stale 409", () => {
    it("marks a proposal changed after REVISION_CONFLICT until the user re-reviews it", () => {
      const session = withCommandError(
        EMPTY_DECISION_SESSION,
        RUN_ID,
        conflict,
        PROPOSAL_A,
      );
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun(),
        session,
      });
      expect(rows(view)[PROPOSAL_A]).toMatchObject({
        state: "changed",
        selectable: false,
        needsReview: true,
        decision: null,
      });
      expect(rows(view)[PROPOSAL_B]?.state).toBe("undecided");
      expect(view.run?.canSettle).toBe(false);

      const reviewed = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun(),
        session: withProposalReviewed(session, PROPOSAL_A),
      });
      expect(rows(reviewed)[PROPOSAL_A]).toMatchObject({
        state: "undecided",
        selectable: true,
      });
    });

    it("marks the whole run changed after a settlement conflict", () => {
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun(),
        session: withCommandError(EMPTY_DECISION_SESSION, RUN_ID, conflict),
      });
      expect(view.run).toMatchObject({
        changed: true,
        canDecide: false,
        canSettle: false,
      });
      expect(view.run?.proposals.every((row) => row.state === "changed")).toBe(
        true,
      );
    });

    it("treats a Diagnosis revision ahead of the run as stale without any request", () => {
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis({ currentRevision: 3 }),
        run: makeRun({ currentDiagnosisRevision: 3 }),
      });
      expect(view.run?.changed).toBe(true);
      expect(view.run?.canDecide).toBe(false);
      expect(view.run?.proposals.map((row) => row.state)).toEqual([
        "changed",
        "changed",
      ]);
    });

    it("shows superseded proposals as changed", () => {
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun({
          proposals: [
            makeProposal({ state: "superseded", reason: "evidence_changed" }),
          ],
        }),
      });
      expect(rows(view)[PROPOSAL_A]?.state).toBe("changed");
    });

    it("freezes every decision after SAFETY_BLOCKED", () => {
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun(),
        session: withCommandError(EMPTY_DECISION_SESSION, RUN_ID, safety),
      });
      expect(view.safetyBlocked).toBe(true);
      expect(view.run?.canDecide).toBe(false);
      expect(view.run?.proposals.some((row) => row.selectable)).toBe(false);
    });

    it("does not mark anything changed for a non-conflict failure", () => {
      const session = withCommandError(
        EMPTY_DECISION_SESSION,
        RUN_ID,
        new ApiRequestError("down", 503, "DIAGNOSIS_UNAVAILABLE"),
        PROPOSAL_A,
      );
      expect(session.changedProposalIds).toEqual([]);
      expect(session.runChanged).toBe(false);
    });
  });

  describe("partial selection", () => {
    it("keeps unchosen proposals undecided and settles with closeRemaining", () => {
      const session = withDecisionReceipt(
        EMPTY_DECISION_SESSION,
        makeDecisionReceipt(),
      );
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun(),
        session,
      });
      expect(rows(view)[PROPOSAL_A]).toMatchObject({
        state: "pending",
        decision: "accept",
        selectable: false,
      });
      expect(rows(view)[PROPOSAL_B]).toMatchObject({
        state: "undecided",
        selectable: true,
      });
      expect(view.run).toMatchObject({
        acceptedCount: 1,
        undecidedCount: 1,
        closesRemaining: true,
        canSettle: true,
        committedReceiptId: null,
      });
    });

    it("shows rejected choices as rejected, never committed", () => {
      const session = withDecisionReceipt(
        EMPTY_DECISION_SESSION,
        makeDecisionReceipt({ decision: "reject" }),
      );
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun(),
        session,
      });
      expect(rows(view)[PROPOSAL_A]?.state).toBe("rejected");
      expect(view.run?.acceptedCount).toBe(0);
    });

    it("ignores a decision receipt for a different fingerprint", () => {
      const session = withDecisionReceipt(
        EMPTY_DECISION_SESSION,
        makeDecisionReceipt({ contentFingerprint: "c".repeat(64) }),
      );
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun(),
        session,
      });
      expect(rows(view)[PROPOSAL_A]?.state).toBe("undecided");
    });
  });

  describe("committed only after receipt", () => {
    it("keeps an accepted choice pending without a settlement receipt", () => {
      const decided = makeProposal({
        state: "decided",
        decision: {
          decisionId: "00000000-0000-4000-8000-0000000000d1",
          decision: "accept",
          contentFingerprint: FINGERPRINT_A,
          receiptId: "00000000-0000-4000-8000-0000000000d2",
          decidedAt: "2026-10-18T03:00:00Z",
        },
      });
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun({ proposals: [decided] }),
      });
      expect(rows(view)[PROPOSAL_A]?.state).toBe("pending");
      expect(view.run?.committedReceiptId).toBeNull();
    });

    it("does not trust a committed run state that has no settlement receipt", () => {
      const receipt = makeDecisionReceipt();
      const committedWithoutReceipt = makeRun({
        state: "committed",
        outcome: "committed",
      });
      const noReceipt = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: committedWithoutReceipt,
        session: withDecisionReceipt(EMPTY_DECISION_SESSION, receipt),
      });
      expect(rows(noReceipt)[PROPOSAL_A]?.state).toBe("pending");
      expect(noReceipt.run?.committedReceiptId).toBeNull();

      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: committedWithoutReceipt,
        session: withSettlementReceipt(
          withDecisionReceipt(EMPTY_DECISION_SESSION, receipt),
          makeSettlementReceipt({
            outcome: "closed_without_change",
            revision: 2,
            diagnosisRevisionId: undefined,
            decisionIds: [receipt.decisionId],
          }),
        ),
      });
      expect(rows(view)[PROPOSAL_A]?.state).toBe("pending");
      expect(view.run?.committedReceiptId).toBeNull();
    });

    it("marks only receipt-listed accepted decisions committed", () => {
      const receipt = makeDecisionReceipt();
      let session = withDecisionReceipt(EMPTY_DECISION_SESSION, receipt);
      session = withSettlementReceipt(
        session,
        makeSettlementReceipt({ decisionIds: [receipt.decisionId] }),
      );
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis({ currentRevision: 2 }),
        run: makeRun(),
        session,
      });
      expect(rows(view)[PROPOSAL_A]?.state).toBe("committed");
      expect(rows(view)[PROPOSAL_B]?.state).toBe("undecided");
      expect(rows(view)[PROPOSAL_B]?.selectable).toBe(false);
      expect(view.run?.committedReceiptId).toEqual(expect.any(String));
      expect(view.run?.canDecide).toBe(false);

      const unlisted = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun(),
        session: withSettlementReceipt(
          withDecisionReceipt(EMPTY_DECISION_SESSION, receipt),
          makeSettlementReceipt({ decisionIds: [] }),
        ),
      });
      expect(rows(unlisted)[PROPOSAL_A]?.state).toBe("pending");
    });

    it("keeps the session's non-commit receipt authoritative over a refetch", () => {
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis({ currentRevision: 3 }),
        run: makeRun({
          state: "committed",
          outcome: "committed",
          currentDiagnosisRevision: 3,
          committedDiagnosisRevision: 3,
          proposals: [
            makeProposal({
              state: "decided",
              decision: {
                decisionId: "00000000-0000-4000-8000-0000000000d1",
                decision: "accept",
                contentFingerprint: FINGERPRINT_A,
                receiptId: "00000000-0000-4000-8000-0000000000d2",
                decidedAt: "2026-10-18T03:00:00Z",
              },
            }),
          ],
          settlement: {
            settlementId: "00000000-0000-4000-8000-0000000000f1",
            outcome: "committed",
            revision: 3,
            diagnosisRevisionId: "00000000-0000-4000-8000-0000000000f2",
            receiptId: "00000000-0000-4000-8000-0000000000f3",
            settledAt: "2026-10-18T03:00:00Z",
          },
        }),
        session: withSettlementReceipt(
          EMPTY_DECISION_SESSION,
          makeSettlementReceipt({ outcome: "safety_blocked", revision: 2 }),
        ),
      });
      // The receipt this session holds wins over a refetched projection; a
      // safety-blocked receipt is never adoption.
      expect(rows(view)[PROPOSAL_A]?.state).toBe("pending");
      expect(view.safetyBlocked).toBe(true);
    });

    it("accepts the server settlement receipt after a lost response", () => {
      const receipt = makeDecisionReceipt();
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis({ currentRevision: 3 }),
        run: makeRun({
          state: "committed",
          outcome: "committed",
          currentDiagnosisRevision: 3,
          settlement: {
            settlementId: "00000000-0000-4000-8000-0000000000f1",
            outcome: "committed",
            revision: 3,
            diagnosisRevisionId: "00000000-0000-4000-8000-0000000000f2",
            receiptId: "00000000-0000-4000-8000-0000000000f3",
            settledAt: "2026-10-18T03:00:00Z",
          },
        }),
        session: withDecisionReceipt(EMPTY_DECISION_SESSION, receipt),
      });
      expect(rows(view)[PROPOSAL_A]?.state).toBe("committed");
      expect(view.run?.committedReceiptId).toBe(
        "00000000-0000-4000-8000-0000000000f3",
      );
      expect(view.run?.canDecide).toBe(false);
    });

    it("hides terminal runs this session did not act on", () => {
      const view = projectNativeDiagnosisWindow({
        diagnosis: makeDiagnosis(),
        run: makeRun({ state: "closed_without_change" }),
      });
      expect(view.run).toBeNull();
    });
  });

  it("derives stable idempotency content per command", () => {
    const row = { proposalId: PROPOSAL_A, contentFingerprint: FINGERPRINT_A };
    expect(decisionAttemptContent(RUN_ID, row, "accept", 2)).toBe(
      decisionAttemptContent(RUN_ID, row, "accept", 2),
    );
    expect(decisionAttemptContent(RUN_ID, row, "accept", 2)).not.toBe(
      decisionAttemptContent(RUN_ID, row, "reject", 2),
    );
    expect(settlementAttemptContent(RUN_ID, 2, true)).not.toBe(
      settlementAttemptContent(RUN_ID, 3, true),
    );
  });
});

describe("Penpot Diagnosis phases", () => {
  it("numbers proposals and stays deciding until a receipt exists", () => {
    const view = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun(),
      session: withDecisionReceipt(
        EMPTY_DECISION_SESSION,
        makeDecisionReceipt(),
      ),
    });
    expect(view.run?.phase).toBe("deciding");
    expect(view.run?.committedCount).toBe(0);
    expect(view.run?.proposals.map((row) => [row.position, row.total])).toEqual(
      [
        [1, 2],
        [2, 2],
      ],
    );
  });

  it("is committed for new conditions and updated for changed ones", () => {
    const receipt = makeDecisionReceipt();
    const session = withSettlementReceipt(
      withDecisionReceipt(EMPTY_DECISION_SESSION, receipt),
      makeSettlementReceipt({ decisionIds: [receipt.decisionId] }),
    );
    const created = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun(),
      session,
    });
    expect(created.run?.phase).toBe("committed");
    expect(created.run?.committedCount).toBe(1);

    const updated = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis({ conditionEntries: [makeCondition()] }),
      run: makeRun({
        proposals: [
          makeProposal({
            changeKind: "update",
            targetConditionId: CONDITION_ID,
          }),
        ],
      }),
      session,
    });
    expect(updated.run?.phase).toBe("updated");
  });

  it("is closed without change after a no-change receipt", () => {
    const view = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun(),
      session: withSettlementReceipt(
        EMPTY_DECISION_SESSION,
        makeSettlementReceipt({
          outcome: "closed_without_change",
          revision: 2,
          diagnosisRevisionId: undefined,
        }),
      ),
    });
    expect(view.run?.phase).toBe("closed_without_change");
    expect(view.run?.committedCount).toBe(0);
  });

  it("is safety stopped, and workspace Safety review keeps it read-only", () => {
    const stopped = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun(),
      session: withCommandError(EMPTY_DECISION_SESSION, RUN_ID, safety),
    });
    expect(stopped.run?.phase).toBe("safety_stopped");

    const review = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun(),
      safetyReview: true,
    });
    expect(review.safetyReview).toBe(true);
    expect(review.run?.phase).toBe("deciding");
    expect(review.run?.canDecide).toBe(false);
    expect(review.run?.proposals.some((row) => row.selectable)).toBe(false);
  });
});

describe("Penpot serial stepper", () => {
  it("walks proposals one at a time over local choices", () => {
    const run = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun(),
    }).run!;
    const first = projectDiagnosisStepper(run, {});
    expect(first).toMatchObject({
      step: 1,
      total: 2,
      isLast: false,
      anyAccepted: false,
    });
    expect(first.current?.proposalId).toBe(PROPOSAL_A);
    expect(first.next?.proposalId).toBe(PROPOSAL_B);
    expect(first.current).toMatchObject({ kindLabel: "新建", kindTone: "ok" });

    const second = projectDiagnosisStepper(run, { [PROPOSAL_A]: "reject" });
    expect(second).toMatchObject({
      step: 2,
      isLast: true,
      anyAccepted: false,
      canSubmitEarly: false,
    });
    expect(second.decided).toEqual([
      expect.objectContaining({ choice: "reject", changeable: true }),
    ]);
    expect(second.current?.proposalId).toBe(PROPOSAL_B);
  });

  it("treats recorded decision receipts as fixed choices", () => {
    const run = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun(),
      session: withDecisionReceipt(
        EMPTY_DECISION_SESSION,
        makeDecisionReceipt(),
      ),
    }).run!;
    const stepper = projectDiagnosisStepper(run, {});
    expect(stepper.decided).toEqual([
      expect.objectContaining({ choice: "accept", changeable: false }),
    ]);
    expect(stepper.anyAccepted).toBe(true);
  });

  it("labels the toolbar as analysis until a committed receipt", () => {
    const pending = withDecisionReceipt(
      EMPTY_DECISION_SESSION,
      makeDecisionReceipt(),
    );
    expect(
      diagnosisHeaderMode(
        projectNativeDiagnosisWindow({
          diagnosis: makeDiagnosis(),
          run: makeRun(),
          session: pending,
        }),
      ),
    ).toBe("analysis");
    expect(
      diagnosisHeaderMode(
        projectNativeDiagnosisWindow({
          diagnosis: makeDiagnosis(),
          run: makeRun(),
          session: withSettlementReceipt(pending, makeSettlementReceipt()),
        }),
      ),
    ).toBe("receipt");
    expect(diagnosisHeaderMode({ run: null })).toBeNull();
  });
});
