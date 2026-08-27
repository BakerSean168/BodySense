import { ArrowLeft, Microscope } from "lucide-react";
import { Button } from "@/components/ui/Button";
import type { AnatomyStructureId } from "../adapters/anatomyViewerPort";

export interface AnatomyStructureSummary {
  id: AnatomyStructureId;
  name: string;
  system: string;
  parentId?: AnatomyStructureId;
}

export function AnatomyInspector({
  mode,
  selected,
  breadcrumb,
  regionLabel,
  onEnterAnatomy,
  onReturnToRegion,
}: {
  mode: "region" | "anatomy";
  selected: AnatomyStructureSummary | null;
  breadcrumb: readonly AnatomyStructureSummary[];
  regionLabel?: string | null;
  onEnterAnatomy: () => void;
  onReturnToRegion: () => void;
}) {
  if (!selected && !regionLabel) return null;

  return (
    <div className="border-t border-border/55 pt-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            {mode === "anatomy" ? "解剖结构" : "当前选择"}
          </p>
          <p className="mt-1 truncate text-sm font-semibold text-foreground">
            {selected?.name ?? regionLabel}
          </p>
          {selected ? (
            <p className="mt-0.5 text-[11px] text-muted-foreground">
              {regionLabel ? `${regionLabel} · ` : ""}
              {selected.system}
            </p>
          ) : null}
        </div>
        {mode === "region" ? (
          <Button size="xs" variant="outline" onClick={onEnterAnatomy}>
            <Microscope className="size-3.5" aria-hidden="true" />
            深入查看
          </Button>
        ) : (
          <Button size="xs" variant="ghost" onClick={onReturnToRegion}>
            <ArrowLeft className="size-3.5" aria-hidden="true" />
            返回区域
          </Button>
        )}
      </div>

      {mode === "anatomy" && breadcrumb.length > 1 ? (
        <nav aria-label="解剖层级" className="mt-2 overflow-hidden">
          <ol className="flex min-w-0 flex-wrap items-center gap-x-1 text-[11px] text-muted-foreground">
            {breadcrumb.map((item, index) => (
              <li key={item.id} className="inline-flex min-w-0 items-center gap-1">
                {index > 0 ? <span aria-hidden="true">/</span> : null}
                <span className="max-w-[150px] truncate">{item.name}</span>
              </li>
            ))}
          </ol>
        </nav>
      ) : null}

      <p className="mt-2 text-[11px] leading-4 text-muted-foreground">
        选择结构只改变探索上下文，不代表该结构就是症状原因或医学诊断。
      </p>
    </div>
  );
}
