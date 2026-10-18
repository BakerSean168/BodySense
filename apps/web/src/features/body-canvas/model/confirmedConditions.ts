/**
 * Condition shapes rendered by the Diagnosis window.
 *
 * The product contract (docs/product/domain.md §3) defines a single current
 * comprehensive Diagnosis made of stable `ConditionEntry` items that may only
 * contain **confirmed** conditions. `diagnosisDecisionViewModel.ts` projects the
 * native `GET /diagnosis` entries onto these shapes; nothing else feeds them.
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
  /** `condition_level` is the handling plan stored on an adopted V2 ConditionEntry. */
  state: "not_confirmed" | "condition_level";
  summary?: string;
  actions?: string[];
  cautionNotes?: string[];
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

export const CERTAINTY_LABELS: Record<ConditionCertainty, string> = {
  clinically_documented: "有医疗材料支持",
  user_reported_clinical_diagnosis: "用户转述的医生诊断",
  user_confirmed_agent_analysis: "你已确认的分析结果",
};
