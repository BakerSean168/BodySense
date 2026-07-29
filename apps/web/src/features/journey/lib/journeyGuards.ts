import type { JourneyStage } from '@bodysense/contracts';

/**
 * Soft route readiness for journey-gated pages.
 *
 * Soft = never hard-block navigation. When the stage is not ready we surface a
 * guidance card with the backend's available_actions so the user can continue
 * the main line without inventing next steps on the client.
 */

export type JourneyGatedRoute = 'training' | 'assessment';

/** Stages that already have enough context for the given route. */
const READY_STAGES: Record<JourneyGatedRoute, ReadonlySet<JourneyStage>> = {
  // Training assumes a treatment plan exists (or is actively being followed).
  training: new Set([
    'plan_ready',
    'training_active',
    'reassessment_due',
    'completed',
  ]),
  // Assessment can start once the profile is in place; earlier stages should
  // complete onboarding first.
  assessment: new Set([
    'profile_ready',
    'assets_uploaded',
    'assessment_ready',
    'consulting',
    'diagnosis_ready',
    'plan_ready',
    'training_active',
    'reassessment_due',
    'completed',
  ]),
};

const ROUTE_COPY: Record<
  JourneyGatedRoute,
  { title: string; notReadyHint: string }
> = {
  training: {
    title: '训练计划',
    notReadyHint: '当前阶段还没有可跟练的训练计划。先完成评估与问诊，生成方案后再来这里。',
  },
  assessment: {
    title: '姿态评估',
    notReadyHint: '开始评估前请先完善身体档案，以便报告基于你的真实指标生成。',
  },
};

export interface JourneyRouteReadiness {
  ready: boolean;
  title: string;
  hint: string;
}

/**
 * Decide whether the current journey stage is ready for a gated route.
 *
 * Unknown / loading stages are treated as ready so the soft guard never
 * blocks the page while journey state is still fetching.
 */
export function getJourneyRouteReadiness(
  route: JourneyGatedRoute,
  stage: JourneyStage | null | undefined,
): JourneyRouteReadiness {
  const copy = ROUTE_COPY[route];
  if (!stage) {
    return { ready: true, title: copy.title, hint: '' };
  }
  const ready = READY_STAGES[route].has(stage);
  return {
    ready,
    title: copy.title,
    hint: ready ? '' : copy.notReadyHint,
  };
}
