import { describe, expect, it } from "vitest";

import type { ZodType } from "zod";

import * as vm from "./presentation-v2";

import * as f from "./presentation-v2.fixtures";

const fixtures: Record<string, unknown> = f.presentationFixtures;

const error = {
  code: "network",
  message: "连接暂时中断，已保留输入",
  retry: "same_request",
};

const binding = { idempotencyKey: "request-demo", expectedRevision: "1" };

describe("V2 presentation contract, not transport or domain implementation", () => {
  it("covers all sixteen queries with synthetic, parseable content", () => {
    expect(Object.keys(vm.presentationV2Queries).sort()).toEqual(
      Object.keys(fixtures).sort(),
    );
    expect(Object.keys(fixtures)).toHaveLength(16);
  });
  for (const [name, schema] of Object.entries(vm.presentationV2Queries)) {
    it(`${name}: accepts the documented ready fixture`, () => {
      schema.parse(fixtures[name]);
    });
    for (const value of [
      { state: "loading" },
      { state: "empty", meta: f.meta, explanation: "当前没有记录" },
      { state: "offline" },
      { state: "recovering" },
      { state: "failed", error },
      { state: "permission_denied", message: "无权查看这项信息" },
    ])
      it(`${name}: distinguishes ${value.state}`, () => {
        schema.parse(value);
      });
    it(`${name}: does not leak cached data on permission denial`, () => {
      expect(
        schema.safeParse({
          state: "permission_denied",
          message: "拒绝",
          data: fixtures[name],
        }).success,
      ).toBe(false);
    });
  }
});

type Probe = [string, ZodType, unknown];

