import { describe, expect, it } from "vitest";

import { ApiRequestError } from "@/lib/api-client";

import {
  EMPTY_DECISION_SESSION,
  diagnosisHeaderMode,
  projectDiagnosisStepper,
  projectNativeDiagnosisWindow,
  withCommandError,
  withDecisionReceipt,
  withSettlementReceipt,
} from "./diagnosisDecisionViewModel";

import {
  CONDITION_ID,
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

const safety = new ApiRequestError("urgent", 409, "SAFETY_BLOCKED");

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
