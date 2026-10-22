import { useEffect, useRef, useState } from "react";
import {
  attachReferenceMuscles,
  attachReferenceSkeleton,
  buildReferenceRig,
  type ReferenceLayer,
  type ReferenceMuscles,
  type ReferenceRig,
} from "./referenceRig";
import {
  loadPinnedReferenceAtlasSurface,
  loadPinnedReferenceMuscles,
  loadPinnedReferenceSkeleton,
  type ReferenceAtlasSurface,
} from "./referenceAtlasLoader";
import type { SkeletonLayerStatus } from "./types";

/** Owns load/abort/dispose for the pinned skin, optional muscles, and
 * opt-in skeleton. 3D resource ownership never crosses the window controller.
 */
export function useReferenceSceneAssets(
  wantsSkeleton: boolean,
  onStatus?: (status: SkeletonLayerStatus) => void,
) {
  const [attempt, setAttempt] = useState(0);
  const [rig, setRig] = useState<ReferenceRig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [muscles, setMuscles] = useState<ReferenceMuscles | null>(null);
  const [skeleton, setSkeleton] = useState<ReferenceLayer | null>(null);
  const [skeletonStatus, setSkeletonStatus] =
    useState<SkeletonLayerStatus>("idle");
  const onSkeletonStatus = useRef(onStatus);
  useEffect(() => {
    onSkeletonStatus.current = onStatus;
  }, [onStatus]);
  useEffect(() => {
    onSkeletonStatus.current?.(skeletonStatus);
  }, [skeletonStatus]);
  // The skeletal layer is fetched once per rig, the first time 骨骼 is turned on,
  // and skinned to the same rig as the muscles. Toggling only hides it.
  const skeletonJob = useRef<{
    rig: ReferenceRig;
    controller: AbortController;
    layer: ReferenceLayer | null;
  } | null>(null);
  useEffect(() => {
    if (!rig || !wantsSkeleton || skeletonJob.current?.rig === rig) return;
    const job = {
      rig,
      controller: new AbortController(),
      layer: null as ReferenceLayer | null,
    };
    skeletonJob.current = job;
    setSkeletonStatus("loading");
    void (async () => {
      let surface: ReferenceAtlasSurface | null = null;
      try {
        surface = await loadPinnedReferenceSkeleton({
          signal: job.controller.signal,
        });
        if (job.controller.signal.aborted) return;
        job.layer = attachReferenceSkeleton(rig, surface.group);
        setSkeleton(job.layer);
        setSkeletonStatus("ready");
      } catch {
        if (job.controller.signal.aborted) return;
        if (skeletonJob.current === job) skeletonJob.current = null;
        setSkeletonStatus("error");
      } finally {
        surface?.dispose();
      }
    })();
  }, [rig, wantsSkeleton]);
  useEffect(
    () => () => {
      const job = skeletonJob.current;
      if (!job || job.rig !== rig) return;
      job.controller.abort();
      job.layer?.dispose();
      skeletonJob.current = null;
      setSkeleton(null);
      setSkeletonStatus("idle");
    },
    [rig],
  );
  useEffect(() => {
    const controller = new AbortController();
    let owned: ReferenceRig | null = null;
    let ownedMuscles: ReferenceMuscles | null = null;
    let surface: ReferenceAtlasSurface | null = null;
    let muscleSurface: ReferenceAtlasSurface | null = null;
    setError(null);
    setRig(null);
    setMuscles(null);
    // The 6.6 MB Z-Anatomy muscle layer downloads in parallel but never blocks the
    // skin: the body is interactive first and the muscles appear when ready.
    let skinReady: () => void = () => undefined;
    const skinBuilt = new Promise<void>((resolve) => {
      skinReady = resolve;
    });
    const musclesPending = loadPinnedReferenceMuscles({
      signal: controller.signal,
      parseAfter: skinBuilt,
    });
    musclesPending.catch(() => undefined);
    void (async () => {
      surface = await loadPinnedReferenceAtlasSurface({
        signal: controller.signal,
      });
      if (controller.signal.aborted) {
        surface.dispose();
        surface = null;
        void musclesPending.then(
          (unused) => unused.dispose(),
          () => undefined,
        );
        return;
      }
      owned = buildReferenceRig(surface.group);
      surface.dispose();
      surface = null;
      setRig(owned);
      // Give React a frame to paint the skin before parsing the muscles.
      setTimeout(skinReady, 50);
      const rigForMuscles = owned;
      try {
        muscleSurface = await musclesPending;
        if (controller.signal.aborted) return;
        ownedMuscles = attachReferenceMuscles(
          rigForMuscles,
          muscleSurface.group,
        );
        setMuscles(ownedMuscles);
      } catch {
        // Optional layer: the translucent skin alone stays fully usable.
      } finally {
        muscleSurface?.dispose();
        muscleSurface = null;
      }
    })().catch((cause: unknown) => {
      skinReady();
      void musclesPending.then(
        (unused) => unused.dispose(),
        () => undefined,
      );
      surface?.dispose();
      surface = null;
      if (!controller.signal.aborted) {
        setError(cause instanceof Error ? cause.message : "3D 暂时无法显示");
      }
    });
    return () => {
      controller.abort();
      skinReady();
      surface?.dispose();
      muscleSurface?.dispose();
      ownedMuscles?.dispose();
      owned?.dispose();
    };
  }, [attempt]);

  return {
    rig,
    muscles,
    skeleton,
    skeletonStatus,
    error,
    setError,
    attempt,
    setAttempt,
  };
}
