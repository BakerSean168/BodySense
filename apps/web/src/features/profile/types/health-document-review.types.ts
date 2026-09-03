export type HealthDocumentReviewAction = "confirm" | "correct" | "reject";

export interface DocumentIndicatorSourceRegion {
  source_ref: string;
  page_number?: number;
  bbox?: number[];
}

export interface DocumentIndicatorCandidate {
  indicator_index: number;
  indicator_id: string;
  name: string;
  value?: unknown;
  unit?: string;
  reference_range?: string;
  evidence_admissibility: {
    status: "admissible" | "needs_review" | "rejected";
    policy_revision: string;
    reason_codes?: string[];
  };
  source_refs?: string[];
  source_regions?: DocumentIndicatorSourceRegion[];
}

export interface DocumentIndicatorReviewRecord {
  id: string;
  extraction_run_id: string;
  upload_id: string;
  indicator_index: number;
  indicator_id: string;
  action: HealthDocumentReviewAction;
  reviewed_payload?: Record<string, unknown>;
  note?: string;
  reviewer_user_id: string;
  created_at: string;
  idempotency_key: string;
}

export interface DocumentIndicatorReviewProjection {
  indicator_index: number;
  indicator_id: string;
  candidate: DocumentIndicatorCandidate;
  effective_review?: DocumentIndicatorReviewRecord;
  history?: DocumentIndicatorReviewRecord[];
}

export interface HealthDocumentReviewContext {
  extraction_run_id: string;
  upload_id: string;
  review_candidates: DocumentIndicatorReviewProjection[];
}

export interface AppendHealthDocumentReviewInput {
  indicator_index: number;
  indicator_id: string;
  action: HealthDocumentReviewAction;
  reviewed_payload?: Record<string, unknown>;
  source_refs: string[];
  idempotency_key: string;
  note?: string;
}
