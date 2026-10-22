import type { ProjectTrainingWindowInput } from "../../model/trainingPlans";

export interface TrainingPlanControls {
  busy?: boolean;
  error?: string | null;
  onRetry?: () => void;
  notice?: "rejected" | null;
  onOpenProposal: (proposalId: string) => void;
  onBackToPlan: () => void;
  onAccept: (proposalId: string) => void;
  onReject: (proposalId: string) => void;
  onDismissNotice: () => void;
  onSelectTemplate: (templateId: string) => void;
  onStart: (planId: string, templateId: string) => void;
  onOpenChat: () => void;
}

export interface TrainingPlanWindowProps {
  /** Raw records + local decision state; projected here, inside the lazy chunk. */
  state: ProjectTrainingWindowInput;
  partialError?: string | null;
  controls: TrainingPlanControls;
  /**
   * Workspace execution capability. The Go gate on the record stays the
   * authority; this only keeps Start closed while the workspace disagrees.
   */
  canExecute: boolean;
}
