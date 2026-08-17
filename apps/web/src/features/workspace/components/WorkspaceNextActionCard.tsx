import { Fragment } from "react";
import { useNavigate } from "react-router";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import type { HealthWorkspace } from "../types/workspace";
import { resolveWorkspaceActions } from "../lib/workspaceActions";

interface WorkspaceNextActionCardProps {
  workspace: HealthWorkspace | null;
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
}

export function WorkspaceNextActionCard({
  workspace,
  isLoading,
  error,
  onRetry,
}: WorkspaceNextActionCardProps) {
  const navigate = useNavigate();

  if (isLoading) {
    return (
      <Card className="p-6">
        <div className="animate-pulse space-y-4">
          <div className="h-4 w-24 rounded bg-[#E5E3DF]" />
          <div className="h-6 w-2/3 rounded bg-[#E5E3DF]" />
          <div className="h-10 w-40 rounded-lg bg-[#E5E3DF]" />
        </div>
      </Card>
    );
  }

  if (error || !workspace) {
    return (
      <Card className="p-6">
        <h2 className="text-lg font-display font-semibold text-[#2E3C36]">
          下一步
        </h2>
        <p className="mt-2 text-sm text-[#5D6B63]">
          暂时无法获取长期健康工作区状态。
        </p>
        <Button variant="outline" size="sm" className="mt-4" onClick={onRetry}>
          重试
        </Button>
      </Card>
    );
  }

  const actions = resolveWorkspaceActions(workspace);
  const [primary, ...secondary] = actions;
  if (!primary) {
    return (
      <Card className="p-6">
        <h2 className="text-lg font-display font-semibold text-[#2E3C36]">
          下一步
        </h2>
        <p className="mt-2 text-sm text-[#5D6B63]">
          当前没有需要优先处理的事项，可以继续记录身体变化。
        </p>
      </Card>
    );
  }

  return (
    <Card className="p-6 sm:p-8">
      <h2 className="text-lg font-display font-semibold text-[#2E3C36]">
        下一步
      </h2>
      <p className="mt-4 text-base font-medium leading-relaxed text-[#2E3C36]">
        {primary.description}
      </p>
      <div className="mt-6 flex flex-wrap gap-3">
        <Button
          className="bg-[#CD7B67] text-white shadow-sm shadow-[#CD7B67]/15 hover:bg-[#B65E49]"
          onClick={() => navigate(primary.href)}
        >
          {primary.label}
        </Button>
        {secondary.slice(0, 3).map((action) => (
          <Fragment key={action.kind}>
            <Button
              variant="outline"
              className="border-[#CD7B67] text-[#CD7B67] hover:bg-[#CD7B67]/5"
              onClick={() => navigate(action.href)}
            >
              {action.label}
            </Button>
          </Fragment>
        ))}
      </div>
    </Card>
  );
}
