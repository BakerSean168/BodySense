import type {
  HealthWorkspace,
  TreatmentRevision,
  WorkspaceCapabilities,
} from "../types/workspace";

export interface WorkspaceSummary {
  bodyStateRevision: number;
  activeFactCount: number;
  confirmedObservationCount: number;
  pendingObservationCount: number;
  activeHypothesisCount: number;
  proposedTreatmentCount: number;
  hasCurrentTreatment: boolean;
  canExecuteTreatment: boolean;
  requiresReview: boolean;
}

export function selectProposedTreatmentRevisions(
  workspace: HealthWorkspace,
): TreatmentRevision[] {
  return workspace.treatment_revisions.filter(
    (revision) => revision.acceptance_state === "proposed",
  );
}

export function selectWorkspaceSummary(
  workspace: HealthWorkspace | null | undefined,
): WorkspaceSummary {
  const bodyState = workspace?.body_state;
  const capabilities = workspace?.capabilities;
  return {
    bodyStateRevision: bodyState?.current_revision ?? 0,
    activeFactCount:
      bodyState?.facts.filter((fact) => fact.lifecycle_state === "active")
        .length ?? 0,
    confirmedObservationCount: bodyState?.observations.length ?? 0,
    pendingObservationCount: bodyState?.pending_observations?.length ?? 0,
    activeHypothesisCount: bodyState?.hypotheses?.length ?? 0,
    proposedTreatmentCount: workspace
      ? selectProposedTreatmentRevisions(workspace).length
      : 0,
    hasCurrentTreatment: Boolean(workspace?.treatment?.current),
    canExecuteTreatment: capabilities?.can_execute_treatment ?? false,
    requiresReview: requiresWorkspaceReview(capabilities),
  };
}

export function requiresWorkspaceReview(
  capabilities: WorkspaceCapabilities | null | undefined,
): boolean {
  return Boolean(
    capabilities?.requires_safety_review ||
    capabilities?.requires_diagnosis_review ||
    capabilities?.requires_treatment_review,
  );
}
