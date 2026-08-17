import { describe, expect, it } from "vitest";
import type { HealthWorkspace } from "../types/workspace";
import { resolveWorkspaceActions } from "./workspaceActions";

function workspace(overrides: Partial<HealthWorkspace> = {}): HealthWorkspace {
  return {
    generated_at: "2026-08-16T00:00:00Z",
    profile_ready: true,
    body_state: {
      user_id: "user-1",
      current_revision: 3,
      safety_state: {},
      facts: [],
      observations: [],
      hypotheses: [],
      recent_revisions: [],
    },
    diagnosis: { candidates: [] },
    treatment: null,
    training_plan: null,
    treatment_revisions: [],
    recent_outcomes: [],
    trends: [],
    capabilities: {
      can_continue_consultation: true,
      can_edit_body_state: true,
      can_request_diagnosis: false,
      can_review_diagnosis: false,
      can_generate_treatment: false,
      can_accept_treatment: false,
      can_execute_treatment: false,
      can_record_outcome: false,
      can_review_treatment: false,
      requires_safety_review: false,
      requires_diagnosis_review: false,
      requires_treatment_review: false,
    },
    actions: [],
    ...overrides,
  };
}

describe("resolveWorkspaceActions", () => {
  it("keeps an active training plan discoverable after workspace reload", () => {
    const value = workspace({
      training_plan: {
        id: "plan-42",
        user_id: "user-1",
        status: "active",
        goal: "reduce neck load",
        duration_weeks: 4,
        current_week: 1,
        phases: [],
        created_at: "2026-08-16T00:00:00Z",
      },
      actions: [
        {
          kind: "open_training",
          priority: 75,
          enabled: true,
          reason: "当前已接受方案可以继续执行。",
          target: { route: "/training/plan-42" },
        },
      ],
    });

    expect(resolveWorkspaceActions(value)).toEqual([
      {
        kind: "open_training",
        label: "继续执行训练",
        description: "当前已接受方案可以继续执行。",
        href: "/training/plan-42",
      },
    ]);
  });
});
