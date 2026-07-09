function SkeletonBlock({ className }: { className: string }) {
  return <div className={`animate-pulse rounded-2xl bg-[#ECE8E1] ${className}`} />;
}

export function InfoPanelSkeleton() {
  return (
    <div className="flex h-full flex-col bg-white" data-testid="info-panel-skeleton">
      <div className="flex items-center justify-between border-b border-[#E5E3DF] bg-[#F7F5F0]/50 p-4">
        <SkeletonBlock className="h-6 w-28 rounded-full" />
        <SkeletonBlock className="h-7 w-20 rounded-full" />
      </div>

      <div className="flex-1 space-y-4 overflow-y-auto p-4">
        <SkeletonBlock className="h-32 w-full" />
        <SkeletonBlock className="h-24 w-full" />
        <SkeletonBlock className="h-40 w-full" />
      </div>
    </div>
  );
}
