import { useMutation } from "@tanstack/react-query";
import { ApiRequestError } from "@/lib/api-client";
import type { AddFactInput } from "../api/workspaceApi";
import { workspaceApi } from "../api/workspaceApi";
import { useWorkspaceInvalidation } from "./useWorkspaceInvalidation";

export type BodyStateCommand =
  | { type: "addFact"; expectedRevision: number; fact: AddFactInput }
  | {
      type: "reviewFact";
      factId: string;
      expectedRevision: number;
      reviewState: string;
    }
  | {
      type: "correctFact";
      factId: string;
      expectedRevision: number;
      replacement: AddFactInput;
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
      reviewState: "confirmed" | "rejected";
    }
  | {
      type: "updateHypothesisLifecycle";
      hypothesisId: string;
      expectedRevision: number;
      lifecycleState: string;
    }
  | {
      type: "resolveSafety";
      expectedRevision: number;
      resolution: string;
      note: string;
    };

export async function executeBodyStateCommand(command: BodyStateCommand) {
  switch (command.type) {
    case "addFact":
      return workspaceApi.addFact(command.expectedRevision, command.fact);
    case "reviewFact":
      return workspaceApi.reviewFact(
        command.factId,
        command.expectedRevision,
        command.reviewState,
      );
    case "correctFact":
      return workspaceApi.correctFact(
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
    case "updateHypothesisLifecycle":
      return workspaceApi.updateHypothesisLifecycle(
        command.hypothesisId,
        command.expectedRevision,
        command.lifecycleState,
      );
    case "resolveSafety":
      return workspaceApi.resolveSafety(
        command.expectedRevision,
        command.resolution,
        command.note,
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
