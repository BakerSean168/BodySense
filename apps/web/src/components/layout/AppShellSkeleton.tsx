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

  return (
    <div
      className="flex min-h-dvh bg-background"
      data-testid="app-shell-skeleton"
      data-variant={variant}
    >
      <aside className="hidden h-dvh w-[72px] shrink-0 flex-col border-r border-border bg-sidebar md:flex">
        <div className="flex h-16 items-center justify-center border-b border-border">
          <SkeletonBlock className="size-9" />
        </div>
        <div className="flex flex-1 flex-col items-center gap-3 px-2 py-4">
          {Array.from({ length: 6 }).map((_, index) => (
            <SkeletonBlock key={index} className="size-11" />
          ))}
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <div className="flex h-16 items-center justify-between border-b border-border px-4">
          <div className="flex items-center gap-3">
            <SkeletonBlock className="size-9 md:hidden" />
            <div>
              <SkeletonBlock className="h-4 w-28" />
              <SkeletonBlock className="mt-2 h-3 w-36" />
            </div>
          </div>
          <div className="flex gap-2">
            <SkeletonBlock className="size-8" />
            <SkeletonBlock className="size-8 rounded-full" />
          </div>
        </div>

        <main className="min-h-0 flex-1 overflow-hidden">
          {isConsultation ? (
            <div className="flex h-full min-h-0 gap-4 p-4 lg:p-6">
              <SkeletonBlock className="hidden h-full w-[38%] md:block" />
              <div className="flex min-w-0 flex-1 flex-col gap-4">
                <SkeletonBlock className="h-12 w-full" />
                <SkeletonBlock className="min-h-0 flex-1 w-full" />
              </div>
            </div>
          ) : (
            <div className="mx-auto w-full max-w-[1600px] space-y-6 px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
              <SkeletonBlock className="h-10 w-64" />
              <SkeletonBlock className="h-36 w-full" />
              <div className="grid gap-6 lg:grid-cols-2">
                <SkeletonBlock className="h-48 w-full" />
                <SkeletonBlock className="h-48 w-full" />
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}
