import { Focus, RotateCcw, ScanSearch } from "lucide-react";
import { Button } from "@/components/ui/Button";

export function BodyViewControls({
  hasSelection,
  isolated,
  onFocus,
  onToggleIsolation,
  onReset,
}: {
  hasSelection: boolean;
  isolated: boolean;
  onFocus: () => void;
  onToggleIsolation: () => void;
  onReset: () => void;
}) {
  return (
    <div
      aria-label="3D 身体视图控制"
      className="flex flex-wrap items-center gap-1.5"
    >
      <Button
        size="xs"
        variant="ghost"
        disabled={!hasSelection}
        onClick={onFocus}
      >
        <Focus className="size-3.5" aria-hidden="true" />
        聚焦
      </Button>
      <Button
        size="xs"
        variant="ghost"
        disabled={!hasSelection}
        aria-pressed={isolated}
        onClick={onToggleIsolation}
      >
        <ScanSearch className="size-3.5" aria-hidden="true" />
        {isolated ? "取消隔离" : "隔离"}
      </Button>
      <Button size="xs" variant="ghost" onClick={onReset}>
        <RotateCcw className="size-3.5" aria-hidden="true" />
        返回全身
      </Button>
    </div>
  );
}
