import type { ReactNode } from "react";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import type { HealthWorkspace } from "@/features/workspace/types/workspace";
import type { BodyExplorerWorkspaceController } from "@/features/body-explorer/hooks/useBodyExplorerWorkspace";
import type { ConsultationSpatialContext } from "@/features/consultation/types/consultation";
import type { DiagnosisRequestMessage } from "../components/DiagnosisRequestEntry";
import type { ChatHistory } from "../model/chatHistory";
import type { BusinessWindowId } from "../model/businessWindows";
import type { MotionId } from "../model/bodyCanvas";
import type { PersonProfileIdentity } from "../windows/person/types";

export interface BodyCanvasWorkspaceProps {
  bodyState: BodyRecords | null;
  workspace?: HealthWorkspace;
  bodyExplorer: BodyExplorerWorkspaceController;
  chat: ReactNode;
  /** The user's latest completed message: the only candidate analysis request. */
  latestUserMessage?: DiagnosisRequestMessage | null;
  /** Days the user wrote in, for the Chat header picker and history list. */
  chatHistory?: ChatHistory;
  onAskContext: (context: ConsultationSpatialContext) => void;
  onOpenProfile: () => void;
  /** Real profile values for the Person window; never invented when absent. */
  identity?: PersonProfileIdentity;
  /** Deep link (?view=…) opens the matching business window once. */
  initialWindow?: BusinessWindowId | null;
  onWindowChange?: (window: BusinessWindowId | null) => void;
  /**
   * Requests a business window from outside the canvas (for example the anatomy
   * overlay's "ask" action). The revision counter makes repeated requests for the
   * same window observable without re-opening on unrelated re-renders.
   */
  windowRequest?: { window: BusinessWindowId; revision: number } | null;
  demonstrationRequest?: { motion: MotionId; revision: number } | null;
  loading?: boolean;
  error?: boolean;
  onRetry?: () => void;
}
