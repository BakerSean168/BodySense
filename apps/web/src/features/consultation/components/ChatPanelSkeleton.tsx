function SkeletonBlock({ className }: { className: string }) {
  return (
    <div
      className={`animate-pulse rounded-2xl bg-white/[0.065] ${className}`}
    />
  );
}

export function ChatPanelSkeleton() {
  return (
    <div
      className="flex h-full flex-col bg-[#171717] px-5 py-6"
      data-testid="chat-panel-skeleton"
    >
      <div className="mx-auto w-full max-w-[760px] flex-1 space-y-6">
        <div className="flex justify-start">
          <SkeletonBlock className="h-4 w-[68%] rounded-md" />
        </div>
        <div className="flex justify-start">
          <SkeletonBlock className="h-4 w-[52%] rounded-md" />
        </div>
        <div className="flex justify-end pt-2">
          <SkeletonBlock className="h-12 w-[58%] rounded-[20px]" />
        </div>
        <div className="flex justify-start pt-2">
          <SkeletonBlock className="h-4 w-[72%] rounded-md" />
        </div>
        <div className="flex justify-start">
          <SkeletonBlock className="h-4 w-[44%] rounded-md" />
        </div>
      </div>

      <div className="mx-auto mt-5 w-full max-w-[760px]">
        <SkeletonBlock className="h-20 w-full rounded-[24px]" />
      </div>
    </div>
  );
}
