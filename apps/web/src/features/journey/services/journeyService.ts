import type { HealthJourneyState } from '@bodysense/contracts';
import { authFetch } from '@/features/auth/services/authService';
import { safeJson, extractErrorMessage } from '@/lib/api-url';

/**
 * Fetch the user's current health journey state.
 *
 * The backend derives the stage and `available_actions` from profile, uploads,
 * consultations, assessments and training plans. The frontend renders whatever
 * the backend reports as actionable — it must not re-derive the next step
 * locally (see docs/plan/active/p3-health-journey-activation.md).
 */
export async function getJourneyState(): Promise<HealthJourneyState> {
  const response = await authFetch('/api/v1/journey');

  if (!response.ok) {
    throw new Error(await extractErrorMessage(response));
  }

  const state = await safeJson<HealthJourneyState | null>(response);
  if (!state || typeof state !== 'object') {
    throw new Error('Empty journey state response');
  }
  return state;
}
