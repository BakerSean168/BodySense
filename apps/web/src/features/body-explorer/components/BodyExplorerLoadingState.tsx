export function BodyExplorerLoadingState({
  label = "正在加载 3D 身体视图",
  progress,
}: {
  label?: string;
  progress?: number | null;
}) {
  return (
    <div
      role="status"
      aria-live="polite"
      className="flex min-h-[420px] w-full flex-col items-center justify-center gap-3 text-center"
    >
      <div className="size-8 animate-spin rounded-full border-2 border-muted-foreground/25 border-t-primary motion-reduce:animate-none" />
      <div>
        <p className="text-sm font-medium text-foreground/90">{label}</p>
        <p className="mt-1 text-xs text-muted-foreground">
          {typeof progress === "number"
            ? `${Math.max(0, Math.min(100, Math.round(progress)))}%`
            : "身体记录仍然可以继续查看"}
        </p>
      </div>
    </div>
  );
}
