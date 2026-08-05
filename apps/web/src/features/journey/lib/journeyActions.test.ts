import { describe, expect, it } from 'vitest';
import type { JourneyArtifacts } from '@bodysense/contracts';
import { JOURNEY_ACTIONS, resolveJourneyActions } from './journeyActions';

function makeArtifacts(overrides: Partial<JourneyArtifacts> = {}): JourneyArtifacts {
  return {
    has_profile: true,
    has_upload: true,
    has_consultation: true,
    has_diagnosis: true,
    has_treatment: true,
    has_training: true,
    needs_reassessment: false,
    has_reassessment: false,
    ...overrides,
  };
}

describe('resolveJourneyActions', () => {
  it('preserves backend ordering so the first action is the primary call to action', () => {
    const resolved = resolveJourneyActions(
      ['view_progress', 'log_training', 'reassess'],
      makeArtifacts(),
    );

    expect(resolved.map((a) => a.action)).toEqual([
      'view_progress',
      'log_training',
      'reassess',
    ]);
  });

  it('drops unknown action codes instead of throwing', () => {
    const resolved = resolveJourneyActions(
      ['start_training', 'some_future_action'],
      makeArtifacts(),
    );

    expect(resolved.map((a) => a.action)).toEqual(['start_training']);
  });

  it('returns an empty list when the backend reports no actions', () => {
    expect(resolveJourneyActions([], makeArtifacts())).toEqual([]);
  });

  it('carries label and description through for rendering', () => {
    const [action] = resolveJourneyActions(['complete_profile'], makeArtifacts());

    expect(action.label).toBe(JOURNEY_ACTIONS.complete_profile.label);
    expect(action.description).toBe(JOURNEY_ACTIONS.complete_profile.description);
  });
});

describe('journey action routes', () => {
  it('deep-links training actions to the active plan', () => {
    const [action] = resolveJourneyActions(
      ['start_training'],
      makeArtifacts({ active_training_plan_id: 'plan-1' }),
    );

    expect(action.href).toBe('/training/plan-1');
  });

  it('falls back to consultation when no training plan id is known', () => {
    // `/training` has no route (App.tsx only defines `/training/:id`), so a
    // missing plan id must not produce a dead link.
    const [action] = resolveJourneyActions(['start_training'], makeArtifacts());

    expect(action.href).toBe('/consultation');
  });

  it('deep-links consultation actions to the active conversation', () => {
    const [action] = resolveJourneyActions(
      ['continue_consultation'],
      makeArtifacts({ active_consultation_id: 'conv-1' }),
    );

    expect(action.href).toBe('/consultation/conv-1');
  });

  it('deep-links reassessment to the latest assessment', () => {
    const [action] = resolveJourneyActions(
      ['reassess'],
      makeArtifacts({ latest_assessment_id: 'report-1' }),
    );

    expect(action.href).toBe('/assessment/report-1');
  });

  it('never resolves to a route the router does not define', () => {
    const artifacts = makeArtifacts();
    const allActions = Object.keys(JOURNEY_ACTIONS);

    // Routes declared in App.tsx, with `:id` segments reduced to their prefix.
    const knownRoutes = [
      '/onboarding',
      '/profile',
      '/assessment',
      '/consultation',
      '/training/',
      '/dashboard',
      '/history',
    ];

    for (const { action, href } of resolveJourneyActions(allActions, artifacts)) {
      const matches = knownRoutes.some(
        (route) => href === route || href.startsWith(route),
      );
      expect(matches, `${action} resolved to unroutable ${href}`).toBe(true);
    }
  });
});
