import { RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { BodyOverview } from "@/features/consultation/components/workbench/BodyOverview";
import type { BodyStateSnapshot } from "@/features/consultation/types/consultation";
import type { AnatomyViewerErrorState } from "../adapters/anatomyViewerPort";

export function BodyExplorerFallback2D({
  snapshot,
  error,
  canRetry,
  onRetry,
  selectionRetained = false,
}: {
  snapshot: BodyStateSnapshot | null;
  error?: AnatomyViewerErrorState | null;
  canRetry?: boolean;
  onRetry?: () => void;
  selectionRetained?: boolean;
}) {
  return (
    <div className="min-h-0">
      {error ? (
        <div
          role="status"
          className="mb-3 flex items-center justify-between gap-3 border-b border-border/55 pb-3"
        >
          <div className="min-w-0">
            <p className="text-xs font-medium text-foreground">
              3D 身体视图暂时不可用
            </p>
            <p className="mt-1 text-[11px] leading-4 text-muted-foreground">
              已切换到可访问的 2D 身体概览。
              {selectionRetained ? " 当前选择已保留。" : ""}
            </p>
          </div>
          {canRetry && onRetry ? (
            <Button size="xs" variant="outline" onClick={onRetry}>
              <RotateCcw className="size-3.5" aria-hidden="true" />
              重试
            </Button>
          ) : null}
        </div>
      ) : null}
      <BodyOverview snapshot={snapshot} />
    </div>
  );
}
