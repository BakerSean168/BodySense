import { useEffect, useMemo, useState } from "react";
import {
  bodyMetricsService,
  type BodyMetricsSnapshot,
} from "@/features/profile/services/bodyMetricsService";
import {
  lifestyleService,
  type LifestyleSnapshot,
} from "@/features/profile/services/lifestyleService";
import { useUploadStore } from "@/stores/uploadStore";
import {
  HABIT_SECTIONS,
  type HabitSectionKey,
  type ProfileUploadLike,
} from "../model/profileCategories";

export interface PersonHabitRow {
  key: HabitSectionKey;
  label: string;
  summary: string;
  updated_at?: string;
}

export interface PersonWindowData {
  metrics: BodyMetricsSnapshot | null;
  habits: PersonHabitRow[];
  uploads: ProfileUploadLike[];
  loading: boolean;
  /** A partial read failure is surfaced; it is never hidden behind empty values. */
  error: string | null;
}

/**
 * Loads the Person window's authoritative sources. The hook lives inside the
 * window, so nothing is fetched until the user actually opens it, and missing
 * values stay missing instead of being replaced with demo content.
 */
export function usePersonWindowData(): PersonWindowData {
  const uploads = useUploadStore((state) => state.uploads);
  const fetchUploads = useUploadStore((state) => state.fetchUploads);
  const [metrics, setMetrics] = useState<BodyMetricsSnapshot | null>(null);
  const [lifestyle, setLifestyle] = useState<LifestyleSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const [metricsResult, lifestyleResult, uploadsResult] = await Promise.allSettled([
        bodyMetricsService.get(),
        lifestyleService.get(),
        fetchUploads(),
      ]);
      if (cancelled) return;
      if (metricsResult.status === "fulfilled") setMetrics(metricsResult.value);
      if (lifestyleResult.status === "fulfilled") setLifestyle(lifestyleResult.value);
      if (
        metricsResult.status === "rejected" ||
        lifestyleResult.status === "rejected" ||
        uploadsResult.status === "rejected"
      ) {
        setError("部分档案资料暂时无法读取。");
      }
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchUploads]);

  const habits = useMemo<PersonHabitRow[]>(
    () =>
      HABIT_SECTIONS.map((section) => ({
        key: section.key,
        label: section.label,
        summary: lifestyle?.[section.key]?.summary ?? "",
        updated_at: lifestyle?.[section.key]?.updated_at,
      })),
    [lifestyle],
  );

  return {
    metrics,
    habits,
    uploads: uploads as ProfileUploadLike[],
    loading,
    error,
  };
}
