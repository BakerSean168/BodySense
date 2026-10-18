import type {
  DiagnosisCandidate,
  DiagnosisCandidateAssessmentState,
} from "@/features/consultation/types/consultation";
import type {
  HealthWorkspace,
  WorkspaceDiagnosis,
} from "@/features/workspace/types/workspace";

/**
 * Diagnosis window projection.
 *
 * The product contract (docs/product/domain.md §3) defines a single current
 * comprehensive Diagnosis made of stable `ConditionEntry` items that may only
 * contain **confirmed** conditions. The shipped API still exposes the analysis
 * projection (`candidates` + `candidate_assessments`), so this module maps the
 * real confirmation state onto the UI shape and marks every field the backend
 * cannot yet supply as an explicit gap instead of inventing content.
 *
 * Contract gaps recorded here (do not paper over them in the UI):
 * - no per-condition `lifecycle`; a condition confirmed in the current analysis
 *   is treated as active, and historical analyses are out of scope;
 * - no `certainty` discriminator; user confirmation of an agent analysis is
 *   never presented as clinical documentation;
 * - no condition-scoped `handlingPlan`; independent TrainingPlans carry no
 *   verified Diagnosis link yet, so handling stays `not_confirmed`.
 */

export type ConditionCertainty =
  | "clinically_documented"
  | "user_reported_clinical_diagnosis"
  | "user_confirmed_agent_analysis";

export interface ConditionCause {
  /** Explicit cause/explanation text, or null when the analysis gave none. */
  summary: string | null;
  basis: string | null;
  basisFactIds: string[];
  evidenceStrength: string | null;
  counterEvidenceCount: number;
  /** Evidence boundary: what would still change the conclusion. */
  informationGaps: string[];
  differential: string | null;
}

export interface ConditionHandling {
  /**
   * `condition_level` is the handling plan stored on an adopted V2
   * ConditionEntry; `plan_level` is the legacy accepted-plan summary.
   */
  state: "not_confirmed" | "plan_level" | "condition_level";
  summary?: string;
  actions?: string[];
  cautionNotes?: string[];
  /** Plan-level handling is tied to a specific accepted plan, never to a condition. */
  planId?: string;
  planGoal?: string;
}

export interface ConfirmedCondition {
  conditionId: string;
  name: string;
  certainty: ConditionCertainty;
  lifecycle: "active";
  cause: ConditionCause;
  handling: ConditionHandling;
  safetyNotes: string[];
}

export interface PendingConditionProposal {
  conditionId: string;
  name: string;
}

export interface DiagnosisView {
  analysisId: string | null;
  /** Only confirmed conditions belong to the current comprehensive diagnosis. */
  confirmed: ConfirmedCondition[];
  /** Proposals that are explicitly NOT part of the diagnosis list. */
  pendingProposals: PendingConditionProposal[];
  summary: string | null;
  status: WorkspaceDiagnosis["status"] | null;
  /** True when a re-analysis should be requested before trusting the list. */
  needsReview: boolean;
  reviewReasons: string[];
  /** True when the analysis itself blocked publication for safety reasons. */
  safetyBlocked: boolean;
}

export const EMPTY_DIAGNOSIS_VIEW: DiagnosisView = {
  analysisId: null,
  confirmed: [],
  pendingProposals: [],
  summary: null,
  status: null,
  needsReview: false,
  reviewReasons: [],
  safetyBlocked: false,
};

function nonEmpty(value: string | undefined | null): string | null {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
}

function stringList(values: readonly (string | undefined | null)[] | undefined): string[] {
  return (values ?? []).flatMap((value) => {
    const trimmed = value?.trim();
    return trimmed ? [trimmed] : [];
  });
}

function assessmentFor(
  diagnosis: WorkspaceDiagnosis,
  candidate: DiagnosisCandidate,
): DiagnosisCandidateAssessmentState | null {
  if (!candidate.candidate_id) return null;
  const match = diagnosis.candidate_assessments.find(
    (assessment) => assessment.candidate_id === candidate.candidate_id,
  );
  return match?.state ?? null;
}

export function projectDiagnosisView(
  // Kept for callers: no workspace field links a plan to a condition today.
  _workspace: HealthWorkspace | undefined,
  diagnosis: WorkspaceDiagnosis | null | undefined,
): DiagnosisView {
  if (!diagnosis) return EMPTY_DIAGNOSIS_VIEW;

  const confirmed: ConfirmedCondition[] = [];
  const pendingProposals: PendingConditionProposal[] = [];

  for (const candidate of diagnosis.candidates) {
    const assessment = assessmentFor(diagnosis, candidate);
    const conditionId = candidate.candidate_id ?? "";
    if (!conditionId) continue;

    if (assessment === "confirmed") {
      confirmed.push({
        conditionId,
        name: candidate.name,
        // The API has no certainty discriminator. A user-confirmed agent
        // analysis must never be presented as a clinician's diagnosis.
        certainty: "user_confirmed_agent_analysis",
        lifecycle: "active",
        cause: {
          summary:
            nonEmpty(candidate.reasoning_summary) ?? nonEmpty(candidate.basis),
          basis: nonEmpty(candidate.basis),
          basisFactIds: stringList(candidate.basis_fact_ids),
          evidenceStrength: nonEmpty(candidate.evidence_strength),
          counterEvidenceCount: (candidate.counterevidence_ids ?? []).length,
          informationGaps: stringList(candidate.missing_information),
          differential: nonEmpty(candidate.differential),
        },
        handling: { state: "not_confirmed" },
        safetyNotes: stringList(candidate.safety_notes),
      });
      continue;
    }

    // Unconfirmed candidates stay out of the diagnosis list. They are only
    // counted so the user knows a Chat confirmation is still pending.
    if (assessment === "unsure" || assessment === null) {
      pendingProposals.push({ conditionId, name: candidate.name });
    }
  }

  const reviewReasons = (diagnosis.freshness?.reasons ?? []).flatMap((reason) =>
    reason.message ? [reason.message] : [],
  );

  return {
    analysisId: diagnosis.analysis_id,
    confirmed,
    pendingProposals,
    summary: nonEmpty(diagnosis.summary),
    status: diagnosis.status,
    needsReview:
      diagnosis.freshness?.state === "stale" ||
      diagnosis.freshness?.state === "potentially_stale",
    reviewReasons,
    safetyBlocked: diagnosis.status === "safety_blocked",
  };
}

export const CERTAINTY_LABELS: Record<ConditionCertainty, string> = {
  clinically_documented: "有医疗材料支持",
  user_reported_clinical_diagnosis: "用户转述的医生诊断",
  user_confirmed_agent_analysis: "你已确认的分析结果",
};
