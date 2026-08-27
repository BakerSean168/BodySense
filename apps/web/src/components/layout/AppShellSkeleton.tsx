interface AppShellSkeletonProps {
  variant?: "default" | "consultation";
}

function SkeletonBlock({ className }: { className: string }) {
  return (
    <div
      className={`animate-pulse rounded-full bg-white/[0.065] motion-reduce:animate-none ${className}`}
    />
  );
}

function BodyPlaceholder() {
  return (
    <div className="flex min-h-[360px] items-center justify-center" aria-hidden="true">
      <div className="relative h-[310px] w-[150px] animate-pulse opacity-70 motion-reduce:animate-none">
        <div className="absolute left-1/2 top-0 size-14 -translate-x-1/2 rounded-full bg-white/[0.075]" />
        <div className="absolute left-1/2 top-[62px] h-[132px] w-[76px] -translate-x-1/2 rounded-[38px] bg-white/[0.065]" />
        <div className="absolute left-[17px] top-[72px] h-[126px] w-5 rotate-[16deg] rounded-full bg-white/[0.06]" />
        <div className="absolute right-[17px] top-[72px] h-[126px] w-5 -rotate-[16deg] rounded-full bg-white/[0.06]" />
        <div className="absolute bottom-0 left-[48px] h-[128px] w-6 rotate-[5deg] rounded-full bg-white/[0.06]" />
        <div className="absolute bottom-0 right-[48px] h-[128px] w-6 -rotate-[5deg] rounded-full bg-white/[0.06]" />
      </div>
    </div>
  );
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
          <div className="mx-auto h-8 w-48 animate-pulse rounded-xl bg-muted motion-reduce:animate-none" />
          <div className="h-64 w-full animate-pulse rounded-xl bg-muted motion-reduce:animate-none" />
        </div>
      </div>
    );
  }

  return (
    <div
      className="flex h-dvh min-h-0 flex-col bg-[#242624] text-[#ededed]"
      data-testid="app-shell-skeleton"
      data-variant={variant}
    >
      <div className="relative flex h-12 shrink-0 items-center border-b border-white/[0.055] bg-[#242624] px-4">
        <SkeletonBlock className="h-5 w-5 rounded-md" />
        <div className="absolute left-1/2 flex -translate-x-1/2 items-center gap-8">
          <SkeletonBlock className="h-3 w-12" />
          <SkeletonBlock className="h-3 w-12" />
          <SkeletonBlock className="h-3 w-12" />
          <SkeletonBlock className="h-3 w-12" />
        </div>
        <SkeletonBlock className="ml-auto size-8" />
      </div>

      <div className="flex min-h-0 flex-1">
        <div className="hidden h-full w-[36%] min-w-[360px] flex-col rounded-tr-[26px] bg-[#171717] px-6 py-7 md:flex">
          <div className="mx-auto w-full max-w-[760px] flex-1 space-y-5">
            <SkeletonBlock className="h-3 w-[66%] rounded-md" />
            <SkeletonBlock className="h-3 w-[48%] rounded-md" />
            <div className="flex justify-end pt-3">
              <SkeletonBlock className="h-11 w-[56%] rounded-[20px]" />
            </div>
            <div className="pt-2">
              <SkeletonBlock className="h-3 w-[72%] rounded-md" />
            </div>
            <SkeletonBlock className="h-3 w-[42%] rounded-md" />
          </div>
          <div className="mx-auto w-full max-w-[760px] pt-5">
            <SkeletonBlock className="h-20 w-full rounded-[24px] bg-white/[0.075]" />
          </div>
        </div>

        <div className="min-w-0 flex-1 bg-[#242624] px-6 py-6 lg:px-8 lg:py-7">
          <div className="grid h-full min-h-0 gap-8 lg:grid-cols-[minmax(280px,0.78fr)_minmax(0,1.42fr)]">
            <div className="min-w-0">
              <BodyPlaceholder />
              <div className="mx-auto mt-3 flex max-w-[300px] items-center justify-between">
                <SkeletonBlock className="h-3 w-16 rounded-md" />
                <SkeletonBlock className="h-3 w-20 rounded-md" />
              </div>
            </div>

            <div className="min-w-0 space-y-7 pt-2">
              <div className="space-y-3">
                <SkeletonBlock className="h-5 w-28 rounded-md" />
                <SkeletonBlock className="h-3 w-[58%] rounded-md" />
              </div>
              {[0, 1, 2].map((row) => (
                <div
                  key={row}
                  className="space-y-3 border-b border-white/[0.055] pb-6"
                >
                  <SkeletonBlock className="h-3 w-[42%] rounded-md" />
                  <SkeletonBlock className="h-3 w-[76%] rounded-md" />
                  <SkeletonBlock className="h-3 w-[54%] rounded-md" />
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
