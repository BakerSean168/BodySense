import type {
  HealthWorkspace,
  TreatmentRevision,
  WorkspaceCapabilities,
} from "../types/workspace";

export interface WorkspaceSummary {
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
  const capabilities = workspace?.capabilities;
  return {
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
