import { useMutation } from "@tanstack/react-query";
import { workspaceApi } from "../api/workspaceApi";
import { useWorkspaceInvalidation } from "./useWorkspaceInvalidation";

export type TreatmentCommand =
  | {
      type: "generateProposal";
      diagnosisAnalysisId: string;
      userConstraints?: Record<string, unknown>;
    }
  | {
      type: "acceptRevision";
      revisionId: string;
      consultationId?: string | null;
    }
  | { type: "rejectRevision"; revisionId: string }
  | { type: "reviewCurrent" }
  | {
      type: "recordOutcome";
      input: Parameters<typeof workspaceApi.recordOutcome>[0];
    };

export async function executeTreatmentCommand(command: TreatmentCommand) {
  switch (command.type) {
    case "generateProposal":
      return workspaceApi.generateTreatmentProposal(
        command.diagnosisAnalysisId,
        command.userConstraints,
      );
    case "acceptRevision":
      return workspaceApi.acceptTreatmentRevision(
        command.revisionId,
        command.consultationId,
      );
    case "rejectRevision":
      return workspaceApi.rejectTreatmentRevision(command.revisionId);
    case "reviewCurrent":
      return workspaceApi.reviewCurrentTreatment();
    case "recordOutcome":
      return workspaceApi.recordOutcome(command.input);
  }
}

export function useTreatmentCommand() {
  const invalidateWorkspace = useWorkspaceInvalidation();
  return useMutation({
    mutationKey: ["workspace", "treatment-command"],
    mutationFn: executeTreatmentCommand,
    onSuccess: invalidateWorkspace,
  });
}
