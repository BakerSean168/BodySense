export { BodyExplorer, detectWebGLSupport } from "./components/BodyExplorer";
export { BodyRegionNavigator } from "./components/BodyRegionNavigator";
export { useBodyExplorerWorkspace } from "./hooks/useBodyExplorerWorkspace";
export {
  getAnatomyMappingForRegion,
  getAnatomyIdsForRegion,
  getBodyRegionForAnatomy,
  getPreferredAnatomyIdForRegion,
  resolveBodyRegionForAnatomy,
} from "./model/anatomyMapping";
export {
  bodyRegionDefinitions,
  getBodyRegionDefinition,
  isBodyRegionId,
  resolveBodyRegionInput,
  type BodyRegionId,
} from "./model/bodyRegionOntology";
export { resolveRecordBodyRegion } from "./model/bodyExplorerSelectors";
export type {
  BodyExplorerProps,
  BodyExplorerSemanticBridge,
} from "./components/BodyExplorer";
export { BodyExplorerFallback2D } from "./components/BodyExplorerFallback2D";
export type {
  AnatomyDisplayMode,
  AnatomyIsolationMode,
  AnatomyLoadState,
  AnatomyStructureId,
  AnatomyViewerErrorState,
  AnatomyViewerPort,
  AnatomyViewerSnapshot,
  AnatomyWebGLState,
} from "./adapters/anatomyViewerPort";
export { anatomyStructureId } from "./adapters/anatomyViewerPort";
