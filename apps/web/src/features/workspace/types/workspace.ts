import type {
  BodyStateSnapshot,
  DiagnosisAnalysis,
} from "@/features/consultation/types/consultation";

export type TreatmentStatus =
  "active" | "review_recommended" | "paused" | "superseded" | "completed";

export type TreatmentAcceptanceState = "proposed" | "accepted" | "rejected";

export interface Intervention {
  id: string;
  user_id: string;
  treatment_id: string;
  treatment_revision_id: string;
  kind: string;
  title: string;
  description: string;
  prescription: Record<string, unknown>;
  position: number;
  status: string;
  started_at?: string | null;
  ended_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface TreatmentPlanContent {
  summary: string;
  goal: string;
  duration_weeks: number;
  interventions: Array<{
    kind: string;
    title: string;
    description: string;
    prescription: Record<string, unknown>;
  }>;
  daily_habits: string[];
  expected_timeline: string;
  warning_signs: string[];
  review_triggers: string[];
  safety_notes: string[];
}

export interface TreatmentRevision {
  id: string;
  treatment_id: string;
  revision: number;
  acceptance_state: TreatmentAcceptanceState;
  lifecycle_state: TreatmentStatus;
  source_body_state_revision: number;
  source_diagnosis_analysis_id: string;
  goal: string;
  duration_weeks: number;
  plan: TreatmentPlanContent;
  user_constraints: Record<string, unknown>;
  evidence_ids: string[];
  governance: Record<string, unknown>;
  change_reason: string;
  created_at: string;
  accepted_at?: string | null;
  interventions: Intervention[];
}

export interface Treatment {
  id: string;
  user_id: string;
  current_revision: number;
  status: TreatmentStatus;
  source_body_state_revision?: number | null;
  source_diagnosis_analysis_id?: string | null;
  status_reasons: Array<{
    code?: string;
    revision?: number;
    change_type?: string;
    concern_key?: string;
    message?: string;
  }>;
  created_at: string;
  updated_at: string;
  current?: TreatmentRevision | null;
}

export interface TrainingExecutionPlan {
  id: string;
  user_id: string;
  consultation_id?: string | null;
  treatment_id?: string | null;
  treatment_revision_id?: string | null;
  status: "active" | "superseded" | string;
  goal: string;
  duration_weeks: number;
  current_week: number;
  phases: Array<Record<string, unknown>>;
  created_at: string;
}

export interface Outcome {
  id: string;
  treatment_id?: string | null;
  treatment_revision_id?: string | null;
  intervention_id?: string | null;
  source_type: string;
  source_key: string;
  kind: string;
  concern_key: string;
  body_region: string;
  value: Record<string, unknown>;
  notes: string;
  association_statement: string;
  causality_level:
    "association_only" | "user_attributed" | "clinician_attributed";
  occurred_at: string;
  body_state_revision?: number | null;
  created_at: string;
}

export interface WorkspaceTrendPoint {
  occurred_at: string;
  source_type: string;
  value: Record<string, unknown>;
  notes?: string;
  causality_level?: string;
}

export interface WorkspaceTrend {
  key: string;
  concern_key: string;
  body_region: string;
  kind: string;
  current_trend: string;
  points: WorkspaceTrendPoint[];
}

export interface WorkspaceCapabilities {
  can_continue_consultation: boolean;
  can_edit_body_state: boolean;
  can_request_diagnosis: boolean;
  can_review_diagnosis: boolean;
  can_generate_treatment: boolean;
  can_accept_treatment: boolean;
  can_execute_treatment: boolean;
  can_record_outcome: boolean;
  can_review_treatment: boolean;
  requires_safety_review: boolean;
  requires_diagnosis_review: boolean;
  requires_treatment_review: boolean;
}

export interface WorkspaceAction {
  kind: string;
  priority: number;
  enabled: boolean;
  reason: string;
  target?: Record<string, unknown>;
}

export interface HealthWorkspace {
  generated_at: string;
  conversation_id?: string | null;
  profile_ready: boolean;
  body_state: BodyStateSnapshot;
  diagnosis?: DiagnosisAnalysis;
  treatment?: Treatment | null;
  training_plan?: TrainingExecutionPlan | null;
  treatment_revisions: TreatmentRevision[];
  recent_outcomes: Outcome[];
  trends: WorkspaceTrend[];
  capabilities: WorkspaceCapabilities;
  actions: WorkspaceAction[];
}
