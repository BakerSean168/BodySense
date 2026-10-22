import type {
  AnatomyStructureId,
  AnatomyViewerErrorState,
} from "../adapters/anatomyViewerPort";

export interface BodyExplorer3DProps {
  selectedAnatomyId: AnatomyStructureId | null;
  onSelectedAnatomyIdChange: (id: AnatomyStructureId | null) => void;
  mode: "region" | "anatomy";
  onModeChange: (mode: "region" | "anatomy") => void;
  onRegionModeRequested?: () => void;
  selectedRegionLabel?: string | null;
  focusRequest?: { id: AnatomyStructureId; key: string | number } | null;
  resetRequestKey?: string | number;
  onAskContext?: (context: {
    anatomyId?: string | null;
    anatomyName?: string | null;
    regionLabel?: string | null;
  }) => void;
  onFatalError: (error: AnatomyViewerErrorState) => void;
  diagnosticSessionId: string;
  attemptId: string;
}
