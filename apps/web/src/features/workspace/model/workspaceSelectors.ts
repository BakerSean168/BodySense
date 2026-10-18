import type {
  HealthWorkspace,
  WorkspaceCapabilities,
} from "../types/workspace";

export interface WorkspaceSummary {
  canExecuteTreatment: boolean;
  requiresReview: boolean;
}

export function selectWorkspaceSummary(
  workspace: HealthWorkspace | null | undefined,
): WorkspaceSummary {
  const capabilities = workspace?.capabilities;
  return {
    canExecuteTreatment: capabilities?.can_execute_treatment ?? false,
    requiresReview: requiresWorkspaceReview(capabilities),
  };
}

export function requiresWorkspaceReview(
  capabilities: WorkspaceCapabilities | null | undefined,
): boolean {
  return Boolean(
    capabilities?.requires_safety_review ||
    capabilities?.requires_diagnosis_review,
  );
}
