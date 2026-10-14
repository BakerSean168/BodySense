import { useEffect, useState } from "react";
import {
  bodyMetricsService,
  type BodyMetricsSnapshot,
} from "@/features/profile/services/bodyMetricsService";
import { useUploadStore } from "@/stores/uploadStore";
import type { ProfileUploadLike } from "../model/profileCategories";

export interface PersonWindowData {
  metrics: BodyMetricsSnapshot | null;
  uploads: ProfileUploadLike[];
  loading: boolean;
  error: string | null;
}

/** Ancillary Person reads. Records/Habits independently use the new BodyFact ledger. */
export function usePersonWindowData(): PersonWindowData {
  const uploads = useUploadStore((state) => state.uploads);
  const fetchUploads = useUploadStore((state) => state.fetchUploads);
  const [metrics, setMetrics] = useState<BodyMetricsSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const [metricsResult, uploadsResult] = await Promise.allSettled([
        bodyMetricsService.get(),
        fetchUploads(),
      ]);
      if (cancelled) return;
      if (metricsResult.status === "fulfilled") setMetrics(metricsResult.value);
      if (
        metricsResult.status === "rejected" ||
        uploadsResult.status === "rejected"
      ) {
        setError("部分基础资料或文件暂时无法读取。");
      }
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchUploads]);

  return { metrics, uploads: uploads as ProfileUploadLike[], loading, error };
}
