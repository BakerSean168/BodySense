import type { JourneyAction, JourneyArtifacts } from '@bodysense/contracts';

/**
 * Presentation metadata for a journey action.
 *
 * The backend owns *which* actions are available; this registry owns only how
 * each one is labelled and where it navigates. Keeping the two separate is what
 * lets the dashboard stop guessing the next step.
 */
export interface JourneyActionDescriptor {
  /** Short button label. */
  label: string;
  /** One-line explanation of what the action does. */
  description: string;
  /** Route to navigate to, given the current artifacts. */
  href: (artifacts: JourneyArtifacts) => string;
}

/**
 * A descriptor with its route already resolved against the current artifacts.
 *
 * `href` is narrowed from a resolver function to the concrete path, so callers
 * render it directly without needing the artifacts again.
 */
export interface ResolvedJourneyAction extends Omit<JourneyActionDescriptor, 'href'> {
  /** The backend action code this was resolved from. */
  action: string;
  /** Concrete route for this action. */
  href: string;
}

const CONSULTATION_ROUTE = '/consultation';

function consultationHref(artifacts: JourneyArtifacts): string {
  return artifacts.active_consultation_id
    ? `${CONSULTATION_ROUTE}/${artifacts.active_consultation_id}`
    : CONSULTATION_ROUTE;
}

function assessmentHref(artifacts: JourneyArtifacts): string {
  return artifacts.latest_assessment_id
    ? `/assessment/${artifacts.latest_assessment_id}`
    : '/assessment';
}

/**
 * Training is only reachable per-plan (`/training/:id`) — there is no index
 * route. Without an active plan the treatment lives on the consultation thread
 * that produced it, so fall back there rather than to a dead link.
 */
function trainingHref(artifacts: JourneyArtifacts): string {
  return artifacts.active_training_plan_id
    ? `/training/${artifacts.active_training_plan_id}`
    : consultationHref(artifacts);
}

/**
 * Registry of every action the backend can report.
 *
 * Kept exhaustive over `JourneyAction` so adding a backend action without a
 * frontend affordance becomes a type error rather than a silently missing
 * button.
 */
export const JOURNEY_ACTIONS: Record<JourneyAction, JourneyActionDescriptor> = {
  complete_profile: {
    label: '完善身体档案',
    description: '填写身高体重、作息与运动习惯，作为后续评估的基础。',
    href: () => '/onboarding',
  },
  upload_report: {
    label: '上传体检报告',
    description: '上传报告后系统会自动识别关键指标。',
    href: () => '/profile',
  },
  upload_photo: {
    label: '上传体态照片',
    description: '上传正面、侧面、背面站姿照片，获取 AI 体态分析。',
    href: () => '/profile',
  },
  start_assessment: {
    label: '开始姿态评估',
    description: '基于档案与已上传资料生成一份完整的体态评估报告。',
    href: () => '/assessment',
  },
  start_consultation: {
    label: '开始智能问诊',
    description: '与 AI 助手描述你的不适，逐步定位体态问题。',
    href: () => CONSULTATION_ROUTE,
  },
  continue_consultation: {
    label: '继续问诊',
    description: '回到进行中的问诊，补充信息以便生成诊断。',
    href: consultationHref,
  },
  request_analysis: {
    label: '请求分析',
    description: '信息已足够，让 AI 汇总并给出诊断候选。',
    href: consultationHref,
  },
  confirm_diagnosis: {
    label: '确认诊断',
    description: '确认诊断结论，作为训练方案的依据。',
    href: consultationHref,
  },
  generate_treatment: {
    label: '生成训练方案',
    description: '根据已确认的诊断生成分阶段训练计划。',
    href: consultationHref,
  },
  view_treatment: {
    label: '查看训练方案',
    description: '查看为你生成的分阶段训练内容。',
    href: trainingHref,
  },
  start_training: {
    label: '开始训练',
    description: '进入训练计划，开始第一次练习。',
    href: trainingHref,
  },
  view_progress: {
    label: '查看训练进度',
    description: '回顾打卡记录与阶段完成情况。',
    href: trainingHref,
  },
  log_training: {
    label: '记录本次训练',
    description: '完成打卡，并记录训练中的身体感受。',
    href: trainingHref,
  },
  reassess: {
    label: '进行复评',
    description: '训练周期已完成，复评以衡量改善幅度。',
    href: assessmentHref,
  },
  review_summary: {
    label: '查看阶段总结',
    description: '回顾整个周期的体态改善情况。',
    href: assessmentHref,
  },
};

/**
 * Resolve backend action codes into renderable descriptors.
 *
 * Unknown codes are dropped rather than thrown on: a backend deploying a new
 * action ahead of the frontend should degrade to fewer buttons, not a blank
 * dashboard.
 */
export function resolveJourneyActions(
  actions: readonly string[],
  artifacts: JourneyArtifacts,
): ResolvedJourneyAction[] {
  return actions.flatMap<ResolvedJourneyAction>((action) => {
    const descriptor = JOURNEY_ACTIONS[action as JourneyAction];
    if (!descriptor) return [];
    return [
      {
        action,
        label: descriptor.label,
        description: descriptor.description,
        href: descriptor.href(artifacts),
      },
    ];
  });
}
