import type { HealthWorkspace, WorkspaceAction } from "../types/workspace";

export interface ResolvedWorkspaceAction {
  kind: string;
  label: string;
  description: string;
  href: string;
}

const LABELS: Record<string, string> = {
  complete_profile: "完善身体档案",
  review_safety: "处理安全提醒",
  review_treatment: "审核当前方案",
  review_diagnosis: "复核最新分析",
  review_treatment_proposal: "审核方案提案",
  open_training: "继续执行训练",
  generate_treatment: "生成改善方案",
  request_diagnosis: "请求可能性分析",
  record_outcome: "记录身体变化",
  continue_consultation: "继续问诊",
};

function consultationHref(workspace: HealthWorkspace): string {
  return workspace.conversation_id
    ? `/consultation/${workspace.conversation_id}`
    : "/consultation";
}

function actionHref(
  action: WorkspaceAction,
  workspace: HealthWorkspace,
): string {
  const explicitRoute = action.target?.route;
  if (typeof explicitRoute === "string" && explicitRoute.startsWith("/")) {
    return explicitRoute;
  }
  return consultationHref(workspace);
}

export function resolveWorkspaceActions(
  workspace: HealthWorkspace,
): ResolvedWorkspaceAction[] {
  return workspace.actions
    .filter((action) => action.enabled)
    .map((action) => ({
      kind: action.kind,
      label: LABELS[action.kind] ?? action.kind,
      description: action.reason,
      href: actionHref(action, workspace),
    }));
}
