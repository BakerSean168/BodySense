import { useMutation } from "@tanstack/react-query";
import { ApiRequestError } from "@/lib/api-client";
import type {
  BodyStateReviewState,
  BodyStateSafetyResolution,
  LifestyleSectionKey,
  SymptomStatementInput,
} from "../api/workspaceApi";
import { workspaceApi } from "../api/workspaceApi";
import { useWorkspaceInvalidation } from "./useWorkspaceInvalidation";

export type BodyStateCommand =
  | {
      type: "reviewFact";
      factId: string;
      expectedRevision: number;
      reviewState: BodyStateReviewState;
    }
  | {
      type: "recordSymptom";
      expectedRevision: number;
      input: SymptomStatementInput;
    }
  | {
      type: "correctSymptom";
      factId: string;
      expectedRevision: number;
      replacement: SymptomStatementInput;
    }
  | {
      type: "updateFactTemporal";
      factId: string;
      expectedRevision: number;
      input: {
        lifecycle_state?: string;
        trend?: string;
        valid_until?: string;
      };
    }
  | {
      type: "reviewObservation";
      observationId: string;
      expectedRevision: number;
      reviewState: BodyStateReviewState;
    }
  | {
      type: "resolveSafety";
      expectedRevision: number;
      resolution: BodyStateSafetyResolution;
      note: string;
    }
  | {
      type: "updateLifestyleCurrent";
      expectedRevision: number;
      section: LifestyleSectionKey;
      summary: string;
      details?: Record<string, unknown>;
    };

export async function executeBodyStateCommand(command: BodyStateCommand) {
  switch (command.type) {
    case "reviewFact":
      return workspaceApi.reviewFact(
        command.factId,
        command.expectedRevision,
        command.reviewState,
      );
    case "recordSymptom":
      return workspaceApi.recordSymptom(
        command.expectedRevision,
        command.input,
      );
    case "correctSymptom":
      return workspaceApi.correctSymptom(
        command.factId,
        command.expectedRevision,
        command.replacement,
      );
    case "updateFactTemporal":
      return workspaceApi.updateFactTemporal(
        command.factId,
        command.expectedRevision,
        command.input,
      );
    case "reviewObservation":
      return workspaceApi.reviewObservation(
        command.observationId,
        command.expectedRevision,
        command.reviewState,
      );
    case "resolveSafety":
      return workspaceApi.resolveSafety(
        command.expectedRevision,
        command.resolution,
        command.note,
      );
    case "updateLifestyleCurrent":
      return workspaceApi.updateLifestyleCurrent(
        command.expectedRevision,
        command.section,
        command.summary,
        command.details,
      );
  }
}

export function useBodyStateCommand() {
  const invalidateWorkspace = useWorkspaceInvalidation();
  return useMutation({
    mutationKey: ["workspace", "body-state-command"],
    mutationFn: executeBodyStateCommand,
    onSuccess: invalidateWorkspace,
    onError: (error) => {
      if (error instanceof ApiRequestError && error.status === 409) {
        void invalidateWorkspace();
      }
    },
  });
}
