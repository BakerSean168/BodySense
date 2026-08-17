interface AppShellSkeletonProps {
  variant?: "default" | "consultation";
}

function SkeletonBlock({ className }: { className: string }) {
  return (
    <div className={`animate-pulse rounded-2xl bg-[#E9E6DF] ${className}`} />
  );
}

export function AppShellSkeleton({
  variant = "default",
}: AppShellSkeletonProps) {
  const isConsultation = variant === "consultation";

  return (
    <div
      className="min-h-screen bg-slate-50 flex"
      data-testid="app-shell-skeleton"
      data-variant={variant}
    >
      <div className="hidden lg:flex w-64 shrink-0 border-r border-[#E5E3DF] bg-[#F7F5F0]">
        <div className="flex w-full flex-col">
          <div className="flex h-16 items-center gap-3 border-b border-[#E5E3DF] px-6">
            <SkeletonBlock className="h-8 w-8 rounded-xl" />
            <SkeletonBlock className="h-5 w-20 rounded-full" />
          </div>
          <div className="flex-1 space-y-3 px-4 py-6">
            {Array.from({ length: 4 }).map((_, index) => (
              <SkeletonBlock key={index} className="h-11 w-full rounded-full" />
            ))}
          </div>
          <div className="border-t border-[#E5E3DF] p-4">
            <SkeletonBlock className="h-16 w-full" />
            <SkeletonBlock className="mt-3 h-10 w-full rounded-full" />
          </div>
        </div>
      </div>

      <div className="flex min-w-0 flex-1 flex-col overflow-hidden bg-[#FBFBFA]">
        <div className="flex h-16 items-center justify-between border-b border-[#E5E3DF] bg-white/90 px-4 lg:hidden">
          <div className="flex items-center gap-3">
            <SkeletonBlock className="h-8 w-8 rounded-xl" />
            <SkeletonBlock className="h-5 w-16 rounded-full" />
          </div>
          <SkeletonBlock className="h-10 w-10 rounded-xl" />
        </div>

        <main className="flex-1 overflow-hidden">
          {isConsultation ? (
            <div className="flex h-full w-full flex-col overflow-hidden">
              <div className="border-b border-[#E5E3DF] bg-[#FBFBFA] px-6 py-4">
                <SkeletonBlock className="h-6 w-48 rounded-full" />
                <SkeletonBlock className="mt-2 h-4 w-32 rounded-full" />
              </div>

              <div className="flex flex-1 min-h-0 gap-4 px-3 py-4 md:px-6 md:py-6">
                <div className="hidden w-64 shrink-0 lg:block">
                  <div className="space-y-3">
                    <SkeletonBlock className="h-10 w-full rounded-full" />
                    {Array.from({ length: 6 }).map((_, index) => (
                      <SkeletonBlock
                        key={index}
                        className="h-14 w-full rounded-2xl"
                      />
                    ))}
                  </div>
                </div>

                <div className="flex min-h-0 flex-1 flex-col">
                  <SkeletonBlock className="flex-1 min-h-[320px] w-full rounded-[28px]" />
                </div>

                <div className="hidden w-[420px] shrink-0 md:block">
                  <SkeletonBlock className="h-full min-h-[320px] w-full rounded-[28px]" />
                </div>
              </div>
            </div>
          ) : (
            <div className="space-y-6 px-6 py-8 lg:px-10">
              <SkeletonBlock className="h-10 w-64 rounded-full" />
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
