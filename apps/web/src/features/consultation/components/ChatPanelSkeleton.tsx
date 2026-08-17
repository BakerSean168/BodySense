function SkeletonBlock({ className }: { className: string }) {
  return (
    <div className={`animate-pulse rounded-2xl bg-[#ECE8E1] ${className}`} />
  );
}

export function ChatPanelSkeleton() {
  return (
    <div
      className="flex h-full flex-col bg-white p-4"
      data-testid="chat-panel-skeleton"
    >
      <div className="flex-1 space-y-4">
        <div className="flex justify-start">
          <SkeletonBlock className="h-20 w-[72%] rounded-[20px] rounded-bl-[4px]" />
        </div>
        <div className="flex justify-end">
          <SkeletonBlock className="h-14 w-[52%] rounded-[20px] rounded-br-[4px]" />
        </div>
        <div className="flex justify-start">
          <SkeletonBlock className="h-24 w-[68%] rounded-[20px] rounded-bl-[4px]" />
        </div>
      </div>

      <div className="mt-4 border-t border-[#E5E3DF] pt-4">
        <SkeletonBlock className="h-14 w-full rounded-[20px]" />
      </div>
    </div>
  );
}
