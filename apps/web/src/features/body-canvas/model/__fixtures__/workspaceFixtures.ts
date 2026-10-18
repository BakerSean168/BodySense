import type { HealthWorkspace } from "@/features/workspace/types/workspace";

export function makeWorkspace(
  overrides: Partial<HealthWorkspace> = {},
): HealthWorkspace {
  return {
    generated_at: "2026-10-11T00:00:00Z",
    profile_ready: true,
    recent_outcomes: [],
    trends: [],
    capabilities: {
      can_continue_consultation: true,
      can_edit_body_state: true,
      can_request_diagnosis: true,
      can_review_diagnosis: true,
      can_generate_treatment: true,
      can_execute_treatment: true,
      can_record_outcome: true,
      requires_safety_review: false,
      requires_diagnosis_review: false,
    },
    actions: [],
    ...overrides,
  };
}
