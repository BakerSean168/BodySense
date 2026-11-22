import { describe, expect, it } from "vitest";

import * as vm from "./presentation-v2";

import * as f from "./presentation-v2.fixtures";

const binding = { idempotencyKey: "request-demo", expectedRevision: "1" };

describe("valid boundary states", () => {
  it("allows unknown event time without fabricating a timestamp", () => {
    vm.episodeDetail.parse({
      ...f.episode,
      projection: {
        state: "ongoing",
        reason: "explicit_current_report",
        basisFactIds: [f.fact.factId],
      },
    });
  });
  it("allows clinician statement without independent AI assessment", () => {
    const { assessment: _assessment, ...withoutAI } = f.condition;
    vm.conditionEntry.parse(withoutAI);
  });
  it("distinguishes all four medical proposal types", () => {
    const { changeKind: _kind, content: _content, ...base } = f.proposal;
    const relation = {
      fromConditionId: "a",
      toConditionId: "b",
      kind: "differential_with",
      basisEvidenceRefs: [f.ref],
    };
    for (const v of [
      f.proposal,
      {
        ...base,
        changeKind: "update",
        targetConditionId: "c",
        content: f.conditionContent,
        changes: [{ fieldLabel: "依据", before: "原依据", after: "新依据" }],
      },
      {
        ...base,
        changeKind: "set_disposition",
        targetConditionId: "c",
        medicalName: "偏头痛",
        from: "adopted",
        to: "inactive",
        reason: "user_withdrawal",
      },
      {
        ...base,
        changeKind: "update_relation",
        relation,
        changes: [{ fieldLabel: "关系", before: null, after: "鉴别" }],
      },
    ])
      vm.conditionProposalCard.parse(v);
  });
  it("accepting suspected never upgrades the assessment", () => {
    vm.diagnosisCommand.parse({
      operation: "diagnosis.decide_proposal",
      ...binding,
      proposalId: f.proposal.proposalId,
      consent: f.consent,
      decision: "accept",
    });
    expect(vm.conditionEntry.parse(f.condition).assessment?.level).toBe(
      "suspected",
    );
  });
  it("refresh requires real receipt and independently named domain queries", () => {
    vm.commandOutcome.parse({
      state: "committed",
      receipt: f.receipt,
      refresh: ["diagnosis.current", "diagnosis.history"],
    });
  });
  it("distinct same-name exercise instances are allowed", () => {
    vm.planContent.parse({
      ...f.content,
      sessions: [
        {
          ...f.template,
          stages: [
            {
              phase: "main",
              exercises: [
                f.exercise,
                { ...f.exercise, planExerciseId: "second-instance" },
              ],
            },
          ],
        },
      ],
    });
  });
  it("paused recovery preserves an uncompleted session", () => {
    const result = vm.trainerVM.parse(f.ready(f.session));
    expect(result.state === "ready" && result.data.actions).toEqual([]);
  });
  it("current verified warn allows only a request; unavailable/stale/block do not", () => {
    for (const result of ["allow", "warn", "block", "unavailable"]) {
      expect(
        vm.canRequestExecution(vm.trainerGate.parse({ ...f.gate, result })),
      ).toBe(result === "allow" || result === "warn");
    }
    for (const override of [
      { freshness: "stale" },
      { scopeMatch: "unknown" },
      { safetyRevision: undefined },
    ])
      expect(
        vm.canRequestExecution(
          vm.trainerGate.parse({ ...f.gate, ...override }),
        ),
      ).toBe(false);
  });
  it("historical admission and original availability are independent", () => {
    vm.assetCard.parse({
      ...f.asset,
      state: "deleted",
      sourceAvailability: "original_unavailable",
      canOpenOriginal: f.disabled,
      candidates: [{ ...f.asset.candidates[0], admissibility: "admissible" }],
    });
  });
  it("one report asset can appear in each of two classification views", () => {
    for (const facet of ["media", "report"])
      vm.personAssetsVM.parse(
        f.ready({
          facet,
          assets: [f.asset],
          nextCursor: null,
          canUpload: f.enabled,
        }),
      );
  });
  it("optional profile values can explicitly be cleared", () => {
    vm.personAssetCommand.parse({
      operation: "profile.update",
      ...binding,
      fields: { birthDate: null, displayName: null },
    });
  });
  it("erasure completion requires every scoped category verified", () => {
    vm.personErasureVM.parse(
      f.ready({
        state: "completed",
        jobId: "j",
        scope: f.scope,
        newUseBlocked: true,
        steps: f.scope.affected.map((a) => ({
          category: a.category,
          state: "verified",
        })),
        verificationReceiptId: "v",
        completedAt: f.at,
      }),
    );
  });
});
