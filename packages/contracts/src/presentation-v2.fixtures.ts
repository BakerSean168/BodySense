/** Synthetic contract/design fixtures only. Never use as production fallback data. */
export const at = "2026-10-13T06:00:00Z";
export const enabled = { state: "enabled" };
export const disabled = { state: "disabled", reason: "需要先核验当前状态" };
export const meta = {
  fetchedAt: at,
  sourceVersions: [
    { domain: "body_state", targetId: "body-demo", revision: "6" },
  ],
};
export const ref = {
  sourceId: "source-demo",
  kind: "user_statement_evidence",
  version: "1",
  recordedAt: at,
};
export const source = {
  ref,
  attribution: "user_statement",
  label: "本人陈述",
  excerpt: "昨天有头痛，现在是否持续尚未说明。",
  availability: "original_available",
  currentValidity: "current",
  usage: "allowed",
};
export const fact = {
  factId: "fact-demo",
  statement: "昨天有头痛",
  bodyRegionIds: ["head"],
  source,
  temporal: {
    precision: "day",
    start: "2026-10-12",
    originalExpression: "昨天",
    timeZone: "Asia/Tokyo",
  },
  recordedAt: at,
  assertionStatus: "asserted",
  kind: "symptom",
  details: { presence: "present", symptomType: "头痛" },
};
export const habit = {
  ...fact,
  factId: "habit-demo",
  kind: "lifestyle",
  statement: "通常每周散步三次",
  bodyRegionIds: [],
  temporal: { precision: "unknown" },
  details: { habitType: "散步", frequency: "每周三次" },
};
export const observation = {
  observationId: "observation-demo",
  kind: "用户自报测量",
  value: { rawValue: "60", rawUnit: "kg" },
  source: { ...source, attribution: "user_measurement", label: "自报测量" },
  recordedAt: at,
  assertionStatus: "asserted",
};
export const hypothesis = {
  hypothesisId: "hypothesis-demo",
  statement: "我猜可能与睡眠有关",
  actor: "user",
  source,
  relatedFactIds: [fact.factId],
  relatedObservationIds: [],
  bodyRegionIds: [],
  assertionStatus: "asserted",
  recordedAt: at,
};
export const episode = {
  episodeId: "episode-demo",
  symptomType: "头痛经历",
  bodyRegionIds: ["head"],
  laterality: "unknown",
  identityStatus: "current",
  onset: { precision: "unknown" },
  projection: {
    state: "unknown",
    reason: "insufficient_evidence",
    basisFactIds: [fact.factId],
  },
  links: [
    {
      factId: fact.factId,
      role: "observation",
      associationMethod: "explicit",
      basisSourceIds: [ref.sourceId],
    },
  ],
  basedOnRevision: "6",
  relations: [],
  history: [
    {
      eventId: "event-demo",
      recordedAt: at,
      eventTime: fact.temporal,
      description: "记录昨天的陈述；尚未确认现在是否持续",
      sourceRefs: [ref],
    },
  ],
};
export const records = {
  facts: [fact],
  observations: [observation],
  hypotheses: [hypothesis],
  episodes: [episode],
  groups: [],
  nextCursor: null,
  canRecord: enabled,
};
export const consent = {
  interactionId: "interaction-demo",
  runId: "run-demo",
  toolCallId: "tool-demo",
  contentFingerprint: "fingerprint-demo",
  sourceVersions: [{ id: ref.sourceId, version: "1" }],
};
export const interaction = {
  interactionId: consent.interactionId,
  runId: consent.runId,
  toolCallId: consent.toolCallId,
  interactionRevision: "1",
  purpose: "clarification",
  title: "这次与上次是同一段经历吗？",
  questions: [
    {
      questionId: "question-demo",
      label: "请确认描述的时间关系",
      required: true,
      kind: "single_choice",
      choices: [
        { value: "same", label: "一直持续到现在" },
        { value: "recurrence", label: "曾经结束，之后再次出现" },
        { value: "unknown", label: "暂不确定" },
      ],
      allowExplanation: true,
    },
  ],
  draftAnswers: [],
  state: "ready",
  canAnswer: enabled,
};
export const chat = {
  conversationId: "conversation-demo",
  messages: [
    {
      messageId: "message-demo",
      seq: 101,
      role: "user",
      createdAt: at,
      text: "昨天有头痛",
      assetIds: [],
      outcomes: [],
      delivery: "delivered",
    },
  ],
  run: {
    state: "waiting_for_user",
    runId: consent.runId,
    interactionId: consent.interactionId,
  },
  composer: { kind: "interaction", interaction, canStop: enabled },
  lastActivityAt: at,
  connection: "online",
  history: {
    mode: "rolling",
    earlierMessagesPruned: true,
    durableFactsPreserved: true,
  },
};
export const conditionContent = {
  medicalName: "偏头痛",
  assessment: {
    level: "suspected",
    rationale: "仅用于检验信息层级的合成示例，不构成诊断建议。",
    informationGaps: ["仍缺少必要信息"],
    differentialNotes: ["需要区分其他可能性"],
    basedOnRunId: "analysis-demo",
  },
  sourceClaims: [],
  evidenceLinks: [
    {
      evidenceType: "body_fact",
      evidenceId: fact.factId,
      evidenceVersion: "1",
      role: "context",
      rationale: "相关自述，不能单独证明病因",
      analyzedByRunId: "analysis-demo",
      source,
    },
  ],
  causeAnalysis: {
    summary: "目前原因尚未明确",
    rationale: "仅展示有来源的分析，不把相关性当作因果",
  },
  informationGaps: ["发作特征与影响"],
  handlingPlan: {
    summary: "补充信息后再进行正式复核",
    actions: [],
    cautionNotes: [],
  },
};
export const condition = {
  ...conditionContent,
  conditionId: "condition-demo",
  disposition: "adopted",
  latestDecisionId: "decision-demo",
  createdAt: at,
  updatedAt: at,
};
export const diagnosis = {
  diagnosisId: "diagnosis-demo",
  adoptedConditions: [condition],
  adoptedConditionCount: 1,
  selectedConditionId: condition.conditionId,
  relations: [],
  reviewRecommendations: [],
  canRequestAnalysis: enabled,
};
export const proposal = {
  proposalId: "proposal-demo",
  runId: consent.runId,
  contentFingerprint: consent.contentFingerprint,
  evidenceRefs: [ref],
  createdAt: at,
  changeKind: "create",
  content: conditionContent,
};
export const confirmation = {
  runId: consent.runId,
  basedOnDiagnosisRevision: "2",
  decisions: [],
  state: "awaiting_decision",
  current: proposal,
  position: 1,
  total: 2,
  consent,
  diagnosisChanged: false,
};
export const receipt = {
  receiptId: "receipt-demo",
  domain: "diagnosis",
  targetId: diagnosis.diagnosisId,
  revision: "3",
  eventIds: ["event-commit-demo"],
  committedAt: at,
};
export const gate = {
  result: "allow",
  matchedRestrictionIds: [],
  reasons: [],
  requiresReview: false,
  evaluatedAt: at,
  safetyRevision: "2",
  scopeMatch: "verified",
  freshness: "current",
};
export const restriction = {
  restrictionId: "restriction-demo",
  riskLevel: "restricted",
  reason: "示例限制：当前动作需先复核",
  scope: { kind: "exercise", exerciseIds: ["exercise-demo"] },
  validFrom: at,
  liftPolicy: "user_reverification",
  liftConditions: ["完成针对该限制的复核"],
  sourceRefs: [ref],
  effect: "active",
};
export const exercise = {
  planExerciseId: "entry-demo",
  exerciseId: "exercise-demo",
  name: "示例动作",
  sets: 2,
  target: { kind: "repetitions", min: 8 },
  restSeconds: 30,
};
export const template = {
  templateId: "template-demo",
  title: "基础课",
  stages: [{ phase: "main", exercises: [exercise] }],
};
export const content = {
  sessions: [template],
  frequency: { sessionsPerWeek: 3 },
  schedule: { mode: "frequency_only" },
  constraints: ["合成展示数据，不用于实际训练"],
};
export const plan = {
  planId: "plan-demo",
  title: "日常活动计划",
  goal: "展示完整计划与独立安全状态",
  relatedConditionIds: [],
  relatedBodyRefs: [],
  status: "active",
  acceptedRevisionId: "plan-revision-demo",
  content,
  reviewRecommendations: [],
  gate,
};
export const planProposal = {
  proposalId: "plan-proposal-demo",
  contentFingerprint: "plan-fingerprint-demo",
  rationale: "计划草案示例，接受前不会进入训练",
  evidenceRefs: [ref],
  createdAt: at,
  expiresAt: "2026-10-20T06:00:00Z",
  state: "pending",
  canAccept: enabled,
  kind: "create",
  title: plan.title,
  goal: plan.goal,
  relatedConditionIds: [],
  relatedBodyRefs: [],
  preview: content,
};
export const session = {
  sessionLogId: "session-demo",
  planId: plan.planId,
  planRevisionId: plan.acceptedRevisionId,
  templateId: template.templateId,
  startedAt: at,
  trainingDate: "2026-10-13",
  timeZoneSnapshot: "Asia/Tokyo",
  templateSnapshot: template,
  actions: [],
  current: { planExerciseId: exercise.planExerciseId, setNumber: 1 },
  verifiedElapsedSeconds: 0,
  gate,
  state: "paused",
  pauseReason: "recovery",
};
export const asset = {
  assetId: "asset-demo",
  facets: ["media", "report"],
  originalName: "示例报告.png",
  mimeType: "image/png",
  byteSize: 120000,
  state: "available",
  sourceAvailability: "original_available",
  createdAt: at,
  extractions: [
    {
      runId: "extraction-demo",
      mechanismVersion: "fixture-v1",
      processingState: "succeeded",
      canRetry: enabled,
    },
  ],
  candidates: [
    {
      candidateId: "candidate-demo",
      extractionRunId: "extraction-demo",
      sourceLocator: "第 1 页",
      admissibility: "needs_review",
      summary: "尚未准入的候选内容",
    },
  ],
  usage: "allowed",
  canOpenOriginal: enabled,
};
export const scope = {
  scopeId: "scope-demo",
  scopeFingerprint: "scope-fingerprint-demo",
  assetIds: [asset.assetId],
  affected: [
    { category: "original", count: 1 },
    { category: "checkpoint", count: 1 },
  ],
  retainedSummary: ["保留无敏感原文的最小操作回执"],
  independentSourceIdsPreserved: ["source-independent-demo"],
};
export const erasure = {
  state: "retryable_failed",
  jobId: "erasure-demo",
  scope,
  newUseBlocked: true,
  steps: [
    { category: "original", state: "verified" },
    {
      category: "checkpoint",
      state: "failed",
      explanation: "关联副本清理尚未核验，仍禁止使用该来源",
    },
  ],
  canRetry: enabled,
};
export function ready(data: unknown) {
  return { state: "ready", meta, data };
}
export const presentationFixtures = {
  "person.records": ready(records),
  "person.habits": ready({
    habits: [habit],
    nextCursor: null,
    canRecord: enabled,
  }),
  "body.episode": ready(episode),
  "body.region": ready({
    regionIds: ["head"],
    selectionOnly: true,
    factIds: [fact.factId],
    episodeIds: [episode.episodeId],
    observationIds: [],
    hypothesisIds: [],
    adoptedConditionIds: [condition.conditionId],
  }),
  "chat.current": ready(chat),
  "diagnosis.current": ready(diagnosis),
  "diagnosis.history": ready({
    entries: [
      {
        at,
        label: "曾经采纳",
        condition: { ...condition, disposition: "inactive" },
        reason: "用户撤回采纳，不代表医学排除",
        decisionId: "decision-history-demo",
      },
    ],
    nextCursor: null,
  }),
  "diagnosis.confirmation": ready(confirmation),
  "training.plans": ready({
    plans: [plan],
    selectedPlanId: plan.planId,
    selectedTemplateId: template.templateId,
  }),
  "training.proposal": ready(planProposal),
  "trainer.current": ready(session),
  "safety.current": ready({
    restrictions: [restriction],
    gate: {
      ...gate,
      result: "block",
      matchedRestrictionIds: [restriction.restrictionId],
      reasons: [restriction.reason],
      requiresReview: true,
    },
  }),
  "person.identity": ready({ profile: {}, updatedAt: at, canEdit: enabled }),
  "person.equipment": ready({
    equipment: [
      {
        equipmentId: "equipment-demo",
        displayName: "弹力带",
        accessKind: "owned",
        availability: "unavailable",
        updatedAt: at,
        affectedPlanIds: [],
      },
    ],
    nextCursor: null,
    canEdit: enabled,
  }),
  "person.assets": ready({
    facet: "report",
    assets: [asset],
    nextCursor: null,
    canUpload: enabled,
  }),
  "person.erasure": ready(erasure),
};
