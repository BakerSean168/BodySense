import { describe, expect, it } from 'vitest';
import { getJourneyRouteReadiness } from './journeyGuards';

describe('getJourneyRouteReadiness', () => {
  it('treats missing stage as ready so loading never hard-blocks', () => {
    expect(getJourneyRouteReadiness('training', null).ready).toBe(true);
    expect(getJourneyRouteReadiness('assessment', undefined).ready).toBe(true);
  });

  it('blocks training before a plan exists', () => {
    const early = getJourneyRouteReadiness('training', 'profile_ready');
    expect(early.ready).toBe(false);
    expect(early.hint).toMatch(/训练计划/);

    expect(getJourneyRouteReadiness('training', 'plan_ready').ready).toBe(true);
    expect(getJourneyRouteReadiness('training', 'training_active').ready).toBe(true);
  });

  it('blocks assessment until the profile is ready', () => {
    const incomplete = getJourneyRouteReadiness('assessment', 'profile_incomplete');
    expect(incomplete.ready).toBe(false);
    expect(incomplete.hint).toMatch(/档案/);

    expect(getJourneyRouteReadiness('assessment', 'profile_ready').ready).toBe(true);
    expect(getJourneyRouteReadiness('assessment', 'assets_uploaded').ready).toBe(true);
  });
});
