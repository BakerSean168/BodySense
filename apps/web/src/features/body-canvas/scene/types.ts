import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import type {
  BodyCanvasLayers,
  CanvasTool,
  MotionId,
} from "../model/bodyCanvas";
import type {
  ReferenceRig,
  ReferenceMuscles,
  ReferenceLayer,
} from "./referenceRig";

export type SkeletonLayerStatus = "idle" | "loading" | "ready" | "error";

export interface BodyCanvasSceneProps {
  layers?: BodyCanvasLayers;
  onSkeletonStatus?: (status: SkeletonLayerStatus) => void;
  tool: CanvasTool;
  selectedRegions: readonly BodyRegionId[];
  motion: MotionId;
  playing: boolean;
  speed: number;
  seek: { phase: number; revision: number };
  resetKey: number;
  annotationEpoch: number;
  dark: boolean;
  onSelect: (regions: BodyRegionId[], append: boolean) => void;
  onGestureStart: () => void;
  onPhaseChange: (phase: number) => void;
  onTextFallback: () => void;
}

export interface ReferenceCanvasContentProps extends BodyCanvasSceneProps {
  rig: ReferenceRig;
  muscles: ReferenceMuscles | null;
  skeleton: ReferenceLayer | null;
  cursor: HTMLDivElement | null;
  box: HTMLDivElement | null;
  onHover: (id: BodyRegionId | null) => void;
  onContextLost: () => void;
}
