import { useEffect, useState } from "react";
import { trainingApi, type TrainingPlan } from "@/features/training";

export interface TrainingWindowData {
  plans: TrainingPlan[];
  /** True only while a training window is open and the list is in flight. */
  loading: boolean;
  error: string | null;
}

/**
 * Loads the user's real training plans on demand. A failed read reports an error;
 * it must never fall back to an example plan, because plan identity is what the
 * user authorises changes against.
 */
export function useTrainingWindowData(enabled: boolean): TrainingWindowData {
  const [plans, setPlans] = useState<TrainingPlan[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    void trainingApi
      .listPlans()
      .then((result) => {
        if (cancelled) return;
        setPlans(result);
      })
      .catch(() => {
        if (cancelled) return;
        setPlans([]);
        setError("训练计划暂时无法读取。");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [enabled]);

  return { plans, loading: enabled && loading, error };
}
