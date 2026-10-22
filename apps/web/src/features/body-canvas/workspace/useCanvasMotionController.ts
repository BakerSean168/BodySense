import {
  useCallback,
  useEffect,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import {
  SHOW_POSE_BAR,
  type CanvasTool,
  type MotionId,
} from "../model/bodyCanvas";

/** Playback is reference-only; neither animation nor scrubbing writes health data. */
export function useCanvasMotionController({
  safetyReview,
  reducedMotion,
  setTool,
  setRegionCardOpen,
  setCanvasOverlay,
  demonstrationRequest,
}: {
  safetyReview: boolean;
  reducedMotion: boolean;
  setTool: Dispatch<SetStateAction<CanvasTool>>;
  setRegionCardOpen: Dispatch<SetStateAction<boolean>>;
  setCanvasOverlay: Dispatch<SetStateAction<"help" | null>>;
  demonstrationRequest?: { motion: MotionId; revision: number } | null;
}) {
  const [motion, setMotion] = useState<MotionId>("stand");
  const [motionMode, setMotionMode] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [phase, setPhase] = useState(0);
  const [seek, setSeek] = useState({ phase: 0, revision: 0 });
  const [resetKey, setResetKey] = useState(0);
  const [annotationEpoch, setAnnotationEpoch] = useState(0);
  useEffect(() => {
    if (reducedMotion) setPlaying(false);
  }, [reducedMotion]);
  const chooseMotion = useCallback(
    (id: MotionId) => {
      setMotion(id);
      setMotionMode(true);
      setPhase(0);
      setSeek((value) => ({ phase: 0, revision: value.revision + 1 }));
      setPlaying(!safetyReview && !reducedMotion);
      setTool("orbit");
      setRegionCardOpen(false);
      setCanvasOverlay(null);
    },
    [safetyReview, reducedMotion],
  );
  const choosePose = useCallback(
    (id: MotionId) => {
      if (id !== "stand") {
        chooseMotion(id);
        return;
      }
      setMotionMode(false);
      setPlaying(false);
      setMotion("stand");
      setSeek((value) => ({ phase: 0, revision: value.revision + 1 }));
    },
    [chooseMotion],
  );
  const requestRevision = demonstrationRequest?.revision;
  useEffect(() => {
    if (SHOW_POSE_BAR && demonstrationRequest)
      chooseMotion(demonstrationRequest.motion);
  }, [requestRevision, chooseMotion]);

  return {
    motion,
    motionMode,
    setMotionMode,
    playing,
    setPlaying,
    speed,
    setSpeed,
    phase,
    setPhase,
    seek,
    setSeek,
    resetKey,
    setResetKey,
    annotationEpoch,
    setAnnotationEpoch,
    choosePose,
  };
}
