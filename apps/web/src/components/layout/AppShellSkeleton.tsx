interface AppShellSkeletonProps {
  variant?: "default" | "consultation";
}

function SkeletonBlock({ className }: { className: string }) {
  return <div className={`animate-pulse rounded-xl bg-muted ${className}`} />;
}

export function AppShellSkeleton({
  variant = "default",
}: AppShellSkeletonProps) {
  const isConsultation = variant === "consultation";

  if (!isConsultation) {
    return (
      <div
        className="flex min-h-dvh items-center justify-center bg-background p-6"
        data-testid="app-shell-skeleton"
        data-variant={variant}
      >
        <div className="w-full max-w-xl space-y-4">
          <SkeletonBlock className="mx-auto h-8 w-48" />
          <SkeletonBlock className="h-64 w-full" />
        </div>
      </div>
    );
  }

  return (
    <div
      className="flex h-dvh min-h-0 flex-col bg-background"
      data-testid="app-shell-skeleton"
      data-variant={variant}
    >
      <div className="flex h-14 shrink-0 items-center justify-between border-b border-border px-3">
        <div className="flex gap-2">
          <SkeletonBlock className="size-8" />
          <SkeletonBlock className="size-8" />
        </div>
        <SkeletonBlock className="h-9 w-64" />
        <SkeletonBlock className="size-8 rounded-full" />
      </div>
      <div className="flex min-h-0 flex-1 gap-3 p-3">
        <SkeletonBlock className="hidden h-full w-[38%] md:block" />
        <div className="flex min-w-0 flex-1 gap-3">
          <SkeletonBlock className="hidden h-full w-[34%] lg:block" />
          <SkeletonBlock className="h-full min-w-0 flex-1" />
        </div>
      </div>
    </div>
  );
}
