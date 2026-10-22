import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  type Dispatch,
  type SetStateAction,
} from "react";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import type { BodyExplorerWorkspaceController } from "@/features/body-explorer/hooks/useBodyExplorerWorkspace";
import {
  getBodyRegionDefinition,
  type BodyRegionId,
} from "@/features/body-explorer/model/bodyRegionOntology";
import { recordBodyRegions } from "@/features/body-explorer/model/bodyExplorerSelectors";
import {
  EMPTY_SELECTION,
  selectionReducer,
  type CanvasTool,
} from "../model/bodyCanvas";

/** Single owner of 3D selection, history and the external anatomy bridge.
 * No API writes take place here: selecting a region never records a symptom.
 */
export function useCanvasBodySelection({
  bodyState,
  bodyExplorer,
  mobile,
  setPlaying,
  setTool,
  setAnnotationEpoch,
  setRegionCardOpen,
}: {
  bodyState: BodyRecords | null;
  bodyExplorer: BodyExplorerWorkspaceController;
  mobile: boolean;
  setPlaying: Dispatch<SetStateAction<boolean>>;
  setTool: Dispatch<SetStateAction<CanvasTool>>;
  setAnnotationEpoch: Dispatch<SetStateAction<number>>;
  setRegionCardOpen: Dispatch<SetStateAction<boolean>>;
}) {
  const [selection, dispatch] = useReducer(selectionReducer, EMPTY_SELECTION);
  const primary = selection.regions[0] ?? null;
  const regionFacts = useMemo(
    () =>
      (bodyState?.facts ?? []).filter((fact) => {
        return (
          recordBodyRegions(fact).some((region) =>
            selection.regions.includes(region),
          ) && fact.assertion_status === "asserted"
        );
      }),
    [bodyState, selection.regions],
  );
  const selectionLabel = selection.regions
    .map((id) => getBodyRegionDefinition(id).labels["zh-CN"])
    .join("、");
  useEffect(() => {
    const external = bodyExplorer.selectedRegionId;
    if (external !== (selection.regions[0] ?? null)) {
      dispatch({ type: "select", regions: external ? [external] : [] });
      setRegionCardOpen(true);
    }
    // A bridge selection from the anatomy panel takes priority only when that bridge changes.
  }, [bodyExplorer.selectedRegionId]);

  const select = useCallback(
    (regions: BodyRegionId[], append = false) => {
      const next = selectionReducer(selection, {
        type: "select",
        regions,
        append,
      });
      dispatch({ type: "select", regions, append });
      bodyExplorer.selectRegion(next.regions[0] ?? null);
      setPlaying(false);
      setRegionCardOpen(true);
      if (mobile) setTool("orbit");
    },
    [selection, bodyExplorer.selectRegion, mobile],
  );
  const clear = useCallback(() => {
    dispatch({ type: "clear" });
    bodyExplorer.selectRegion(null);
    setAnnotationEpoch((value) => value + 1);
  }, [bodyExplorer.selectRegion]);
  const undo = useCallback(() => {
    const next = selectionReducer(selection, { type: "undo" });
    dispatch({ type: "undo" });
    bodyExplorer.selectRegion(next.regions[0] ?? null);
    setAnnotationEpoch((value) => value + 1);
  }, [selection, bodyExplorer.selectRegion]);
  const removeRegion = (region: BodyRegionId) => {
    const next = selectionReducer(selection, { type: "remove", region });
    dispatch({ type: "remove", region });
    bodyExplorer.selectRegion(next.regions[0] ?? null);
  };
  return {
    selection,
    primary,
    regionFacts,
    selectionLabel,
    select,
    clear,
    undo,
    removeRegion,
  };
}
