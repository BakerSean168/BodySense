import type {
  HealthWorkspace,
  WorkspaceCapabilities,
} from "../types/workspace";

export interface WorkspaceSummary {
  canStartTraining: boolean;
  requiresReview: boolean;
}

export function selectWorkspaceSummary(
  workspace: HealthWorkspace | null | undefined,
): WorkspaceSummary {
  const capabilities = workspace?.capabilities;
  return {
    canStartTraining: capabilities?.can_start_training ?? false,
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
