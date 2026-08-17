import { describe, expect, it } from "vitest";
import type { HealthWorkspace } from "../types/workspace";
import {
  selectProposedTreatmentRevisions,
  selectWorkspaceSummary,
} from "./workspaceSelectors";

const workspace = {
  body_state: {
    current_revision: 7,
    facts: [
      { id: "f1", lifecycle_state: "active" },
      { id: "f2", lifecycle_state: "resolved" },
    ],
    observations: [{ id: "o1" }],
    pending_observations: [{ id: "o2" }],
    hypotheses: [{ id: "h1" }],
  },
  treatment_revisions: [
    { id: "r1", acceptance_state: "proposed" },
    { id: "r2", acceptance_state: "accepted" },
  ],
  treatment: { current: { id: "r2" } },
  capabilities: {
    can_execute_treatment: true,
    requires_safety_review: false,
    requires_diagnosis_review: true,
    requires_treatment_review: false,
  },
} as unknown as HealthWorkspace;

describe("workspace selectors", () => {
  it("projects a compact presentation summary", () => {
    expect(selectWorkspaceSummary(workspace)).toEqual({
      bodyStateRevision: 7,
      activeFactCount: 1,
      confirmedObservationCount: 1,
      pendingObservationCount: 1,
      activeHypothesisCount: 1,
      proposedTreatmentCount: 1,
      hasCurrentTreatment: true,
      canExecuteTreatment: true,
      requiresReview: true,
    });
  });

  it("keeps proposal filtering out of rendering components", () => {
    expect(
      selectProposedTreatmentRevisions(workspace).map(({ id }) => id),
    ).toEqual(["r1"]);
  });
});