const invalid: Probe[] = [
  [
    "asserted is not an active symptom lifecycle",
    vm.bodyFactCard,
    { ...f.fact, assertionStatus: "active" },
  ],
  [
    "unknown event time cannot borrow saved time",
    vm.temporalValue,
    { precision: "unknown", start: f.at },
  ],
  [
    "AI hypothesis cannot become user's hypothesis",
    vm.bodyHypothesisCard,
    { ...f.hypothesis, actor: "agent" },
  ],
  [
    "unadmitted OCR cannot become Observation",
    vm.bodyObservationCard,
    {
      ...f.observation,
      source: {
        ...f.source,
        attribution: "document",
        historicalAdmission: "needs_review",
      },
    },
  ],
  [
    "Episode ending needs a concrete basis",
    vm.episodeDetail,
    {
      ...f.episode,
      projection: {
        state: "ended",
        reason: "explicit_resolution",
        basisFactIds: [],
        end: { precision: "day", start: "2026-10-12" },
      },
    },
  ],
  [
    "no fake current symptom from region selection",
    vm.regionSummaryVM,
    f.ready({ ...f.presentationFixtures["body.region"], pain: true }),
  ],
  [
    "Ask User cannot coexist with ordinary composer",
    vm.chatVM,
    f.ready({
      ...f.chat,
      composer: {
        kind: "text",
        draft: "",
        assetIds: [],
        selectedRegionIds: [],
        canSend: f.enabled,
      },
    }),
  ],
  [
    "Ask User must retain original Run",
    vm.chatVM,
    f.ready({
      ...f.chat,
      composer: {
        ...f.chat.composer,
        interaction: { ...f.interaction, runId: "new-run" },
      },
    }),
  ],
  [
    "Ask User must retain interaction identity",
    vm.chatVM,
    f.ready({
      ...f.chat,
      composer: {
        ...f.chat.composer,
        interaction: { ...f.interaction, interactionId: "other" },
      },
    }),
  ],
  [
    "rolling history must not renumber or duplicate seq",
    vm.chatVM,
    f.ready({ ...f.chat, messages: [...f.chat.messages, ...f.chat.messages] }),
  ],
  [
    "no conversation-list field in single Chat",
    vm.chatVM,
    f.ready({ ...f.chat, conversations: [] }),
  ],
  [
    "clinical confidence is not a percentage",
    vm.diagnosisAssessment,
    { ...f.conditionContent.assessment, level: 0.8 },
  ],
  [
    "clinical diagnosis is not AI assessment level",
    vm.diagnosisAssessment,
    { ...f.conditionContent.assessment, level: "clinical_diagnosis" },
  ],
  [
    "inactive is not included in current adopted list",
    vm.diagnosisVM,
    f.ready({
      ...f.diagnosis,
      adoptedConditions: [{ ...f.condition, disposition: "inactive" }],
    }),
  ],
  [
    "Dock count cannot include pending proposals",
    vm.diagnosisVM,
    f.ready({ ...f.diagnosis, adoptedConditionCount: 2 }),
  ],
  [
    "Diagnosis has no generic lifecycle",
    vm.conditionEntry,
    { ...f.condition, lifecycle: "resolved" },
  ],
  [
    "proposal has no invented Diagnosis TTL",
    vm.conditionProposalCard,
    { ...f.proposal, expiresAt: "2026-10-20T06:00:00Z" },
  ],
  [
    "confirmation binds content fingerprint",
    vm.diagnosisConfirmationVM,
    f.ready({
      ...f.confirmation,
      consent: { ...f.consent, contentFingerprint: "other-content" },
    }),
  ],
  [
    "decision recorded is not committed",
    vm.commandOutcome,
    {
      state: "decision_recorded",
      decisionId: "d",
      runId: "r",
      proposalId: "p",
      diagnosisChanged: true,
    },
  ],
  [
    "tool completion is not DomainCommitReceipt",
    vm.commandOutcome,
    { state: "committed", toolCallId: "tool", refresh: ["diagnosis.current"] },
  ],
  [
    "urgent terminal run cannot resume or commit",
    vm.diagnosisConfirmationVM,
    f.ready({
      runId: "r",
      basedOnDiagnosisRevision: "1",
      decisions: [],
      state: "safety_blocked",
      explanation: "停止分析",
      restrictionIds: ["restriction"],
      canResumeOldRun: true,
      diagnosisChanged: false,
    }),
  ],
  [
    "Draft cannot be an accepted plan status",
    vm.acceptedPlan,
    { ...f.plan, status: "draft" },
  ],
  [
    "Safety must not become plan status",
    vm.acceptedPlan,
    { ...f.plan, status: "safety_hold" },
  ],
  [
    "training proposal cannot exceed seven days",
    vm.trainingProposalVM,
    f.ready({ ...f.planProposal, expiresAt: "2026-10-21T06:00:00Z" }),
  ],
  [
    "frequency does not manufacture missed days",
    vm.planContent,
    { ...f.content, schedule: { mode: "frequency_only", missedDays: 1 } },
  ],
  [
    "repetition range cannot be inverted",
    vm.exerciseTarget,
    { kind: "repetitions", min: 10, max: 8 },
  ],
  [
    "reused name is not a reused instance ID",
    vm.planContent,
    {
      ...f.content,
      sessions: [
        {
          ...f.template,
          stages: [{ phase: "main", exercises: [f.exercise, f.exercise] }],
        },
      ],
    },
  ],
  [
    "unknown exercise cannot be session cursor",
    vm.trainerVM,
    f.ready({
      ...f.session,
      current: { planExerciseId: "foreign", setNumber: 1 },
    }),
  ],
  [
    "next/demo is not completion command",
    vm.presentationV2Command,
    {
      operation: "trainer.next",
      ...binding,
      sessionLogId: "s",
      completed: true,
    },
  ],
  [
    "set skip must identify its scope precisely",
    vm.trainingCommand,
    {
      operation: "trainer.skip",
      ...binding,
      sessionLogId: "s",
      planExerciseId: "e",
      scope: "set",
    },
  ],
  [
    "urgent cannot expire automatically",
    vm.safetyRestriction,
    {
      ...f.restriction,
      riskLevel: "urgent",
      scope: { kind: "all_training" },
      liftPolicy: "on_expiry",
      validUntil: f.at,
    },
  ],
  [
    "on_expiry cannot omit expiry time",
    vm.safetyRestriction,
    { ...f.restriction, liftPolicy: "on_expiry" },
  ],
  [
    "removed original cannot remain verifiable",
    vm.assetCard,
    { ...f.asset, sourceAvailability: "original_unavailable" },
  ],
  [
    "erased asset cannot resurrect OCR",
    vm.assetCard,
    {
      ...f.asset,
      sourceAvailability: "erased",
      usage: "blocked",
      canOpenOriginal: f.disabled,
    },
  ],
  [
    "erased evidence cannot expose raw snippet",
    vm.evidenceSource,
    { ...f.source, availability: "erased", usage: "blocked" },
  ],
  [
    "health erasure requires scope fingerprint",
    vm.personAssetCommand,
    {
      operation: "asset.erase_related_health_data",
      ...binding,
      assetId: "a",
      scopeId: "s",
      consent: f.consent,
    },
  ],
  [
    "original-only deletion cannot imply all copies erased",
    vm.personErasureVM,
    f.ready({
      state: "completed",
      jobId: "j",
      scope: f.scope,
      newUseBlocked: true,
      steps: [{ category: "original", state: "verified" }],
      verificationReceiptId: "v",
      completedAt: f.at,
    }),
  ],
  [
    "failed erasure keeps source blocked",
    vm.personErasureVM,
    f.ready({ ...f.erasure, newUseBlocked: false }),
  ],
  [
    "Profile must not duplicate health facts",
    vm.personIdentityVM,
    f.ready({ profile: { height: 175 }, updatedAt: f.at, canEdit: f.enabled }),
  ],
  [
    "client commands cannot inject another user",
    vm.bodyStateCommand,
    {
      operation: "body.record_statement",
      idempotencyKey: "x",
      statement: "描述",
      bodyRegionIds: [],
      userId: "someone-else",
    },
  ],
];

for (const [label, schema, value] of invalid)
  it(`rejects: ${label}`, () => {
    expect(schema.safeParse(value).success, label).toBe(false);
  });
