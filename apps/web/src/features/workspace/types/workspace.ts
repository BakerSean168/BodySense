export interface TrainingExecutionPlan {
  id: string;
  consultation_id?: string | null;
  status: "active" | "superseded" | string;
  goal: string;
  duration_weeks: number;
  current_week: number;
  phases: Array<Record<string, unknown>>;
  created_at: string;
}

export interface Outcome {
  id: string;
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
  can_execute_treatment: boolean;
  can_record_outcome: boolean;
  requires_safety_review: boolean;
  requires_diagnosis_review: boolean;
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
  training_plan?: TrainingExecutionPlan | null;
  recent_outcomes: Outcome[];
  trends: WorkspaceTrend[];
  capabilities: WorkspaceCapabilities;
  actions: WorkspaceAction[];
}
