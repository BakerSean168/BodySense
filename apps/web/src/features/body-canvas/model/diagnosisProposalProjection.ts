import type {
  NativeDiagnosisCondition as NativeDiagnosisConditionOutput,
  NativeDiagnosisProposal as NativeDiagnosisRunProposalOutput,
} from "@/features/consultation/services/nativeDiagnosisService";
import type {
  ConditionCertainty,
  ConfirmedCondition,
} from "./confirmedConditions";
import type { ProposalRow } from "./diagnosisDecisionTypes";
import { projectConditionReview } from "./diagnosisReview";

const LEVEL_LABELS: Record<string, string> = {
  suspected: "疑似",
  likely: "较倾向",
};

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function text(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

function texts(values: readonly string[] | undefined): string[] {
  return (values ?? []).flatMap((value) => {
    const trimmed = value.trim();
    return trimmed ? [trimmed] : [];
  });
}

function certaintyOf(
  condition: NativeDiagnosisConditionOutput,
): ConditionCertainty {
  const kinds = condition.sourceClaimRefs.map((claim) => claim.sourceKind);
  if (kinds.includes("clinical_document")) return "clinically_documented";
  if (kinds.includes("user_reported_clinical"))
    return "user_reported_clinical_diagnosis";
  return "user_confirmed_agent_analysis";
}

export function projectAdoptedCondition(
  condition: NativeDiagnosisConditionOutput,
): ConfirmedCondition {
  const assessment = condition.assessment;
  const handling = condition.handlingPlan;
  const hasHandling =
    !!text(handling.summary) ||
    texts(handling.actions).length > 0 ||
    texts(handling.cautionNotes).length > 0;
  return {
    conditionId: condition.conditionId,
    name: condition.medicalName,
    certainty: certaintyOf(condition),
    lifecycle: "active",
    cause: {
      summary: text(condition.causeAnalysis.summary),
      basis: text(condition.causeAnalysis.rationale),
      basisFactIds: condition.evidenceLinks
        .filter((link) => link.role === "supporting")
        .map((link) => link.evidenceId),
      evidenceStrength: assessment
        ? (LEVEL_LABELS[assessment.level] ?? null)
        : null,
      counterEvidenceCount: condition.evidenceLinks.filter(
        (link) => link.role === "counter",
      ).length,
      informationGaps: texts(condition.informationGaps),
      differential: assessment
        ? texts(assessment.differentialNotes).join("；") || null
        : null,
    },
    handling: hasHandling
      ? {
          state: "condition_level",
          summary: text(handling.summary) ?? undefined,
          actions: texts(handling.actions),
          cautionNotes: texts(handling.cautionNotes),
        }
      : { state: "not_confirmed" },
    safetyNotes: [],
    disposition: condition.disposition,
    review: condition.freshness ? projectConditionReview(condition) : undefined,
  };
}

function strings(value: unknown): string[] {
  return Array.isArray(value)
    ? value.flatMap((item) => {
        const trimmed = text(item);
        return trimmed ? [trimmed] : [];
      })
    : [];
}

export function proposalKind(
  proposal: NativeDiagnosisRunProposalOutput,
): Pick<ProposalRow, "kindLabel" | "kindTone"> {
  switch (proposal.changeKind) {
    case "create":
      return { kindLabel: "新建", kindTone: "ok" };
    case "update":
      return { kindLabel: "更新", kindTone: "ok" };
    case "set_disposition":
      return text(record(proposal.content)?.disposition) === "inactive"
        ? { kindLabel: "停用", kindTone: "idle" }
        : { kindLabel: "调整状态", kindTone: "idle" };
    case "update_relation":
      return { kindLabel: "调整关联", kindTone: "idle" };
  }
}

export function proposalDetail(
  proposal: NativeDiagnosisRunProposalOutput,
): Pick<ProposalRow, "supportCount" | "counterCount" | "rationale" | "gaps"> {
  const entry = record(record(proposal.content)?.proposedEntry);
  const cause = record(entry?.causeAnalysis);
  const assessment = record(entry?.assessment);
  const gaps = strings(entry?.informationGaps);
  return {
    supportCount: proposal.evidenceRefs.length,
    counterCount: proposal.counterEvidenceRefs.length,
    rationale: text(cause?.rationale) ?? text(assessment?.rationale),
    gaps: gaps.length > 0 ? gaps : strings(assessment?.informationGaps),
  };
}

export function describeProposal(
  proposal: NativeDiagnosisRunProposalOutput,
  conditions: readonly NativeDiagnosisConditionOutput[],
): Pick<ProposalRow, "title" | "summary" | "levelLabel"> {
  const content = record(proposal.content);
  const entry = record(content?.proposedEntry);
  const target = conditions.find(
    (condition) => condition.conditionId === proposal.targetConditionId,
  );
  const targetName = target?.medicalName ?? "现有病情";
  const assessment = record(entry?.assessment);
  const level = text(assessment?.level);
  const cause = record(entry?.causeAnalysis);
  switch (proposal.changeKind) {
    case "create":
      return {
        title: text(entry?.medicalName) ?? "新的病情提案",
        summary: text(cause?.summary),
        levelLabel: level ? (LEVEL_LABELS[level] ?? null) : null,
      };
    // Penpot: the kind chip (更新 / 停用 / 调整关联) carries the change; the
    // title stays the condition's own name.
    case "update":
      return { title: targetName, summary: null, levelLabel: null };
    case "set_disposition":
      return {
        title: targetName,
        summary: text(content?.dispositionReason),
        levelLabel: null,
      };
    case "update_relation":
      return { title: targetName, summary: null, levelLabel: null };
  }
}
