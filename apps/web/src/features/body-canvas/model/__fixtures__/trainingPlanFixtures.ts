import type {
  PlanProposalRecord,
  TrainingPlanRecord,
} from "@/features/training/services/trainingService";

export const PLAN_ID = "11111111-1111-4111-8111-111111111111";
export const REVISION_ID = "22222222-2222-4222-8222-222222222222";
export const NEXT_REVISION_ID = "33333333-3333-4333-8333-333333333333";
export const PROPOSAL_ID = "44444444-4444-4444-8444-444444444444";
export const CREATE_PROPOSAL_ID = "55555555-5555-4555-8555-555555555555";
export const NOW = new Date("2026-10-18T12:00:00+08:00");

type Content = TrainingPlanRecord["currentContent"];
type Exercise =
  Content["sessions"][number]["stages"][number]["exercises"][number];

export function exercise(
  overrides: Partial<Exercise> &
    Pick<Exercise, "planExerciseId" | "nameSnapshot">,
): Exercise {
  return {
    exerciseId: overrides.planExerciseId,
    sets: 3,
    target: { kind: "repetitions", min: 12 },
    ...overrides,
  };
}

export function makeContent(
  main: Exercise[] = [
    exercise({
      planExerciseId: "bridge",
      nameSnapshot: "臀桥",
      restSeconds: 45,
    }),
    exercise({
      planExerciseId: "wall-sit",
      nameSnapshot: "靠墙静蹲",
      target: { kind: "duration", seconds: 30 },
      restSeconds: 60,
    }),
    exercise({
      planExerciseId: "band-walk",
      nameSnapshot: "弹力带侧走",
      sets: 2,
      restSeconds: 45,
      side: "alternating",
    }),
  ],
): Content {
  return {
    sessions: [
      {
        templateId: "a",
        title: "A · 基础课",
        stages: [
          {
            phase: "warmup",
            exercises: [
              exercise({
                planExerciseId: "cat-cow",
                nameSnapshot: "猫牛式",
                sets: 1,
                target: { kind: "repetitions", min: 10 },
              }),
            ],
          },
          { phase: "main", exercises: main },
          {
            phase: "cooldown",
            exercises: [
              exercise({
                planExerciseId: "quad",
                nameSnapshot: "股四头肌拉伸",
                sets: 1,
                target: { kind: "duration", seconds: 30 },
                side: "both",
              }),
            ],
          },
        ],
      },
      {
        templateId: "b",
        title: "B · 进阶课",
        stages: [
          {
            phase: "main",
            exercises: [
              exercise({ planExerciseId: "lunge", nameSnapshot: "弓步蹲" }),
            ],
          },
        ],
      },
    ],
    frequency: { sessionsPerWeek: 3 },
    schedule: { mode: "frequency_only" },
    constraints: [],
  };
}

export function makePlan(
  overrides: Partial<TrainingPlanRecord> = {},
): TrainingPlanRecord {
  return {
    planId: PLAN_ID,
    title: "日常活动计划",
    goal: "恢复日常活动能力",
    status: "active",
    statusRevision: 0,
    currentAcceptedRevisionId: REVISION_ID,
    relatedConditionIds: [],
    relatedBodyRefs: [],
    currentContent: makeContent(),
    review: { recommended: false, sessionFeedbackCount: 0 },
    executionGate: {
      result: "allow",
      permitsExecution: true,
      reasons: [],
      matchedRestrictionIds: [],
      requiresReview: false,
      urgentCare: false,
      safetyRevision: 4,
    },
    createdAt: "2026-10-08T08:00:00+08:00",
    ...overrides,
  };
}

/** The Penpot Proposal board: 臀桥 3×12 → 3×15, add 死虫式. */
export function updatedContent(): Content {
  return makeContent([
    exercise({
      planExerciseId: "bridge",
      nameSnapshot: "臀桥",
      target: { kind: "repetitions", min: 15 },
      restSeconds: 45,
    }),
    exercise({
      planExerciseId: "dead-bug",
      nameSnapshot: "死虫式",
      sets: 2,
      target: { kind: "repetitions", min: 10 },
      restSeconds: 30,
    }),
    exercise({
      planExerciseId: "wall-sit",
      nameSnapshot: "靠墙静蹲",
      target: { kind: "duration", seconds: 30 },
      restSeconds: 60,
    }),
    exercise({
      planExerciseId: "band-walk",
      nameSnapshot: "弹力带侧走",
      sets: 2,
      restSeconds: 45,
      side: "alternating",
    }),
  ]);
}

export function makeProposal(
  overrides: Partial<PlanProposalRecord> = {},
): PlanProposalRecord {
  return {
    proposalId: PROPOSAL_ID,
    kind: "update",
    state: "pending",
    initiatedBy: "agent",
    planId: PLAN_ID,
    basedOnRevisionId: REVISION_ID,
    draft: {
      title: "日常活动计划",
      goal: "恢复日常活动能力",
      relatedConditionIds: [],
      relatedBodyRefs: [],
      content: updatedContent(),
    },
    contentFingerprint: "a".repeat(64),
    createdAt: "2026-10-17T12:00:00+08:00",
    expiresAt: "2026-10-24T12:00:00+08:00",
    ...overrides,
  };
}
