import { useNavigate } from "react-router";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { useHealthWorkspaceQuery } from "../hooks/useHealthWorkspaceQuery";
import { resolveWorkspaceActions } from "../lib/workspaceActions";

export type WorkspaceGatedRoute = "training" | "assessment";

interface WorkspaceSoftGuardProps {
  route: WorkspaceGatedRoute;
  children: React.ReactNode;
}

function isReady(
  route: WorkspaceGatedRoute,
  workspace: NonNullable<ReturnType<typeof useHealthWorkspaceQuery>["data"]>,
) {
  if (route === "assessment") return workspace.profile_ready;
  return (
    workspace.capabilities.can_execute_treatment ||
    workspace.capabilities.can_record_outcome
  );
}

export function WorkspaceSoftGuard({
  route,
  children,
}: WorkspaceSoftGuardProps) {
  const navigate = useNavigate();
  const query = useHealthWorkspaceQuery();
  const workspace = query.data;

  if (
    query.isLoading ||
    query.error ||
    !workspace ||
    isReady(route, workspace)
  ) {
    return <>{children}</>;
  }

  const [primary] = resolveWorkspaceActions(workspace);
  const title = route === "training" ? "训练计划" : "姿态评估";
  const hint =
    route === "training"
      ? "当前还没有已接受且可执行的改善方案。先继续完善 BodyState、分析并审核方案。"
      : "开始评估前请先完善身体档案。";

  return (
    <Card className="mx-auto max-w-xl p-8 text-center shadow-sm">
      <h2 className="text-xl font-display font-semibold text-foreground">
        还不能进入{title}
      </h2>
      <p className="mt-3 text-sm font-medium leading-relaxed text-muted-foreground">
        {hint}
      </p>
      <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
        <Button
          className="bg-primary text-primary-foreground shadow-sm hover:bg-primary/90"
          onClick={() => navigate(primary?.href ?? "/dashboard")}
        >
          {primary?.label ?? "返回首页"}
        </Button>
        <Button variant="ghost" size="sm" onClick={() => void query.refetch()}>
          刷新状态
        </Button>
      </div>
    </Card>
  );
}
