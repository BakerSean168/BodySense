export function BodyExplorerLoadingState({
  label = "正在加载 3D 身体视图",
  progress,
}: {
  label?: string;
  progress?: number | null;
}) {
  const normalizedProgress =
    typeof progress === "number"
      ? Math.max(0, Math.min(100, Math.round(progress)))
      : null;
  const hasMeaningfulProgress =
    normalizedProgress !== null && normalizedProgress > 0;

  return (
    <div
      role="status"
      aria-live="polite"
      className="flex min-h-[420px] w-full flex-col items-center justify-center px-6 text-center"
    >
      <div
        className="relative mb-6 h-[250px] w-[126px] animate-pulse opacity-80 motion-reduce:animate-none"
        aria-hidden="true"
      >
        <div className="absolute left-1/2 top-0 size-12 -translate-x-1/2 rounded-full border-2 border-white/10 bg-white/[0.035]" />
        <div className="absolute left-1/2 top-[54px] h-[108px] w-[64px] -translate-x-1/2 rounded-[32px] border-2 border-white/10 bg-white/[0.025]" />
        <div className="absolute left-[12px] top-[62px] h-[104px] w-4 rotate-[16deg] rounded-full bg-white/[0.07]" />
        <div className="absolute right-[12px] top-[62px] h-[104px] w-4 -rotate-[16deg] rounded-full bg-white/[0.07]" />
        <div className="absolute bottom-0 left-[38px] h-[102px] w-5 rotate-[5deg] rounded-full bg-white/[0.07]" />
        <div className="absolute bottom-0 right-[38px] h-[102px] w-5 -rotate-[5deg] rounded-full bg-white/[0.07]" />
      </div>

      <p className="text-sm font-medium text-foreground/90">{label}</p>
      <p className="mt-1.5 text-xs text-muted-foreground">
        {hasMeaningfulProgress
          ? `${normalizedProgress}%`
          : "身体记录会先显示，3D 视图随后补齐"}
      </p>
      {hasMeaningfulProgress ? (
        <div className="mt-3 h-1 w-36 overflow-hidden rounded-full bg-white/[0.06]">
          <div
            className="h-full rounded-full bg-primary/70 transition-[width] duration-200"
            style={{ width: `${normalizedProgress}%` }}
          />
        </div>
      ) : null}
    </div>
  );
}
