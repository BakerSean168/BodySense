import { useCallback, useEffect, useState } from 'react';
import type { HealthJourneyState } from '@bodysense/contracts';
import { getJourneyState } from '../services/journeyService';

export interface UseJourneyStateResult {
  journey: HealthJourneyState | null;
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

/**
 * Load the backend-derived health journey state.
 *
 * Deliberately holds no opinion about what the next step is — that is the
 * backend's `available_actions`. This hook only owns fetch lifecycle.
 */
export function useJourneyState(): UseJourneyStateResult {
  const [journey, setJourney] = useState<HealthJourneyState | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      setJourney(await getJourneyState());
    } catch (err) {
      setJourney(null);
      setError(err instanceof Error ? err.message : 'Failed to load journey state');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return { journey, isLoading, error, refresh: load };
}
